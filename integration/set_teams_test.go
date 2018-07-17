package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyCmd    *exec.Cmd
		cmdParams []string
	)

	JustBeforeEach(func(){
		params := append([]string{"-t", targetName, "set-teams", "-c"}, cmdParams...)
		flyCmd = exec.Command(flyPath, params...)
	})

	Describe("flag validation", func(){

		Describe("config", func() {
			Context("config flag not provided", func() {
				BeforeEach(func() {
					cmdParams = []string{}
				})

				It("returns an error", func() {
					//sess, err := gexec.Start(flyCmd, nil, nil)
					//Expect(err).ToNot(HaveOccurred())
					//Eventually(sess.Err).Should(gbytes.Say("You have not provided a whitelist of users or groups. To continue, run:"))
					//Eventually(sess.Err).Should(gbytes.Say("fly -t testserver set-team -n venture --allow-all-users"))
					//Eventually(sess.Err).Should(gbytes.Say("This will allow team access to all logged in users in the system."))
					//Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})

	Describe("set-teams", func() {
		Context("when given a list of teams", func() {
			It("sets the given list of teams", func(){

			})
		})

		Context("when given a list of teams where some are incorrectly configured", func() {
			It("fails to set the incorrectly configured teams", func() {

			})
		})
	})
})
