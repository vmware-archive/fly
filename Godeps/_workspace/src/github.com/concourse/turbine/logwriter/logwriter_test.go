package logwriter_test

import (
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/turbine/event"
	efakes "github.com/concourse/turbine/event/fakes"
	. "github.com/concourse/turbine/logwriter"
)

const nihongo = "日本語"

var _ = Describe("Writer", func() {
	var (
		emitter *efakes.FakeEmitter
		origin  event.Origin

		writer io.Writer
	)

	BeforeEach(func() {
		emitter = new(efakes.FakeEmitter)

		origin = event.Origin{
			Type: event.OriginTypeRun,
			Name: "some-source",
		}

		writer = NewWriter(emitter, origin)
	})

	It("does not transmit utf8 codepoints that are split in twain", func() {
		writer.Write([]byte("hello"))
		Ω(emitter.EmitEventCallCount()).Should(Equal(1))
		Ω(emitter.EmitEventArgsForCall(0)).Should(Equal(event.Log{
			Payload: "hello",
			Origin:  origin,
		}))

		writer.Write([]byte(nihongo[:7]))
		Ω(emitter.EmitEventCallCount()).Should(Equal(1))

		writer.Write([]byte(nihongo[7:]))
		Ω(emitter.EmitEventCallCount()).Should(Equal(2))
		Ω(emitter.EmitEventArgsForCall(1)).Should(Equal(event.Log{
			Payload: nihongo,
			Origin:  origin,
		}))
	})
})
