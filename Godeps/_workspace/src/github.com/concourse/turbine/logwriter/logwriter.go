package logwriter

import (
	"io"
	"unicode/utf8"

	"github.com/concourse/turbine/event"
)

type writer struct {
	emitter event.Emitter
	origin  event.Origin

	dangling []byte
}

func NewWriter(emitter event.Emitter, origin event.Origin) io.Writer {
	return &writer{
		emitter: emitter,
		origin:  origin,
	}
}

func (writer *writer) Write(data []byte) (int, error) {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return len(data), nil
	}

	writer.dangling = nil

	writer.emitter.EmitEvent(event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}
