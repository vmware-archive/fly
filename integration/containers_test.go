package integration_test

import (
	"os/exec"

	"github.com/concourse/atc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("containers", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "containers")
		})

		Context("when containers are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/containers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Container{
							{
								ID:           "handle-1",
								WorkerName:   "worker-name-1",
								PipelineName: "pipeline-name",
								Type:         "check",
								ResourceName: "git-repo",
							},
							{
								ID:           "early-handle",
								WorkerName:   "worker-name-1",
								PipelineName: "pipeline-name",
								JobName:      "job-name-1",
								BuildName:    "3",
								BuildID:      123,
								Type:         "get",
								StepName:     "git-repo",
								Attempt:      "1.5",
							},
							{
								ID:           "other-handle",
								WorkerName:   "worker-name-2",
								PipelineName: "pipeline-name",
								JobName:      "job-name-2",
								BuildName:    "2",
								BuildID:      122,
								Type:         "task",
								StepName:     "unit-tests",
							},
							{
								ID:         "post-handle",
								WorkerName: "worker-name-3",
								BuildID:    142,
								Type:       "task",
								StepName:   "one-off",
							},
						}),
					),
				)
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
              {
                "id": "handle-1",
                "worker_name": "worker-name-1",
                "type": "check",
                "pipeline_name": "pipeline-name",
                "resource_name": "git-repo"
              },
              {
                "id": "early-handle",
                "worker_name": "worker-name-1",
                "type": "get",
                "step_name": "git-repo",
                "attempt": "1.5",
                "build_id": 123,
                "pipeline_name": "pipeline-name",
                "job_name": "job-name-1",
                "build_name": "3"
              },
              {
                "id": "other-handle",
                "worker_name": "worker-name-2",
                "type": "task",
                "step_name": "unit-tests",
                "build_id": 122,
                "pipeline_name": "pipeline-name",
                "job_name": "job-name-2",
                "build_name": "2"
              },
              {
                "id": "post-handle",
                "worker_name": "worker-name-3",
                "type": "task",
                "step_name": "one-off",
                "build_id": 142
              }
            ]`))
				})
			})

			It("lists them to the user, ordered by name", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "pipeline", Color: color.New(color.Bold)},
						{Contents: "job", Color: color.New(color.Bold)},
						{Contents: "build #", Color: color.New(color.Bold)},
						{Contents: "build id", Color: color.New(color.Bold)},
						{Contents: "type", Color: color.New(color.Bold)},
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "attempt", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "early-handle"}, {Contents: "worker-name-1"}, {Contents: "pipeline-name"}, {Contents: "job-name-1"}, {Contents: "3"}, {Contents: "123"}, {Contents: "get"}, {Contents: "git-repo"}, {Contents: "1.5"}},
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "pipeline-name"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "check"}, {Contents: "git-repo"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "other-handle"}, {Contents: "worker-name-2"}, {Contents: "pipeline-name"}, {Contents: "job-name-2"}, {Contents: "2"}, {Contents: "122"}, {Contents: "task"}, {Contents: "unit-tests"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "post-handle"}, {Contents: "worker-name-3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "142"}, {Contents: "task"}, {Contents: "one-off"}, {Contents: "n/a", Color: color.New(color.Faint)}},
					},
				}))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/containers"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
			})
		})
	})

	Describe("resources", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "resources")
		})

		Context("when check containers summary is returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/checkcontainers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Container{
							{
								ID:               "handle-1",
								WorkerName:       "worker-name-1",
								Type:             "check",
								ResourceConfigID: 432,
								ResourceTypeName: "git",
							},
							{
								ID:               "handle-2",
								WorkerName:       "worker-name-1",
								Type:             "check",
								ResourceConfigID: 433,
								ResourceTypeName: "bitbucket-build-status",
							},
							{
								ID:               "handle-3",
								WorkerName:       "worker-name-2",
								Type:             "check",
								ResourceConfigID: 434,
								ResourceTypeName: "Used by parent resource 433",
							},
						}),
					),
				)
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
	      {
		"id":               "handle-1",
		"worker_name":       "worker-name-1",
		"type":             "check",
		"resource_config_id": 432,
		"resource_type_name":     "git"
	      },
	      {
		"id":               "handle-2",
		"worker_name":       "worker-name-1",
		"type":             "check",
		"resource_config_id": 433,
		"resource_type_name":     "bitbucket-build-status"
	      },
	      {
	 	"id":               "handle-3",
	 	"worker_name":       "worker-name-2",
	 	"type":             "check",
	 	"resource_config_id": 434,
	 	"resource_type_name":     "Used by parent resource 433"
	      }
            ]`))
				})
			})

			It("lists them to the user, ordered by name", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "container-type", Color: color.New(color.Bold)},
						{Contents: "resource-config-id", Color: color.New(color.Bold)},
						{Contents: "resource-type", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "432"}, {Contents: "git"}},
						{{Contents: "handle-2"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "433"}, {Contents: "bitbucket-build-status"}},
						{{Contents: "handle-3"}, {Contents: "worker-name-2"}, {Contents: "check"}, {Contents: "434"}, {Contents: "Used by parent resource 433"}},
					},
				}))
			})
		})

		Context("when check containers details are returned from the API", func() {
			BeforeEach(func() {
				flyCmd.Args = append(flyCmd.Args, "--detailed")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/checkcontainersdetailed"),
						ghttp.RespondWithJSONEncoded(200, []atc.Container{
							{
								ID:               "handle-1",
								WorkerName:       "worker-name-1",
								Type:             "check",
								PipelineID:       1,
								PipelineName:     "mypipeline-1",
								ResourceConfigID: 432,
								ResourceName:     "myresourcename-1",
								ResourceTypeName: "git",
							},
							{
								ID:               "handle-1",
								WorkerName:       "worker-name-1",
								Type:             "check",
								PipelineID:       2,
								PipelineName:     "mypipeline-2",
								ResourceConfigID: 432,
								ResourceName:     "myresourcename-2",
								ResourceTypeName: "git",
							},
							{
								ID:               "handle-1",
								WorkerName:       "worker-name-1",
								Type:             "check",
								PipelineID:       1,
								PipelineName:     "mypipeline-1",
								ResourceConfigID: 432,
								ResourceName:     "myresourcename-2",
								ResourceTypeName: "git",
							},
							{
								ID:               "handle-2",
								WorkerName:       "worker-name-1",
								Type:             "check",
								ResourceConfigID: 433,
								ResourceTypeName: "bitbucket-build-status",
							},
							{
								ID:               "handle-3",
								WorkerName:       "worker-name-2",
								Type:             "check",
								PipelineID:       1,
								PipelineName:     "mypipeline-1",
								ResourceConfigID: 434,
								ResourceName:     "myresourcename-3",
								ResourceTypeName: "Used by parent resource 433",
							},
						}),
					),
				)
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
	      {
		      "id":               "handle-1",
		      "worker_name":       "worker-name-1",
		      "type":             "check",
		      "pipeline_id":       1,
		      "pipeline_name":     "mypipeline-1",
		      "resource_config_id": 432,
		      "resource_name":     "myresourcename-1",
		      "resource_type_name": "git"
	      },
	      {
		      "id":               "handle-1",
		      "worker_name":       "worker-name-1",
		      "type":             "check",
		      "pipeline_id":       2,
		      "pipeline_name":     "mypipeline-2",
		      "resource_config_id": 432,
		      "resource_name":     "myresourcename-2",
		      "resource_type_name": "git"
	      },
	      {
		      "id":               "handle-1",
		      "worker_name":       "worker-name-1",
		      "type":             "check",
		      "pipeline_id":       1,
		      "pipeline_name":     "mypipeline-1",
		      "resource_config_id": 432,
		      "resource_name":     "myresourcename-2",
		      "resource_type_name": "git"
	      },
	      {
		      "id":               "handle-2",
		      "worker_name":       "worker-name-1",
		      "type":             "check",
		      "resource_config_id": 433,
		      "resource_type_name": "bitbucket-build-status"
	      },
	      {
		      "id":               "handle-3",
		      "worker_name":       "worker-name-2",
		      "type":             "check",
		      "pipeline_id":       1,
		      "pipeline_name":     "mypipeline-1",
		      "resource_config_id": 434,
		      "resource_name":     "myresourcename-3",
		      "resource_type_name": "Used by parent resource 433"
	      }
            ]`))
				})
			})

			It("lists them to the user, ordered by name", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "container-type", Color: color.New(color.Bold)},
						{Contents: "resource-config-id", Color: color.New(color.Bold)},
						{Contents: "resource-type", Color: color.New(color.Bold)},
						{Contents: "pipeline", Color: color.New(color.Bold)},
						{Contents: "resource-name", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "432"}, {Contents: "git"}, {Contents: "mypipeline-1"}, {Contents: "myresourcename-1"}},
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "432"}, {Contents: "git"}, {Contents: "mypipeline-2"}, {Contents: "myresourcename-2"}},
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "432"}, {Contents: "git"}, {Contents: "mypipeline-1"}, {Contents: "myresourcename-2"}},
						{{Contents: "handle-2"}, {Contents: "worker-name-1"}, {Contents: "check"}, {Contents: "433"}, {Contents: "bitbucket-build-status"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
						{{Contents: "handle-3"}, {Contents: "worker-name-2"}, {Contents: "check"}, {Contents: "434"}, {Contents: "Used by parent resource 433"}, {Contents: "mypipeline-1"}, {Contents: "myresourcename-3"}},
					},
				}))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/checkcontainers"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
			})
		})
	})
})
