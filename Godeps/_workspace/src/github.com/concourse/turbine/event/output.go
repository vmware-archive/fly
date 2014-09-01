package event

import "github.com/concourse/turbine/api/builds"

type Output struct {
	Output builds.Output `json:"input"`
}

func (Output) EventType() EventType { return EventTypeOutput }
