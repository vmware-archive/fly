package compressor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCompressor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compressor Suite")
}
