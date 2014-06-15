package builds

import "time"

type Build struct {
	Guid      string              `json:"guid,omitempty"`
	CreatedAt time.Time           `json:"created_at,omitempty"`
	Image     string              `json:"image"`
	Path      string              `json:"path,omitempty"`
	Script    string              `json:"script"`
	Env       []map[string]string `json:"env"`
	Status    string              `json:"status,omitempty"`
}

type BuildResult struct {
	Status string `json:"status"`
}
