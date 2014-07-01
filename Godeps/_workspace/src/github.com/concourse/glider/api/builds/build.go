package builds

import (
	"time"

	"github.com/concourse/turbine/api/builds"
)

type Build struct {
	Guid      string        `json:"guid,omitempty"`
	CreatedAt time.Time     `json:"created_at,omitempty"`
	Config    builds.Config `json:"config"`
	Path      string        `json:"path,omitempty"`
	Status    string        `json:"status,omitempty"`
}

type BuildResult struct {
	Status string `json:"status"`
}
