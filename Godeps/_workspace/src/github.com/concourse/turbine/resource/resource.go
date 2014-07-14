package resource

import (
	"io"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/api/builds"
)

type Resource interface {
	In(builds.Input) (io.Reader, builds.Input, builds.Config, error)
	Out(io.Reader, builds.Output) (builds.Output, error)
	Check(builds.Input) ([]builds.Version, error)
}

type resource struct {
	container warden.Container
	logs      io.Writer
	abort     <-chan struct{}
}

func NewResource(
	container warden.Container,
	logs io.Writer,
	abort <-chan struct{},
) Resource {
	return &resource{
		container: container,
		logs:      logs,
		abort:     abort,
	}
}
