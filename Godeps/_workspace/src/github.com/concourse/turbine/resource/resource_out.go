package resource

import (
	"io"

	"github.com/concourse/turbine/api/builds"
)

// Request payload from resource to /opt/resource/out script
type outRequest struct {
	Params builds.Params `json:"params"`
	Source builds.Source `json:"source"`
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
			Params: output.Params,
			Source: output.Source,
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
