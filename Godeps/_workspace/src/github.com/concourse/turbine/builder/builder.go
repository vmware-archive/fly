package builder

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"

	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/concourse/turbine/logwriter"
	"github.com/concourse/turbine/resource"
)

var ErrAborted = errors.New("build aborted")

type Builder interface {
	Start(builds.Build, event.Emitter, <-chan struct{}) (RunningBuild, error)
	Attach(RunningBuild, event.Emitter, <-chan struct{}) (SucceededBuild, error, error)
	Hijack(RunningBuild, warden.ProcessSpec, warden.ProcessIO) (warden.Process, error)
	Complete(SucceededBuild, event.Emitter, <-chan struct{}) (builds.Build, error)
}

type RunningBuild struct {
	Build builds.Build

	ContainerHandle string
	Container       warden.Container

	ProcessID uint32
	Process   warden.Process
}

type SucceededBuild struct {
	Build builds.Build

	ContainerHandle string
	Container       warden.Container
}

type builder struct {
	tracker      resource.Tracker
	wardenClient warden.Client
}

func NewBuilder(
	tracker resource.Tracker,
	wardenClient warden.Client,
) Builder {
	return &builder{
		tracker:      tracker,
		wardenClient: wardenClient,
	}
}

func (builder *builder) Start(build builds.Build, emitter event.Emitter, abort <-chan struct{}) (RunningBuild, error) {
	resources := map[string]io.Reader{}

	for i, input := range build.Inputs {
		eventLog := logwriter.NewWriter(emitter, event.Origin{
			Type: event.OriginTypeInput,
			Name: input.Name,
		})

		resource, err := builder.tracker.Init(input.Type, eventLog, abort)
		if err != nil {
			return RunningBuild{}, builder.emitError(emitter, "failed to initialize "+input.Name, err)
		}

		defer builder.tracker.Release(resource)

		tarStream, computedInput, buildConfig, err := resource.In(input)
		if err != nil {
			return RunningBuild{}, builder.emitError(emitter, "failed to fetch "+input.Name, err)
		}

		build.Inputs[i] = computedInput

		build.Config = build.Config.Merge(buildConfig)

		resources[input.Name] = tarStream
	}

	container, err := builder.createBuildContainer(build.Config, emitter)
	if err != nil {
		return RunningBuild{}, builder.emitError(emitter, "failed to create container", err)
	}

	err = builder.streamInResources(container, resources, build.Config.Paths)
	if err != nil {
		return RunningBuild{}, builder.emitError(emitter, "failed to stream in resources", err)
	}

	process, err := builder.runBuild(container, build.Privileged, build.Config, emitter)
	if err != nil {
		return RunningBuild{}, builder.emitError(emitter, "failed to run", err)
	}

	return RunningBuild{
		Build: build,

		ContainerHandle: container.Handle(),
		Container:       container,

		ProcessID: process.ID(),
		Process:   process,
	}, nil
}

func (builder *builder) Attach(running RunningBuild, emitter event.Emitter, abort <-chan struct{}) (SucceededBuild, error, error) {
	if running.Container == nil {
		container, err := builder.wardenClient.Lookup(running.ContainerHandle)
		if err != nil {
			return SucceededBuild{}, nil, builder.emitError(emitter, "failed to lookup container", err)
		}

		running.Container = container
	}

	if running.Process == nil {
		process, err := running.Container.Attach(
			running.ProcessID,
			emitterProcessIO(emitter),
		)
		if err != nil {
			return SucceededBuild{}, nil, builder.emitError(emitter, "failed to attach to process", err)
		}

		running.Process = process
	}

	status, err := builder.waitForRunToEnd(running, abort)
	if err != nil {
		return SucceededBuild{}, nil, builder.emitError(emitter, "result unknown", err)
	}

	if status != 0 {
		return SucceededBuild{}, fmt.Errorf("exit status %d", status), nil
	}

	return SucceededBuild{
		Build:     running.Build,
		Container: running.Container,
	}, nil, nil
}

func (builder *builder) Complete(succeeded SucceededBuild, emitter event.Emitter, abort <-chan struct{}) (builds.Build, error) {
	outputs, err := builder.performOutputs(succeeded.Container, succeeded.Build, emitter, abort)
	if err != nil {
		return builds.Build{}, builder.emitError(emitter, "outputs failed", err)
	}

	succeeded.Build.Outputs = outputs

	return succeeded.Build, nil
}

func (builder *builder) Hijack(running RunningBuild, spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
	container, err := builder.wardenClient.Lookup(running.ContainerHandle)
	if err != nil {
		return nil, err
	}

	return container.Run(spec, io)
}

func (builder *builder) emitError(emitter event.Emitter, message string, err error) error {
	emitter.EmitEvent(event.Error{
		Message: fmt.Sprintf("%s: %s", message, err),
	})

	return err
}

func (builder *builder) createBuildContainer(
	buildConfig builds.Config,
	emitter event.Emitter,
) (warden.Container, error) {
	emitter.EmitEvent(event.Initialize{
		BuildConfig: buildConfig,
	})

	containerSpec := warden.ContainerSpec{
		RootFSPath: buildConfig.Image,
	}

	return builder.wardenClient.Create(containerSpec)
}

func (builder *builder) streamInResources(
	container warden.Container,
	resources map[string]io.Reader,
	paths map[string]string,
) error {
	for name, streamOut := range resources {
		destination, found := paths[name]
		if !found {
			destination = name
		}

		err := container.StreamIn("/tmp/build/src/"+destination, streamOut)
		if err != nil {
			return err
		}
	}

	return nil
}

func (builder *builder) runBuild(
	container warden.Container,
	privileged bool,
	buildConfig builds.Config,
	emitter event.Emitter,
) (warden.Process, error) {
	emitter.EmitEvent(event.Start{
		Time: time.Now().Unix(),
	})

	env := []string{}
	for n, v := range buildConfig.Params {
		env = append(env, n+"="+v)
	}

	return container.Run(warden.ProcessSpec{
		Path: buildConfig.Run.Path,
		Args: buildConfig.Run.Args,
		Env:  env,
		Dir:  "/tmp/build/src",

		TTY: &warden.TTYSpec{},

		Privileged: privileged,
	}, emitterProcessIO(emitter))
}

func (builder *builder) waitForRunToEnd(
	running RunningBuild,
	abort <-chan struct{},
) (int, error) {
	statusCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		status, err := running.Process.Wait()
		if err != nil {
			errCh <- err
		} else {
			statusCh <- status
		}
	}()

	select {
	case status := <-statusCh:
		return status, nil

	case err := <-errCh:
		return 0, err

	case <-abort:
		running.Container.Stop(false)
		return 0, ErrAborted
	}
}

func (builder *builder) performOutputs(
	container warden.Container,
	build builds.Build,
	emitter event.Emitter,
	abort <-chan struct{},
) ([]builds.Output, error) {
	allOutputs := map[string]builds.Output{}

	// implicit outputs
	for _, input := range build.Inputs {
		allOutputs[input.Name] = builds.Output{
			Name:     input.Name,
			Type:     input.Type,
			Source:   input.Source,
			Version:  input.Version,
			Metadata: input.Metadata,
		}
	}

	if len(build.Outputs) > 0 {
		errs := make(chan error, len(build.Outputs))
		results := make(chan builds.Output, len(build.Outputs))

		for _, output := range build.Outputs {
			go func(output builds.Output) {
				inputOutput, found := allOutputs[output.Name]
				if found {
					output.Version = inputOutput.Version
				}

				streamOut, err := container.StreamOut("/tmp/build/src/")
				if err != nil {
					errs <- err
					return
				}

				eventLog := logwriter.NewWriter(emitter, event.Origin{
					Type: event.OriginTypeOutput,
					Name: output.Name,
				})

				resource, err := builder.tracker.Init(output.Type, eventLog, abort)
				if err != nil {
					errs <- err
					return
				}

				defer builder.tracker.Release(resource)

				computedOutput, err := resource.Out(streamOut, output)

				errs <- err
				results <- computedOutput
			}(output)
		}

		var outputErr error
		for i := 0; i < len(build.Outputs); i++ {
			err := <-errs
			if err != nil {
				outputErr = err
			}
		}

		if outputErr != nil {
			return nil, outputErr
		}

		for i := 0; i < len(build.Outputs); i++ {
			output := <-results
			allOutputs[output.Name] = output
		}
	}

	outputs := []builds.Output{}
	for _, output := range allOutputs {
		outputs = append(outputs, output)
	}

	return outputs, nil
}

func emitterProcessIO(emitter event.Emitter) warden.ProcessIO {
	return warden.ProcessIO{
		Stdout: logwriter.NewWriter(emitter, event.Origin{
			Type: event.OriginTypeRun,
			Name: "stdout",
		}),
		Stderr: logwriter.NewWriter(emitter, event.Origin{
			Type: event.OriginTypeRun,
			Name: "stderr",
		}),
	}
}
