package eventstream

import (
	"github.com/concourse/go-concourse/concourse/eventstream"
	colorable "github.com/mattn/go-colorable"
	"github.com/vito/go-sse/sse"
)

func RenderStream(eventSource *sse.EventSource) (int, error) {
	return Render(colorable.NewColorableStdout(), eventstream.NewSSEEventStream(eventSource)), nil
}
