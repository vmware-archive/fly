package outputs

import (
	"fmt"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/concourse/turbine/logwriter"
	"github.com/concourse/turbine/resource"
)

type Performer interface {
	PerformOutputs(warden.Container, []builds.Output, event.Emitter, <-chan struct{}) ([]builds.Output, error)
}

func NewParallelPerformer(tracker resource.Tracker) Performer {
	return parallelPerformer{tracker: tracker}
}

type parallelPerformer struct {
	tracker resource.Tracker
}

func (p parallelPerformer) PerformOutputs(
	container warden.Container,
	outputs []builds.Output,
	emitter event.Emitter,
	abort <-chan struct{},
) ([]builds.Output, error) {
	errs := make(chan error, len(outputs))
	results := make(chan builds.Output, len(outputs))

	for _, output := range outputs {
		go func(output builds.Output) {
			streamOut, err := container.StreamOut("/tmp/build/src/")
			if err != nil {
				errs <- err
				return
			}

			eventLog := logwriter.NewWriter(emitter, event.Origin{
				Type: event.OriginTypeOutput,
				Name: output.Name,
			})

			resource, err := p.tracker.Init(output.Type, eventLog, abort)
			if err != nil {
				errs <- err
				return
			}

			defer p.tracker.Release(resource)

			computedOutput, err := resource.Out(streamOut, output)

			if err != nil {
				emitter.EmitEvent(event.Error{
					Message: fmt.Sprintf(output.Name+" output failed: %s", err),
				})
			} else {
				emitter.EmitEvent(event.Output{Output: computedOutput})
			}

			errs <- err
			results <- computedOutput
		}(output)
	}

	var outputErr error
	for i := 0; i < len(outputs); i++ {
		err := <-errs
		if err != nil {
			outputErr = err
		}
	}

	if outputErr != nil {
		return nil, outputErr
	}

	var resultingOutputs []builds.Output
	for i := 0; i < len(outputs); i++ {
		resultingOutputs = append(resultingOutputs, <-results)
	}

	return resultingOutputs, nil
}
