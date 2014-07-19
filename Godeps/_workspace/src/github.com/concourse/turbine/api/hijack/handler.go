package hijack

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/scheduler"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	scheduler scheduler.Scheduler
}

type ProcessPayload struct {
	Stdin   []byte
	TTYSpec *warden.TTYSpec
}

func NewHandler(logger lager.Logger, scheduler scheduler.Scheduler) http.Handler {
	return &handler{
		logger: logger,

		scheduler: scheduler,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	guid := r.FormValue(":guid")

	log := handler.logger.Session("hijack", lager.Data{
		"guid": guid,
	})

	var spec warden.ProcessSpec
	err := json.NewDecoder(r.Body).Decode(&spec)
	if err != nil {
		log.Error("malformed-request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Info("start", lager.Data{
		"spec": spec,
	})

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

	log.Info("hijacked", lager.Data{
		"process": process.ID(),
	})

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

			if payload.TTYSpec != nil {
				process.SetTTY(*payload.TTYSpec)
			}
		}
	}()

	status, err := process.Wait()

	log.Info("completed", lager.Data{
		"process": process.ID(),
		"status":  status,
		"error":   fmt.Sprintf("%s", err),
	})
}
