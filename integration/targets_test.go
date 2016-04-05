package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyCmd *exec.Cmd
		flyrc  string
	)

	BeforeEach(func() {
		flyrc = filepath.Join(userHomeDir(), ".flyrc")

		flyFixtureFile, err := os.OpenFile("./fixtures/flyrc.yml", os.O_RDONLY, 0600)
		Expect(err).NotTo(HaveOccurred())

		flyFixtureData, err := ioutil.ReadAll(flyFixtureFile)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(flyrc, flyFixtureData, 0600)
		Expect(err).NotTo(HaveOccurred())

		flyCmd = exec.Command(flyPath, "targets")
	})

	AfterEach(func() {
		os.RemoveAll(flyrc)
	})

	Describe("targets", func() {
		Context("when there are targets in the .flyrc", func() {
			It("displays all the targets with their token expiration", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "url", Color: color.New(color.Bold)},
						{Contents: "expiry", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "Fri, 18 Mar 2016 18:54:30 PDT"}},
						{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "n/a"}},
						{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "Sun, 20 Mar 2016 18:54:30 PDT"}},
						{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "Fri, 25 Mar 2016 16:29:57 PDT"}},
					},
				}))
			})
		})

		Context("when no targets are available", func() {
			BeforeEach(func() {
				os.RemoveAll(flyrc)
			})

			It("prints an empty table", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "url", Color: color.New(color.Bold)},
						{Contents: "expiry", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{}}))
			})
		})
	})
})
