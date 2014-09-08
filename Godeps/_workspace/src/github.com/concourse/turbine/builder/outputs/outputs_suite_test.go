package outputs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOutputs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Outputs Suite")
}
