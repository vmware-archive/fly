package outputs_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"sync"

	"github.com/cloudfoundry-incubator/garden/client/fake_warden_client"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/api/builds"
	. "github.com/concourse/turbine/builder/outputs"
	"github.com/concourse/turbine/event"
	efakes "github.com/concourse/turbine/event/fakes"
	"github.com/concourse/turbine/resource"
	rfakes "github.com/concourse/turbine/resource/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Performer", func() {
	var (
		wardenClient *fake_warden_client.FakeClient

		tracker   *rfakes.FakeTracker
		performer Performer

		container        warden.Container
		outputsToPerform []builds.Output
		emitter          *efakes.FakeEmitter
		events           *eventLog
		abort            chan struct{}

		resource1 *rfakes.FakeResource
		resource2 *rfakes.FakeResource

		performedOutputs []builds.Output
		performErr       error
	)

	BeforeEach(func() {
		var err error

		wardenClient = fake_warden_client.New()

		wardenClient.Connection.CreateReturns("the-performing-container", nil)

		container, err = wardenClient.Create(warden.ContainerSpec{})
		Ω(err).ShouldNot(HaveOccurred())

		resource1 = new(rfakes.FakeResource)
		resource2 = new(rfakes.FakeResource)

		resources := make(chan resource.Resource, 2)
		resources <- resource1
		resources <- resource2

		tracker = new(rfakes.FakeTracker)
		tracker.InitStub = func(typ string, logs io.Writer, abort <-chan struct{}) (resource.Resource, error) {
			select {
			case r := <-resources:
				return r, nil
			default:
				Fail("initialized too many resources")
				panic("unreachable")
			}
		}

		outputsToPerform = []builds.Output{
			builds.Output{
				Name:   "banana",
				Type:   "some-type",
				Params: builds.Params{"key": "banana-param"},
				Source: builds.Source{"uri": "http://banana-uri"},
			},
			builds.Output{
				Name:   "monkey",
				Type:   "some-type",
				Params: builds.Params{"key": "monkey-param"},
				Source: builds.Source{"uri": "http://monkey-uri"},
			},
		}

		emitter = new(efakes.FakeEmitter)

		events = &eventLog{}
		emitter.EmitEventStub = events.Add

		performer = NewParallelPerformer(tracker)
	})

	JustBeforeEach(func() {
		abort = make(chan struct{})
		performedOutputs, performErr = performer.PerformOutputs(container, outputsToPerform, emitter, abort)
	})

	performedOutput := func(output builds.Output) builds.Output {
		output.Version = builds.Version{"version": output.Name + "-performed"}
		output.Metadata = []builds.MetadataField{{Name: "output", Value: output.Name}}
		return output
	}

	Context("when streaming out succeeds", func() {
		BeforeEach(func() {
			wardenClient.Connection.StreamOutStub = func(handle string, srcPath string) (io.ReadCloser, error) {
				return ioutil.NopCloser(bytes.NewBufferString("streamed-out")), nil
			}
		})

		Context("when each output succeeds", func() {
			BeforeEach(func() {
				sync := new(sync.WaitGroup)
				sync.Add(2)

				resource1.OutStub = func(src io.Reader, output builds.Output) (builds.Output, error) {
					sync.Done()
					sync.Wait()
					return performedOutput(output), nil
				}

				resource2.OutStub = func(src io.Reader, output builds.Output) (builds.Output, error) {
					sync.Done()
					sync.Wait()
					return performedOutput(output), nil
				}
			})

			It("performs each output in parallel", func() {
				Ω(resource1.OutCallCount()).Should(Equal(1))
				streamIn, output1 := resource1.OutArgsForCall(0)
				streamedIn, err := ioutil.ReadAll(streamIn)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(streamedIn)).Should(Equal("streamed-out"))

				Ω(resource2.OutCallCount()).Should(Equal(1))
				streamIn, output2 := resource2.OutArgsForCall(0)
				streamedIn, err = ioutil.ReadAll(streamIn)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(streamedIn)).Should(Equal("streamed-out"))

				Ω([]builds.Output{output1, output2}).Should(ConsistOf(outputsToPerform))
			})

			It("returns the outputs and emits events for each explicit output", func() {
				Ω(performedOutputs).Should(HaveLen(2))

				monkeyResult := builds.Output{
					Name:     "monkey",
					Type:     "some-type",
					Source:   builds.Source{"uri": "http://monkey-uri"},
					Params:   builds.Params{"key": "monkey-param"},
					Version:  builds.Version{"version": "monkey-performed"},
					Metadata: []builds.MetadataField{{Name: "output", Value: "monkey"}},
				}

				bananaResult := builds.Output{
					Name:     "banana",
					Type:     "some-type",
					Source:   builds.Source{"uri": "http://banana-uri"},
					Params:   builds.Params{"key": "banana-param"},
					Version:  builds.Version{"version": "banana-performed"},
					Metadata: []builds.MetadataField{{Name: "output", Value: "banana"}},
				}

				Ω(performedOutputs).Should(ContainElement(monkeyResult))
				Ω(events.Sent()).Should(ContainElement(event.Output{monkeyResult}))

				Ω(performedOutputs).Should(ContainElement(bananaResult))
				Ω(events.Sent()).Should(ContainElement(event.Output{bananaResult}))
			})

			It("releases each resource", func() {
				Ω(tracker.ReleaseCallCount()).Should(Equal(2))

				allReleased := []resource.Resource{
					tracker.ReleaseArgsForCall(0),
					tracker.ReleaseArgsForCall(1),
				}

				Ω(allReleased).Should(ContainElement(resource1))
				Ω(allReleased).Should(ContainElement(resource2))
			})
		})

		Context("when an output fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				resource1.OutReturns(builds.Output{Name: "failed-output"}, disaster)
			})

			It("returns an error", func() {
				Ω(performErr).Should(Equal(disaster))
			})

			It("emits an error event", func() {
				_, output := resource1.OutArgsForCall(0)

				Eventually(events.Sent).Should(ContainElement(event.Error{
					Message: output.Name + " output failed: oh no!",
				}))
			})

			It("does not emit a bogus output event", func() {
				Ω(events.Sent()).ShouldNot(ContainElement(event.Output{
					builds.Output{Name: "failed-output"},
				}))
			})

			It("releases each resource", func() {
				Ω(tracker.ReleaseCallCount()).Should(Equal(2))

				allReleased := []resource.Resource{
					tracker.ReleaseArgsForCall(0),
					tracker.ReleaseArgsForCall(1),
				}

				Ω(allReleased).Should(ContainElement(resource1))
				Ω(allReleased).Should(ContainElement(resource2))
			})
		})

		Describe("when the outputs emit logs", func() {
			BeforeEach(func() {
				for i, resource := range []*rfakes.FakeResource{resource1, resource2} {
					idx := i

					resource.OutStub = func(src io.Reader, output builds.Output) (builds.Output, error) {
						defer GinkgoRecover()

						_, logs, _ := tracker.InitArgsForCall(idx)

						Ω(logs).ShouldNot(BeNil())
						logs.Write([]byte("hello from outputter"))

						return output, nil
					}
				}
			})

			It("emits output events", func() {
				Eventually(events.Sent).Should(ContainElement(event.Log{
					Payload: "hello from outputter",
					Origin: event.Origin{
						Type: event.OriginTypeOutput,
						Name: "monkey",
					},
				}))

				Eventually(events.Sent).Should(ContainElement(event.Log{
					Payload: "hello from outputter",
					Origin: event.Origin{
						Type: event.OriginTypeOutput,
						Name: "banana",
					},
				}))
			})
		})

		Context("when the build is aborted", func() {
			errAborted := errors.New("aborted!")

			BeforeEach(func() {
				resource1.OutStub = func(io.Reader, builds.Output) (builds.Output, error) {
					// return abort error to simulate fetching being aborted;
					// assert that the channel closed below
					return builds.Output{}, errAborted
				}
			})

			It("aborts all resource activity", func() {
				Ω(performErr).Should(Equal(errAborted))

				close(abort)

				_, _, resourceAbort := tracker.InitArgsForCall(0)
				Ω(resourceAbort).Should(BeClosed())
			})
		})
	})

	Context("when streaming out fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			wardenClient.Connection.StreamOutReturns(nil, disaster)
		})

		It("returns the error", func() {
			Ω(performErr).Should(Equal(disaster))
		})
	})
})

// TODO dedup this from builder
type eventLog struct {
	events  []event.Event
	eventsL sync.RWMutex
}

func (l *eventLog) Add(e event.Event) {
	l.eventsL.Lock()
	l.events = append(l.events, e)
	l.eventsL.Unlock()
}

func (l *eventLog) Sent() []event.Event {
	l.eventsL.RLock()
	events := make([]event.Event, len(l.events))
	copy(events, l.events)
	l.eventsL.RUnlock()
	return events
}
