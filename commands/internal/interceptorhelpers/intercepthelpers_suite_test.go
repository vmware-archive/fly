package interceptorhelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIntercepthelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Intercepthelpers Suite")
}
