package resource

import (
	"archive/tar"
	"io"
	"path"

	"github.com/concourse/turbine/api/builds"
	"github.com/fraenkel/candiedyaml"
)

// Request payload from sourcefetcher to /opt/resource/in script
type inRequest struct {
	Version builds.Version `json:"version,omitempty"`
	Source  builds.Source  `json:"source"`
	Params  builds.Params  `json:"params,omitempty"`
}

// Response payload from /opt/resource/in script to sourcefetcher
type inResponse struct {
	// Version is returned because request payload
	// may not contain Version to fetch relying on
	// 'in' script to fetch latest version.
	Version builds.Version `json:"version"`

	Metadata []builds.MetadataField `json:"metadata,omitempty"`
}

func (resource *resource) In(input builds.Input) (io.Reader, builds.Input, builds.Config, error) {
	var resp inResponse

	err := resource.runScript(
		"/opt/resource/in",
		[]string{"/tmp/build/src/" + input.Name},
		inRequest{input.Version, input.Source, input.Params},
		&resp,
	)
	if err != nil {
		return nil, builds.Input{}, builds.Config{}, err
	}

	buildConfig, err := resource.extractConfig(input)
	if err != nil {
		return nil, builds.Input{}, builds.Config{}, err
	}

	input.Version = resp.Version
	input.Metadata = resp.Metadata

	outStream, err := resource.container.StreamOut(path.Join("/tmp/build/src", input.Name) + "/")
	if err != nil {
		return nil, builds.Input{}, builds.Config{}, err
	}

	return outStream, input, buildConfig, nil
}

func (resource *resource) extractConfig(input builds.Input) (builds.Config, error) {
	if input.ConfigPath == "" {
		return builds.Config{}, nil
	}

	configPath := path.Join("/tmp/build/src", input.Name, input.ConfigPath)

	configStream, err := resource.container.StreamOut(configPath)
	if err != nil {
		return builds.Config{}, err
	}

	reader := tar.NewReader(configStream)

	_, err = reader.Next()
	if err != nil {
		return builds.Config{}, err
	}

	var buildConfig builds.Config

	err = candiedyaml.NewDecoder(reader).Decode(&buildConfig)
	if err != nil {
		return builds.Config{}, err
	}

	return buildConfig, nil
}
