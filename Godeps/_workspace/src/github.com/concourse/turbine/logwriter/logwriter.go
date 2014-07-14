package logwriter

import (
	"io"
	"sync"
	"time"

	"code.google.com/p/go.net/websocket"
)

type writer struct {
	url string

	conn  *websocket.Conn
	connL *sync.Mutex
}

func NewWriter(url string) io.WriteCloser {
	return &writer{
		url: url,

		connL: new(sync.Mutex),
	}
}

func (writer *writer) Write(data []byte) (int, error) {
	for {
		writer.connect()

		n, err := writer.conn.Write(data)
		if err == nil {
			return n, err
		}

		writer.close()

		time.Sleep(time.Second)
	}
}

func (writer *writer) Close() error {
	return writer.close()
}

func (writer *writer) connect() {
	writer.connL.Lock()
	defer writer.connL.Unlock()

	if writer.conn != nil {
		return
	}

	var err error

	for {
		writer.conn, err = websocket.Dial(writer.url, "", "http://0.0.0.0")
		if err == nil {
			writer.conn.PayloadType = websocket.BinaryFrame
			return
		}

		time.Sleep(time.Second)
	}
}

func (writer *writer) close() error {
	writer.connL.Lock()
	defer writer.connL.Unlock()

	if writer.conn != nil {
		conn := writer.conn
		writer.conn = nil
		return conn.Close()
	}

	return nil
}
