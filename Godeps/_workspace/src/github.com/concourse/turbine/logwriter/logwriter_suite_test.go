package logwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogwriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logwriter Suite")
}
