package scheduler_test

import (
	"bytes"
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
	. "github.com/concourse/turbine/scheduler"
)

var _ = Describe("Scheduler", func() {
	var fakeBuilder *bfakes.FakeBuilder
	var scheduler Scheduler

	var build builds.Build

	BeforeEach(func() {
		fakeBuilder = new(bfakes.FakeBuilder)
		scheduler = NewScheduler(lagertest.NewTestLogger("test"), fakeBuilder)

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
		}
	})

	Describe("Schedule", func() {
		It("kicks off a builder", func() {
			scheduler.Start(build)

			Eventually(fakeBuilder.StartCallCount).Should(Equal(1))

			build, _ := fakeBuilder.StartArgsForCall(0)
			Ω(build).Should(Equal(build))
		})

		Context("when there is a callback registered", func() {
			var callbackServer *ghttp.Server

			BeforeEach(func() {
				callbackServer = ghttp.NewServer()
				build.Callback = callbackServer.URL() + "/abc"
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

				itRetries(0, func() {
					Eventually(gotStartedCallback, 3).Should(BeClosed())
				})

				Context("when the build succeeds", func() {
					var succeeded builder.SucceededBuild

					BeforeEach(func() {
						succeeded = builder.SucceededBuild{
							Build: build,
						}

						succeeded.Build.Status = builds.StatusSucceeded

						fakeBuilder.AttachReturns(succeeded, nil, nil)
					})

					It("completes the build", func() {
						scheduler.Start(build)

						Eventually(fakeBuilder.CompleteCallCount).Should(Equal(1))

						completing, _ := fakeBuilder.CompleteArgsForCall(0)
						Ω(completing).Should(Equal(succeeded))
					})

					Context("and the build completes", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.CompleteReturns(running.Build, nil)

							gotRequest = handleBuild(succeeded.Build)
						})

						It("reports the started build as succeeded", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})

					Context("and the build fails to complete", func() {
						var gotRequest <-chan struct{}

						BeforeEach(func() {
							fakeBuilder.CompleteReturns(builds.Build{}, errors.New("oh no!"))

							erroredBuild := running.Build
							erroredBuild.Status = builds.StatusErrored

							gotRequest = handleBuild(erroredBuild)
						})

						It("reports the started build as errored", func() {
							scheduler.Start(build)

							Eventually(gotRequest).Should(BeClosed())
						})

						itRetries(1, func() {
							Eventually(gotRequest, 3).Should(BeClosed())
						})
					})
				})

				Context("when the build fails", func() {
					var gotRequest <-chan struct{}

					BeforeEach(func() {
						fakeBuilder.AttachReturns(builder.SucceededBuild{}, errors.New("exit status 1"), nil)

						failedBuild := running.Build
						failedBuild.Status = builds.StatusFailed

						gotRequest = handleBuild(failedBuild)
					})

					It("reports the build as failed", func() {
						scheduler.Start(build)

						Eventually(gotRequest).Should(BeClosed())
					})

					itRetries(1, func() {
						Eventually(gotRequest, 3).Should(BeClosed())
					})
				})

				Context("when building fails", func() {
					var gotRequest <-chan struct{}

					BeforeEach(func() {
						fakeBuilder.AttachReturns(builder.SucceededBuild{}, nil, errors.New("oh no!"))

						erroredBuild := running.Build
						erroredBuild.Status = builds.StatusErrored

						gotRequest = handleBuild(erroredBuild)
					})

					It("reports the build as errored", func() {
						scheduler.Start(build)

						Eventually(gotRequest).Should(BeClosed())
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

				fakeBuilder.StartStub = func(builds.Build, <-chan struct{}) (builder.RunningBuild, error) {
					return running, nil
				}

				fakeBuilder.AttachStub = func(build builder.RunningBuild, abort <-chan struct{}) (builder.SucceededBuild, error, error) {
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
						TTY:  true,
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

				fakeBuilder.StartStub = func(build builds.Build, abort <-chan struct{}) (builder.RunningBuild, error) {
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

				fakeBuilder.StartStub = func(build builds.Build, abort <-chan struct{}) (builder.RunningBuild, error) {
					return builder.RunningBuild{Build: build}, nil
				}

				fakeBuilder.AttachStub = func(build builder.RunningBuild, abort <-chan struct{}) (builder.SucceededBuild, error, error) {
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

				fakeBuilder.StartStub = func(build builds.Build, abort <-chan struct{}) (builder.RunningBuild, error) {
					return builder.RunningBuild{Build: build}, nil
				}

				fakeBuilder.AttachStub = func(running builder.RunningBuild, abort <-chan struct{}) (builder.SucceededBuild, error, error) {
					return builder.SucceededBuild{Build: running.Build}, nil, nil
				}

				fakeBuilder.CompleteStub = func(build builder.SucceededBuild, abort <-chan struct{}) (builds.Build, error) {
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

				fakeBuilder.StartStub = func(build builds.Build, abort <-chan struct{}) (builder.RunningBuild, error) {
					if build.Guid == "starting" {
						return <-running, nil
					}

					return builder.RunningBuild{
						Build: build,
					}, nil
				}

				fakeBuilder.AttachStub = func(running builder.RunningBuild, abort <-chan struct{}) (builder.SucceededBuild, error, error) {
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

					fakeBuilder.StartStub = func(builds.Build, <-chan struct{}) (builder.RunningBuild, error) {
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

				fakeBuilder.AttachStub = func(builder.RunningBuild, <-chan struct{}) (builder.SucceededBuild, error, error) {
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

				fakeBuilder.CompleteStub = func(builder.SucceededBuild, <-chan struct{}) (builds.Build, error) {
					close(completing)
					return <-finished, nil
				}
			})

			It("waits for it to error and does not return the completed build", func() {
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
