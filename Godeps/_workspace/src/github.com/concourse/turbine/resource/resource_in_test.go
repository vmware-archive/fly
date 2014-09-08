package resource_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/ioutil"

	"github.com/cloudfoundry-incubator/garden/warden"
	wfakes "github.com/cloudfoundry-incubator/garden/warden/fakes"
	"github.com/concourse/turbine/api/builds"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource In", func() {
	var (
		input builds.Input

		inScriptStdout     string
		inScriptStderr     string
		inScriptExitStatus int
		runInError         error

		inScriptProcess *wfakes.FakeProcess

		inStream io.Reader
		inConfig builds.Config
		inInput  builds.Input
		inErr    error
	)

	BeforeEach(func() {
		input = builds.Input{
			Name:    "some-name",
			Type:    "some-resource",
			Source:  builds.Source{"some": "source"},
			Version: builds.Version{"some": "version"},
			Params:  builds.Params{"some": "params"},
		}

		inScriptStdout = "{}"
		inScriptStderr = ""
		inScriptExitStatus = 0
		runInError = nil

		inScriptProcess = new(wfakes.FakeProcess)
		inScriptProcess.WaitStub = func() (int, error) {
			return inScriptExitStatus, nil
		}
	})

	JustBeforeEach(func() {
		wardenClient.Connection.RunStub = func(handle string, spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
			if runInError != nil {
				return nil, runInError
			}

			_, err := io.Stdout.Write([]byte(inScriptStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(inScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return inScriptProcess, nil
		}

		inStream, inInput, inConfig, inErr = resource.In(input)
	})

	It("runs /opt/resource/in <destination> with the request on stdin", func() {
		Ω(inErr).ShouldNot(HaveOccurred())

		handle, spec, io := wardenClient.Connection.RunArgsForCall(0)
		Ω(handle).Should(Equal("some-handle"))
		Ω(spec.Path).Should(Equal("/opt/resource/in"))
		Ω(spec.Args).Should(Equal([]string{"/tmp/build/src/some-name"}))
		Ω(spec.Privileged).Should(BeTrue())

		request, err := ioutil.ReadAll(io.Stdin)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(request).Should(MatchJSON(`{
			"source": {"some":"source"},
			"params": {"some":"params"},
			"version": {"some":"version"}
		}`))
	})

	Context("when /opt/resource/in prints the source", func() {
		BeforeEach(func() {
			inScriptStdout = `{
					"version": {"some": "new-version"},
					"metadata": [
						{"name": "a", "value":"a-value"},
						{"name": "b","value": "b-value"}
					]
				}`
		})

		It("returns the build source printed out by /opt/resource/in", func() {
			expectedFetchedInput := input
			expectedFetchedInput.Version = builds.Version{"some": "new-version"}
			expectedFetchedInput.Metadata = []builds.MetadataField{
				{Name: "a", Value: "a-value"},
				{Name: "b", Value: "b-value"},
			}

			Ω(inInput).Should(Equal(expectedFetchedInput))
		})
	})

	Context("when /in outputs to stderr", func() {
		BeforeEach(func() {
			inScriptStderr = "some stderr data"
		})

		It("emits it to the log sink", func() {
			Ω(inErr).ShouldNot(HaveOccurred())

			Ω(string(logs.Contents())).Should(Equal("some stderr data"))
		})
	})

	Context("when streaming out succeeds", func() {
		BeforeEach(func() {
			wardenClient.Connection.StreamOutStub = func(handle string, source string) (io.ReadCloser, error) {
				Ω(handle).Should(Equal("some-handle"))

				streamOut := new(bytes.Buffer)

				if source == "/tmp/build/src/some-name/" {
					streamOut.WriteString("sup")
				}

				return ioutil.NopCloser(streamOut), nil
			}
		})

		It("returns the output stream of /tmp/build/src/some-name/", func() {
			contents, err := ioutil.ReadAll(inStream)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(string(contents)).Should(Equal("sup"))
		})
	})

	Context("when a config path is specified", func() {
		BeforeEach(func() {
			input.ConfigPath = "some/config/path.yml"
		})

		Context("and the config path exists", func() {
			BeforeEach(func() {
				wardenClient.Connection.StreamOutStub = func(handle string, src string) (io.ReadCloser, error) {
					Ω(handle).Should(Equal("some-handle"))

					buf := new(bytes.Buffer)

					if src == "/tmp/build/src/some-name/some/config/path.yml" {
						tarWriter := tar.NewWriter(buf)

						contents := []byte("---\nimage: some-reconfigured-image\n")

						tarWriter.WriteHeader(&tar.Header{
							Name: "./doesnt-matter",
							Mode: 0644,
							Size: int64(len(contents)),
						})

						tarWriter.Write(contents)
					}

					return ioutil.NopCloser(buf), nil
				}
			})

			It("is parsed and returned as a Build", func() {
				Ω(inConfig.Image).Should(Equal("some-reconfigured-image"))
			})

			Context("but the output is invalid", func() {
				BeforeEach(func() {
					wardenClient.Connection.StreamOutStub = func(handle string, src string) (io.ReadCloser, error) {
						Ω(handle).Should(Equal("some-handle"))

						buf := new(bytes.Buffer)

						if src == "/tmp/build/src/some-name/some/config/path.yml" {
							tarWriter := tar.NewWriter(buf)

							contents := []byte("[")

							tarWriter.WriteHeader(&tar.Header{
								Name: "./doesnt-matter",
								Mode: 0644,
								Size: int64(len(contents)),
							})

							tarWriter.Write(contents)
						}

						return ioutil.NopCloser(buf), nil
					}
				})

				It("returns an error", func() {
					Ω(inErr).Should(HaveOccurred())
				})
			})
		})

		Context("when the config cannot be fetched", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				wardenClient.Connection.StreamOutReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(inErr).Should(Equal(disaster))
			})
		})

		Context("when the config path does not exist", func() {
			BeforeEach(func() {
				wardenClient.Connection.StreamOutStub = func(string, string) (io.ReadCloser, error) {
					return ioutil.NopCloser(new(bytes.Buffer)), nil
				}
			})

			It("returns an error", func() {
				Ω(inErr).Should(HaveOccurred())
			})
		})
	})

	Context("when running /opt/resource/in fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			runInError = disaster
		})

		It("returns an err containing stdout/stderr of the process", func() {
			Ω(inErr).Should(Equal(disaster))
		})
	})

	Context("when /opt/resource/in exits nonzero", func() {
		BeforeEach(func() {
			inScriptStdout = "some-stdout-data"
			inScriptStderr = "some-stderr-data"
			inScriptExitStatus = 9
		})

		It("returns an err containing stdout/stderr of the process", func() {
			Ω(inErr).Should(HaveOccurred())
			Ω(inErr.Error()).Should(ContainSubstring("some-stdout-data"))
			Ω(inErr.Error()).Should(ContainSubstring("some-stderr-data"))
			Ω(inErr.Error()).Should(ContainSubstring("exit status 9"))
		})
	})

	Context("when streaming out fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			wardenClient.Connection.StreamOutReturns(nil, disaster)
		})

		It("returns the error", func() {
			Ω(inErr).Should(Equal(disaster))
		})
	})

	Context("when aborting", func() {
		var waited chan<- struct{}

		BeforeEach(func() {
			waiting := make(chan struct{})
			waited = waiting

			inScriptProcess.WaitStub = func() (int, error) {
				// cause waiting to block so that it can be aborted
				<-waiting
				return 0, nil
			}

			close(abort)
		})

		It("stops the container", func() {
			Eventually(wardenClient.Connection.StopCallCount).Should(Equal(1))

			handle, kill := wardenClient.Connection.StopArgsForCall(0)
			Ω(handle).Should(Equal("some-handle"))
			Ω(kill).Should(BeFalse())

			close(waited)
		})
	})
})
