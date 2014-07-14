package resource

import "github.com/concourse/turbine/api/builds"

type versionAndSource struct {
	Version builds.Version `json:"version"`
	Source  builds.Source  `json:"source"`
}

func (resource *resource) Check(input builds.Input) ([]builds.Version, error) {
	var versions []builds.Version

	err := resource.runScript(
		"/opt/resource/check",
		nil,
		versionAndSource{input.Version, input.Source},
		&versions,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
