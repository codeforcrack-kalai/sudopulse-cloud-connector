package tunnel

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

// wsConn adapts a *websocket.Conn to implement io.ReadWriteCloser so it can
// be used as the underlying transport for yamux sessions. It translates between
// WebSocket message framing and the stream-oriented interface yamux expects.
type wsConn struct {
	conn   *websocket.Conn
	reader io.Reader
	mu     sync.Mutex // serialises writes; reads happen on a single goroutine
}

// newWSConn wraps a WebSocket connection as an io.ReadWriteCloser.
func newWSConn(conn *websocket.Conn) *wsConn {
	return &wsConn{conn: conn}
}

// Read implements io.Reader. It reads from the current WebSocket message,
// advancing to the next message when the current one is exhausted.
func (w *wsConn) Read(p []byte) (int, error) {
	for {
		if w.reader != nil {
			n, err := w.reader.Read(p)
			if err == io.EOF {
				// Current message consumed; move to next.
				w.reader = nil
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}

		// Block until the next message arrives.
		_, reader, err := w.conn.NextReader()
		if err != nil {
			return 0, err
		}
		w.reader = reader
	}
}

// Write implements io.Writer. Each call sends a single binary WebSocket message.
func (w *wsConn) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close implements io.Closer.
func (w *wsConn) Close() error {
	return w.conn.Close()
}
