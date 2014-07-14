package hijack

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/scheduler"
)

type handler struct {
	scheduler scheduler.Scheduler
}

type ProcessPayload struct {
	Stdin      []byte
	WindowSize *WindowSize
}

type WindowSize struct {
	Columns int
	Rows    int
}

func NewHandler(scheduler scheduler.Scheduler) http.Handler {
	return &handler{
		scheduler: scheduler,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	guid := r.FormValue(":guid")

	var spec warden.ProcessSpec
	err := json.NewDecoder(r.Body).Decode(&spec)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}

	defer conn.Close()

	inR, inW := io.Pipe()

	process, err := handler.scheduler.Hijack(guid, spec, warden.ProcessIO{
		Stdin:  inR,
		Stdout: conn,
		Stderr: conn,
	})
	if err != nil {
		fmt.Fprintf(conn, "error: %s\n", err)
		return
	}

	decoder := gob.NewDecoder(br)

	go func() {
		defer inW.Close()

		for {
			var payload ProcessPayload
			err := decoder.Decode(&payload)
			if err != nil {
				break
			}

			if payload.Stdin != nil {
				_, err := inW.Write(payload.Stdin)
				if err != nil {
					break
				}
			}

			if payload.WindowSize != nil {
				size := *payload.WindowSize
				process.SetWindowSize(size.Columns, size.Rows)
			}
		}
	}()

	process.Wait()
}
