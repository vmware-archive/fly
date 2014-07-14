package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/garden/warden"
)

var ErrAborted = errors.New("script aborted")

type ErrResourceScriptFailed struct {
	Path       string
	Args       []string
	Stdout     string
	Stderr     string
	ExitStatus int
}

func (err ErrResourceScriptFailed) Error() string {
	return fmt.Sprintf(
		"resource script '%s %v' failed: exit status %d\n\nstdout:\n\n%s\n\nstderr:\n\n%s",
		err.Path,
		err.Args,
		err.ExitStatus,
		err.Stdout,
		err.Stderr,
	)
}

func (resource *resource) runScript(path string, args []string, input interface{}, output interface{}) error {
	request, err := json.Marshal(input)
	if err != nil {
		return err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	process, err := resource.container.Run(warden.ProcessSpec{
		Path:       path,
		Args:       args,
		Privileged: true,
	}, warden.ProcessIO{
		Stdin:  bytes.NewBuffer(request),
		Stderr: io.MultiWriter(resource.logs, stderr),
		Stdout: stdout,
	})
	if err != nil {
		return err
	}

	statusCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		status, err := process.Wait()
		if err != nil {
			errCh <- err
		} else {
			statusCh <- status
		}
	}()

	select {
	case status := <-statusCh:
		if status != 0 {
			return ErrResourceScriptFailed{
				Path:       path,
				Args:       args,
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitStatus: status,
			}
		}

		return json.Unmarshal(stdout.Bytes(), output)

	case err := <-errCh:
		return err

	case <-resource.abort:
		resource.container.Stop(false)
		return ErrAborted
	}
}
