package resource

import (
	"io"

	"github.com/concourse/turbine/api/builds"
)

// Request payload from resource to /opt/resource/out script
type outRequest struct {
	Source  builds.Source  `json:"source"`
	Params  builds.Params  `json:"params,omitempty"`
	Version builds.Version `json:"version,omitempty"`
}

// Response payload from /opt/resource/out script to resource
type outResponse struct {
	Version  builds.Version         `json:"version"`
	Metadata []builds.MetadataField `json:"metadata"`
}

func (resource *resource) Out(sourceStream io.Reader, output builds.Output) (builds.Output, error) {
	err := resource.container.StreamIn("/tmp/build/src", sourceStream)
	if err != nil {
		return builds.Output{}, err
	}

	var resp outResponse

	err = resource.runScript(
		"/opt/resource/out",
		[]string{"/tmp/build/src"},
		outRequest{
			Params:  output.Params,
			Source:  output.Source,
			Version: output.Version,
		},
		&resp,
	)
	if err != nil {
		return builds.Output{}, err
	}

	output.Version = resp.Version
	output.Metadata = resp.Metadata

	return output, nil
}
