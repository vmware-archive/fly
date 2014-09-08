package scheduler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/cloudfoundry-incubator/garden/warden"
	wfakes "github.com/cloudfoundry-incubator/garden/warden/fakes"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/builder"
	bfakes "github.com/concourse/turbine/builder/fakes"
	"github.com/concourse/turbine/event"
	efakes "github.com/concourse/turbine/event/fakes"
	. "github.com/concourse/turbine/scheduler"
)

var _ = Describe("Scheduler", func() {
	var fakeBuilder *bfakes.FakeBuilder
	var emitter *efakes.FakeEmitter
	var emittedEvents <-chan event.Event
	var scheduler Scheduler

	var build builds.Build

	BeforeEach(func() {
		fakeBuilder = new(bfakes.FakeBuilder)

		emitter = new(efakes.FakeEmitter)

		events := make(chan event.Event, 100)
		emittedEvents = events

		emitter.EmitEventStub = func(e event.Event) {
			payload, err := json.Marshal(event.Message{e})
			Ω(err).ShouldNot(HaveOccurred())

			var duped event.Message
			err = json.Unmarshal(payload, &duped)
			Ω(err).ShouldNot(HaveOccurred())

			events <- duped.Event
		}

		createEmitter := func(logsURL string, draining <-chan struct{}) event.Emitter {
			defer GinkgoRecover()

			if logsURL == "" {
				return event.NullEmitter{}
			}

			Ω(logsURL).Should(Equal("some-events-callback"))
			Ω(draining).ShouldNot(BeNil())

			return emitter
		}

		scheduler = NewScheduler(lagertest.NewTestLogger("test"), fakeBuilder, createEmitter)

		build = builds.Build{
			Guid: "abc",

			Inputs: []builds.Input{
				{
					Type: "git",
				},
			},

			Config: builds.Config{
				Params: map[string]string{
					"FOO":  "bar",
					"FIZZ": "buzz",
				},
			},

			EventsCallback: "some-events-callback",
		}
	})

	Describe("Start", func() {
		It("kicks off a builder", func() {
			scheduler.Start(build)

			Eventually(fakeBuilder.StartCallCount).Should(Equal(1))

			startedBuild, startedEmitter, _ := fakeBuilder.StartArgsForCall(0)
			Ω(startedBuild).Should(Equal(build))
			Ω(startedEmitter).Should(Equal(emitter))
		})

		Context("when there is a callback registered", func() {
			var callbackServer *ghttp.Server

			BeforeEach(func() {
				callbackServer = ghttp.NewServer()
				build.StatusCallback = callbackServer.URL() + "/abc"
			})

			handleBuild := func(build builds.Build) <-chan struct{} {
				gotRequest := make(chan struct{})

				callbackServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/abc"),
						ghttp.VerifyJSONRepresenting(build),
						ghttp.RespondWith(http.StatusOK, ""),
						func(http.ResponseWriter, *http.Request) {
							close(gotRequest)
						},
					),
				)

				return gotRequest
			}

			itRetries := func(index int, assertion func()) {
				Context("and the callback URI fails", func() {
					BeforeEach(func() {
						handler := callbackServer.GetHandler(index)

						callbackServer.SetHandler(index, func(w http.ResponseWriter, r *http.Request) {
							callbackServer.HTTPTestServer.CloseClientConnections()
						})

						callbackServer.AppendHandlers(handler)
					})

					It("retries", func() {
						// ignore completion callback
						callbackServer.AllowUnhandledRequests = true

						scheduler.Start(build)

						assertion()
					})
				})
			}

			Context("and the build starts", func() {
				var running builder.RunningBuild
				var gotStartedCallback <-chan struct{}

				BeforeEach(func() {
					running = builder.RunningBuild{
						Build: build,
					}

					running.Build.Status = builds.StatusStarted

					fakeBuilder.StartReturns(running, nil)

					gotStartedCallback = handleBuild(running.Build)
				})

				It("reports the started build as started", func() {
					// ignore completion callback
					callbackServer.AllowUnhandledRequests = true

					scheduler.Start(build)

					Eventually(gotStartedCallback).Should(BeClosed())
				})

				It("emits a started status event", func() {
					scheduler.Start(build)

					Eventually(emittedEvents).Should(Receive(Equal(event.Status{
						Status: builds.StatusStarted,
					})))
				})

				itRetries(0, func() {
					Eventually(gotStartedCallback, 3).Should(BeClosed())
				})

				Context("when the build exits 0", func() {
					var exited builder.ExitedBuild

					BeforeEach(func() {
						exited = builder.ExitedBuild{
							Build: build,

							ExitStatus: 0,
						}

						fakeBuilder.AttachReturns(exited, nil)
					})

					It("finishes the build", func() {
						scheduler.Start(build)

						Eventually(fakeBuilder.FinishCallCount).Should(Equal(1))

						completing, completingEmitter, _ := fakeBuilder.FinishArgsForCall(0)
						Ω(completing).Should(Equal(exited))
						Ω(completingEmitter).Should(Equal(emitter))
					})

					Context("and the build finishes", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.FinishReturns(running.Build, nil)

							succeededBuild := exited.Build
							succeededBuild.Status = builds.StatusSucceeded

							gotRequest = handleBuild(succeededBuild)
						})

						It("reports the started build as succeeded", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						It("emits a succeeded status event", func() {
							scheduler.Start(build)

							Eventually(emittedEvents).Should(Receive(Equal(event.Status{
								Status: builds.StatusSucceeded,
							})))
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})

					Context("and the build fails to finish", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.FinishReturns(builds.Build{}, errors.New("oh no!"))

							erroredBuild := running.Build
							erroredBuild.Status = builds.StatusErrored

							gotRequest = handleBuild(erroredBuild)
						})

						It("reports the started build as errored", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						It("emits an errored status event", func() {
							scheduler.Start(build)

							Eventually(emittedEvents).Should(Receive(Equal(event.Status{
								Status: builds.StatusErrored,
							})))
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})
				})

				Context("when the build exited nonzero", func() {
					var exited builder.ExitedBuild

					BeforeEach(func() {
						exited = builder.ExitedBuild{
							Build: build,

							ExitStatus: 2,
						}

						fakeBuilder.AttachReturns(exited, nil)
					})

					It("finishes the build", func() {
						scheduler.Start(build)

						Eventually(fakeBuilder.FinishCallCount).Should(Equal(1))

						completing, completingEmitter, _ := fakeBuilder.FinishArgsForCall(0)
						Ω(completing).Should(Equal(exited))
						Ω(completingEmitter).Should(Equal(emitter))
					})

					Context("and the build finishes", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.FinishReturns(running.Build, nil)

							failedBuild := exited.Build
							failedBuild.Status = builds.StatusFailed

							gotRequest = handleBuild(failedBuild)
						})

						It("reports the started build as fao;ed", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						It("emits a failed status event", func() {
							scheduler.Start(build)

							Eventually(emittedEvents).Should(Receive(Equal(event.Status{
								Status: builds.StatusFailed,
							})))
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})

					Context("and the build fails to finish", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.FinishReturns(builds.Build{}, errors.New("oh no!"))

							erroredBuild := running.Build
							erroredBuild.Status = builds.StatusErrored

							gotRequest = handleBuild(erroredBuild)
						})

						It("reports the started build as errored", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						It("emits an errored status event", func() {
							scheduler.Start(build)

							Eventually(emittedEvents).Should(Receive(Equal(event.Status{
								Status: builds.StatusErrored,
							})))
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})
				})

				Context("when building fails", func() {
					var gotRequest <-chan struct{}

					BeforeEach(func() {
						fakeBuilder.AttachReturns(builder.ExitedBuild{}, errors.New("oh no!"))

						erroredBuild := running.Build
						erroredBuild.Status = builds.StatusErrored

						gotRequest = handleBuild(erroredBuild)
					})

					It("reports the build as errored", func() {
						scheduler.Start(build)

						Eventually(gotRequest).Should(BeClosed())
					})

					It("emits an errored status event", func() {
						scheduler.Start(build)

						Eventually(emittedEvents).Should(Receive(Equal(event.Status{
							Status: builds.StatusErrored,
						})))
					})

					itRetries(1, func() {
						Eventually(gotRequest, 3).Should(BeClosed())
					})
				})
			})

			Context("and the build fails to start", func() {
				var gotRequest <-chan struct{}

				BeforeEach(func() {
					fakeBuilder.StartReturns(builder.RunningBuild{}, errors.New("oh no!"))

					erroredBuild := build
					erroredBuild.Status = builds.StatusErrored

					gotRequest = handleBuild(erroredBuild)
				})

				It("reports the build as errored", func() {
					scheduler.Start(build)

					Eventually(gotRequest).Should(BeClosed())
				})

				It("emits an errored status event", func() {
					scheduler.Start(build)

					Eventually(emittedEvents).Should(Receive(Equal(event.Status{
						Status: builds.StatusErrored,
					})))
				})

				itRetries(0, func() {
					Eventually(gotRequest, 3).Should(BeClosed())
				})
			})
		})
	})

	Describe("Hijack", func() {
		Context("when the build is not running", func() {
			It("returns an error", func() {
				_, err := scheduler.Hijack("bogus-guid", warden.ProcessSpec{}, warden.ProcessIO{})
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the build is running", func() {
			var attaching chan struct{}
			var running builder.RunningBuild

			BeforeEach(func() {
				attaching = make(chan struct{})

				build.Status = builds.StatusStarted

				running = builder.RunningBuild{
					Build:           build,
					ContainerHandle: "some-handle",
					ProcessID:       42,
				}

				fakeBuilder.StartStub = func(builds.Build, event.Emitter, <-chan struct{}) (builder.RunningBuild, error) {
					return running, nil
				}

				fakeBuilder.AttachStub = func(build builder.RunningBuild, emitter event.Emitter, abort <-chan struct{}) (builder.ExitedBuild, error) {
					close(attaching)
					select {}
				}
			})

			Context("when hijacking succeeds", func() {
				BeforeEach(func() {
					fakeProcess := new(wfakes.FakeProcess)
					fakeProcess.WaitReturns(42, nil)

					fakeBuilder.HijackReturns(fakeProcess, nil)
				})

				It("hijacks via the builder", func() {
					scheduler.Start(build)

					Eventually(attaching).Should(BeClosed())

					spec := warden.ProcessSpec{
						Path: "process-path",
						Args: []string{"process", "args"},
						TTY: &warden.TTYSpec{
							WindowSize: &warden.WindowSize{
								Columns: 123,
								Rows:    456,
							},
						},
					}

					io := warden.ProcessIO{
						Stdin:  new(bytes.Buffer),
						Stdout: new(bytes.Buffer),
					}

					process, err := scheduler.Hijack(running.Build.Guid, spec, io)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeBuilder.HijackCallCount()).Should(Equal(1))

					build, spec, io := fakeBuilder.HijackArgsForCall(0)
					Ω(build).Should(Equal(running))
					Ω(spec).Should(Equal(spec))
					Ω(io).Should(Equal(io))

					Ω(process.Wait()).Should(Equal(42))
				})
			})

			Context("when hijacking fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					fakeBuilder.HijackReturns(nil, disaster)
				})

				It("returns the error", func() {
					scheduler.Start(build)

					Eventually(attaching).Should(BeClosed())

					spec := warden.ProcessSpec{}

					io := warden.ProcessIO{}

					_, err := scheduler.Hijack(running.Build.Guid, spec, io)
					Ω(err).Should(Equal(disaster))
				})
			})
		})
	})

	Describe("Abort", func() {
		Context("when starting a build", func() {
			var gotAborting chan (<-chan struct{})

			BeforeEach(func() {
				gotAborting = make(chan (<-chan struct{}), 1)

				fakeBuilder.StartStub = func(build builds.Build, emitter event.Emitter, abort <-chan struct{}) (builder.RunningBuild, error) {
					gotAborting <- abort
					select {}
				}
			})

			It("signals to the builder to abort", func() {
				scheduler.Start(build)

				var abort <-chan struct{}
				Eventually(gotAborting).Should(Receive(&abort))

				scheduler.Abort(build.Guid)

				Ω(abort).Should(BeClosed())
			})
		})

		Context("when attached to a build", func() {
			var gotAborting chan (<-chan struct{})

			BeforeEach(func() {
				gotAborting = make(chan (<-chan struct{}), 1)

				fakeBuilder.StartStub = func(build builds.Build, emitter event.Emitter, abort <-chan struct{}) (builder.RunningBuild, error) {
					return builder.RunningBuild{Build: build}, nil
				}

				fakeBuilder.AttachStub = func(build builder.RunningBuild, emitter event.Emitter, abort <-chan struct{}) (builder.ExitedBuild, error) {
					gotAborting <- abort
					select {}
				}
			})

			It("signals to the builder to abort", func() {
				scheduler.Start(build)

				var abort <-chan struct{}
				Eventually(gotAborting).Should(Receive(&abort))

				scheduler.Abort(build.Guid)

				Ω(abort).Should(BeClosed())
			})
		})

		Context("when completing a build", func() {
			var gotAborting chan (<-chan struct{})

			BeforeEach(func() {
				gotAborting = make(chan (<-chan struct{}), 1)

				fakeBuilder.StartStub = func(build builds.Build, emitter event.Emitter, abort <-chan struct{}) (builder.RunningBuild, error) {
					return builder.RunningBuild{Build: build}, nil
				}

				fakeBuilder.AttachStub = func(running builder.RunningBuild, emitter event.Emitter, abort <-chan struct{}) (builder.ExitedBuild, error) {
					return builder.ExitedBuild{Build: running.Build}, nil
				}

				fakeBuilder.FinishStub = func(build builder.ExitedBuild, emitter event.Emitter, abort <-chan struct{}) (builds.Build, error) {
					gotAborting <- abort
					select {}
				}
			})

			It("signals to the builder to abort", func() {
				scheduler.Start(build)

				var abort <-chan struct{}
				Eventually(gotAborting).Should(Receive(&abort))

				scheduler.Abort(build.Guid)

				Ω(abort).Should(BeClosed())
			})
		})
	})

	Describe("Drain", func() {
		Context("when a build is starting", func() {
			startingBuild := builds.Build{
				Guid: "starting",
			}

			var running chan builder.RunningBuild

			BeforeEach(func() {
				running = make(chan builder.RunningBuild)

				fakeBuilder.StartStub = func(build builds.Build, emitter event.Emitter, abort <-chan struct{}) (builder.RunningBuild, error) {
					if build.Guid == "starting" {
						return <-running, nil
					}

					return builder.RunningBuild{
						Build: build,
					}, nil
				}

				fakeBuilder.AttachStub = func(running builder.RunningBuild, emitter event.Emitter, abort <-chan struct{}) (builder.ExitedBuild, error) {
					select {}
				}
			})

			It("waits for it to start running and returns its running state", func() {
				scheduler.Start(startingBuild)

				startedBuild := startingBuild
				startedBuild.Status = builds.StatusStarted
				runningBuild := builder.RunningBuild{Build: startedBuild}

				drained := make(chan []builder.RunningBuild)

				go func() {
					drained <- scheduler.Drain()
				}()

				Consistently(drained).ShouldNot(Receive())

				running <- runningBuild

				Eventually(drained).Should(Receive(Equal([]builder.RunningBuild{runningBuild})))
			})

			Context("and it errors", func() {
				var errored chan error

				BeforeEach(func() {
					errored = make(chan error)

					fakeBuilder.StartStub = func(builds.Build, event.Emitter, <-chan struct{}) (builder.RunningBuild, error) {
						return builder.RunningBuild{}, <-errored
					}
				})

				It("waits for it to error and does not return it", func() {
					scheduler.Start(build)

					drained := make(chan []builder.RunningBuild)

					go func() {
						drained <- scheduler.Drain()
					}()

					Consistently(drained).ShouldNot(Receive())

					errored <- errors.New("oh no!")

					Eventually(drained).Should(Receive(BeEmpty()))
				})
			})
		})

		Context("when a build is running", func() {
			var running chan struct{}

			BeforeEach(func() {
				running = make(chan struct{})

				fakeBuilder.AttachStub = func(builder.RunningBuild, event.Emitter, <-chan struct{}) (builder.ExitedBuild, error) {
					close(running)
					select {}
				}
			})

			It("does not wait for it to finish", func() {
				scheduler.Start(build)

				Eventually(running).Should(BeClosed())

				drained := make(chan []builder.RunningBuild)

				go func() {
					drained <- scheduler.Drain()
				}()

				Eventually(drained).Should(Receive(HaveLen(1)))
			})
		})

		Context("when a build is completing", func() {
			var completing chan struct{}
			var finished chan builds.Build

			BeforeEach(func() {
				completing = make(chan struct{})
				finished = make(chan builds.Build)

				fakeBuilder.FinishStub = func(builder.ExitedBuild, event.Emitter, <-chan struct{}) (builds.Build, error) {
					close(completing)
					return <-finished, nil
				}
			})

			It("waits for it to error and does not return the finished build", func() {
				scheduler.Start(build)

				Eventually(completing).Should(BeClosed())

				drained := make(chan []builder.RunningBuild)

				go func() {
					drained <- scheduler.Drain()
				}()

				Consistently(drained).ShouldNot(Receive())

				finished <- builds.Build{}

				Eventually(drained).Should(Receive(BeEmpty()))
			})
		})

		Context("when no builds are being scheduled", func() {
			It("returns an empty slice", func() {
				Ω(scheduler.Drain()).Should(BeEmpty())
			})
		})
	})
})
