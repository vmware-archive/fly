package eventstream

import (
	"github.com/concourse/go-concourse/concourse/eventstream"
	"github.com/fatih/color"
	"github.com/vito/go-sse/sse"
)

func RenderStream(eventSource *sse.EventSource) (int, error) {
	return Render(color.Output, eventstream.NewSSEEventStream(eventSource)), nil
}
