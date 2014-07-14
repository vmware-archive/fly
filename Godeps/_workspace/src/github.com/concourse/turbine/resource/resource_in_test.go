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

		inStdout     string
		inStderr     string
		inExitStatus int
		inError      error

		fetchedStream io.Reader
		fetchedConfig builds.Config
		fetchedInput  builds.Input
		fetchError    error
	)

	BeforeEach(func() {
		input = builds.Input{
			Name:    "some-name",
			Type:    "some-resource",
			Source:  builds.Source{"some": "source"},
			Version: builds.Version{"some": "version"},
			Params:  builds.Params{"some": "params"},
		}

		inStdout = "{}"
		inStderr = ""
		inExitStatus = 0
		inError = nil
	})

	JustBeforeEach(func() {
		wardenClient.Connection.RunStub = func(handle string, spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
			if inError != nil {
				return nil, inError
			}

			_, err := io.Stdout.Write([]byte(inStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(inStderr))
			Ω(err).ShouldNot(HaveOccurred())

			process := new(wfakes.FakeProcess)
			process.WaitReturns(inExitStatus, nil)

			return process, nil
		}

		fetchedStream, fetchedInput, fetchedConfig, fetchError = resource.In(input)
	})

	It("runs /opt/resource/in <destination> with the request on stdin", func() {
		Ω(fetchError).ShouldNot(HaveOccurred())

		handle, spec, io := wardenClient.Connection.RunArgsForCall(0)
		Ω(handle).Should(Equal("some-handle"))
		Ω(spec.Path).Should(Equal("/opt/resource/in"))
		Ω(spec.Args).Should(Equal([]string{"/tmp/build/src/some-name"}))
		Ω(spec.Privileged).Should(BeTrue())

		request, err := ioutil.ReadAll(io.Stdin)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(string(request)).Should(Equal(`{"version":{"some":"version"},"source":{"some":"source"},"params":{"some":"params"}}`))
	})

	Context("when /opt/resource/in prints the source", func() {
		BeforeEach(func() {
			inStdout = `{
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

			Ω(fetchedInput).Should(Equal(expectedFetchedInput))
		})
	})

	Context("when /in outputs to stderr", func() {
		BeforeEach(func() {
			inStderr = "some stderr data"
		})

		It("emits it to the log sink", func() {
			Ω(fetchError).ShouldNot(HaveOccurred())

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
			contents, err := ioutil.ReadAll(fetchedStream)
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
				Ω(fetchedConfig.Image).Should(Equal("some-reconfigured-image"))
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
					Ω(fetchError).Should(HaveOccurred())
				})
			})
		})

		Context("when the config cannot be fetched", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				wardenClient.Connection.StreamOutReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(fetchError).Should(Equal(disaster))
			})
		})

		Context("when the config path does not exist", func() {
			BeforeEach(func() {
				wardenClient.Connection.StreamOutStub = func(string, string) (io.ReadCloser, error) {
					return ioutil.NopCloser(new(bytes.Buffer)), nil
				}
			})

			It("returns an error", func() {
				Ω(fetchError).Should(HaveOccurred())
			})
		})
	})

	Context("when running /opt/resource/in fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			inError = disaster
		})

		It("returns an err containing stdout/stderr of the process", func() {
			Ω(fetchError).Should(Equal(disaster))
		})
	})

	Context("when /opt/resource/in exits nonzero", func() {
		BeforeEach(func() {
			inStdout = "some-stdout-data"
			inStderr = "some-stderr-data"
			inExitStatus = 9
		})

		It("returns an err containing stdout/stderr of the process", func() {
			Ω(fetchError).Should(HaveOccurred())
			Ω(fetchError.Error()).Should(ContainSubstring("some-stdout-data"))
			Ω(fetchError.Error()).Should(ContainSubstring("some-stderr-data"))
			Ω(fetchError.Error()).Should(ContainSubstring("exit status 9"))
		})
	})

	Context("when streaming out fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			wardenClient.Connection.StreamOutReturns(nil, disaster)
		})

		It("returns the error", func() {
			Ω(fetchError).Should(Equal(disaster))
		})
	})

	Context("when aborting", func() {
		BeforeEach(func() {
			wardenClient.Connection.RunStub = func(handle string, spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
				process := new(wfakes.FakeProcess)
				process.WaitStub = func() (int, error) {
					// cause waiting to block so that it can be aborted
					select {}
					return 0, nil
				}

				return process, nil
			}
		})

		It("stops the container", func() {
			go resource.In(input)

			close(abort)

			Eventually(wardenClient.Connection.StopCallCount).Should(Equal(1))

			handle, kill := wardenClient.Connection.StopArgsForCall(0)
			Ω(handle).Should(Equal("some-handle"))
			Ω(kill).Should(BeFalse())
		})
	})
})
