package scheduler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/builder"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager"
)

type Scheduler interface {
	Start(builds.Build)
	Attach(builder.RunningBuild)
	Abort(guid string)
	Hijack(guid string, process warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error)

	Drain() []builder.RunningBuild
}

type scheduler struct {
	logger lager.Logger

	builder builder.Builder

	createEmitter EmitterFactory

	httpClient *http.Client

	inFlight *sync.WaitGroup
	draining chan struct{}
	running  map[string]builder.RunningBuild
	aborting map[string]chan struct{}

	mutex *sync.RWMutex
}

type EmitterFactory func(logsURL string, drain <-chan struct{}) event.Emitter

func NewScheduler(
	l lager.Logger,
	b builder.Builder,
	createEmitter EmitterFactory,
) Scheduler {
	return &scheduler{
		logger: l,

		builder: b,

		createEmitter: createEmitter,

		httpClient: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},

		inFlight: new(sync.WaitGroup),
		draining: make(chan struct{}),
		running:  make(map[string]builder.RunningBuild),
		aborting: make(map[string]chan struct{}),

		mutex: new(sync.RWMutex),
	}
}

func (scheduler *scheduler) Drain() []builder.RunningBuild {
	close(scheduler.draining)
	scheduler.inFlight.Wait()
	return scheduler.runningBuilds()
}

func (scheduler *scheduler) Start(build builds.Build) {
	scheduler.inFlight.Add(1)

	log := scheduler.logger.Session("start", lager.Data{
		"build": build,
	})

	abort := scheduler.abortChannel(build.Guid)

	go func() {
		emitter := scheduler.createEmitter(build.EventsCallback, scheduler.draining)
		defer emitter.Close()

		running, err := scheduler.builder.Start(build, emitter, abort)
		if err != nil {
			log.Error("errored", err)

			build.Status = builds.StatusErrored
			scheduler.reportBuild(build, log, emitter)
		} else {
			log.Info("started")

			running.Build.Status = builds.StatusStarted
			scheduler.reportBuild(running.Build, log, emitter)

			scheduler.attach(running, emitter)
		}

		scheduler.unregisterAbortChannel(build.Guid)
		scheduler.inFlight.Done()
	}()
}

func (scheduler *scheduler) Attach(running builder.RunningBuild) {
	emitter := scheduler.createEmitter(running.Build.EventsCallback, scheduler.draining)
	scheduler.attach(running, emitter)
}

func (scheduler *scheduler) Abort(guid string) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	abort, found := scheduler.aborting[guid]
	if !found {
		return
	}

	close(abort)
}

func (scheduler *scheduler) Hijack(guid string, spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
	scheduler.mutex.Lock()
	running, found := scheduler.running[guid]
	scheduler.mutex.Unlock()

	if !found {
		return nil, errors.New("unknown build")
	}

	return scheduler.builder.Hijack(running, spec, io)
}

func (scheduler *scheduler) attach(running builder.RunningBuild, emitter event.Emitter) {
	scheduler.inFlight.Add(1) // in addition to .Start's
	defer scheduler.inFlight.Done()

	scheduler.addRunning(running)

	abort := scheduler.abortChannel(running.Build.Guid)
	defer scheduler.unregisterAbortChannel(running.Build.Guid)

	log := scheduler.logger.Session("attach", lager.Data{
		"build": running.Build,
	})

	exited := make(chan builder.ExitedBuild, 1)
	errored := make(chan error, 1)

	go func() {
		ex, err := scheduler.builder.Attach(running, emitter, abort)
		if err != nil {
			errored <- err
		} else {
			exited <- ex
		}
	}()

	select {
	case build := <-exited:
		log.Info("exited")

		scheduler.finish(build, emitter)
	case err := <-errored:
		log.Error("errored", err)

		running.Build.Status = builds.StatusErrored
		scheduler.reportBuild(running.Build, log, emitter)
	case <-scheduler.draining:
		return
	}

	scheduler.removeRunning(running)
}

func (scheduler *scheduler) finish(exited builder.ExitedBuild, emitter event.Emitter) {
	abort := scheduler.abortChannel(exited.Build.Guid)

	log := scheduler.logger.Session("finish", lager.Data{
		"build": exited.Build,
	})

	finished, err := scheduler.builder.Finish(exited, emitter, abort)
	if err != nil {
		log.Error("failed", err)

		exited.Build.Status = builds.StatusErrored
		scheduler.reportBuild(exited.Build, log, emitter)
	} else {
		log.Info("finished")

		if exited.ExitStatus == 0 {
			finished.Status = builds.StatusSucceeded
		} else {
			finished.Status = builds.StatusFailed
		}

		scheduler.reportBuild(finished, log, emitter)
	}
}

func (scheduler *scheduler) runningBuilds() []builder.RunningBuild {
	scheduler.mutex.RLock()

	running := []builder.RunningBuild{}
	for _, build := range scheduler.running {
		running = append(running, build)
	}

	scheduler.mutex.RUnlock()

	return running
}

func (scheduler *scheduler) addRunning(running builder.RunningBuild) {
	scheduler.mutex.Lock()
	scheduler.running[running.Build.Guid] = running
	scheduler.mutex.Unlock()
}

func (scheduler *scheduler) removeRunning(running builder.RunningBuild) {
	scheduler.mutex.Lock()
	delete(scheduler.running, running.Build.Guid)
	scheduler.mutex.Unlock()
}

func (scheduler *scheduler) abortChannel(guid string) chan struct{} {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	abort, found := scheduler.aborting[guid]
	if !found {
		abort = make(chan struct{})
		scheduler.aborting[guid] = abort
	}

	return abort
}

func (scheduler *scheduler) unregisterAbortChannel(guid string) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	delete(scheduler.aborting, guid)
}

func (scheduler *scheduler) reportBuild(build builds.Build, logger lager.Logger, emitter event.Emitter) {
	emitter.EmitEvent(event.Status{
		Status: build.Status,
	})

	if build.StatusCallback == "" {
		return
	}

	log := logger.Session("report", lager.Data{
		"build": build,
	})

	// this should always successfully parse (it's done via validation)
	destination, _ := url.ParseRequestURI(build.StatusCallback)

	payload, _ := json.Marshal(build)

	for {
		res, err := scheduler.httpClient.Do(&http.Request{
			Method: "PUT",
			URL:    destination,

			ContentLength: int64(len(payload)),

			Header: map[string][]string{
				"Content-Type": {"application/json"},
			},

			Body: ioutil.NopCloser(bytes.NewBuffer(payload)),
		})

		if err != nil {
			log.Error("failed", err)

			select {
			case <-time.After(time.Second):
				// retry every second
				continue
			case <-scheduler.draining:
				// don't block draining on failing callbacks
				return
			}
		}

		res.Body.Close()

		break
	}
}
