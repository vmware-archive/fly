package event

type Event interface {
	EventType() EventType
}

type EventType string

const (
	EventTypeInvalid    EventType = ""
	EventTypeLog        EventType = "log"
	EventTypeStatus     EventType = "status"
	EventTypeInitialize EventType = "initialize"
	EventTypeStart      EventType = "start"
	EventTypeError      EventType = "error"
)
