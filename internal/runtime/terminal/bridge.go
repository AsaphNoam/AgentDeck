package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/coder/websocket"
)

// PTYConn is the bidirectional PTY a WebSocket bridges to the browser (§3.4):
// keystrokes are written to it, its output is read back, and resize messages
// adjust the window size. The xterm driver's Tab exposes one of these.
type PTYConn interface {
	io.ReadWriteCloser
	Resize(rows, cols uint16) error
}

// wsConn is the minimal WebSocket surface the pumps need, so the bridge logic is
// unit-testable against a fake without a real handshake (§11 PTY-bridge tests).
type wsConn interface {
	// read returns one client frame: isText distinguishes a JSON control frame
	// (resize) from a binary keystroke frame.
	read(ctx context.Context) (isText bool, data []byte, err error)
	// writeBinary sends one PTY-output frame to the browser.
	writeBinary(ctx context.Context, data []byte) error
	close() error
}

// resizeMsg is the JSON control frame the browser sends on terminal resize
// ({cols, rows}) → pty.Setsize (§3.4).
type resizeMsg struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Bridge pumps bytes between a WebSocket and a PTY until either side closes:
// client binary frames → PTY master (keystrokes); client text frames → resize;
// PTY output → client binary frames (§3.4). Returns when both pumps have ended.
func Bridge(ctx context.Context, ws wsConn, p PTYConn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errc := make(chan error, 2)
	go func() { errc <- pumpClientToPTY(ctx, ws, p) }()
	go func() { errc <- pumpPTYToClient(ctx, ws, p) }()

	// First pump to finish tears the bridge down; the second then unblocks.
	err := <-errc
	cancel()
	_ = p.Close()
	_ = ws.close()
	<-errc
	return err
}

// pumpClientToPTY routes inbound client frames: text → resize, binary → keystrokes.
func pumpClientToPTY(ctx context.Context, ws wsConn, p PTYConn) error {
	for {
		isText, data, err := ws.read(ctx)
		if err != nil {
			return err
		}
		if isText {
			var rs resizeMsg
			if json.Unmarshal(data, &rs) == nil && (rs.Cols > 0 || rs.Rows > 0) {
				_ = p.Resize(rs.Rows, rs.Cols)
			}
			continue
		}
		if _, err := p.Write(data); err != nil {
			return err
		}
	}
}

// pumpPTYToClient streams PTY output to the browser as binary frames.
func pumpPTYToClient(ctx context.Context, ws wsConn, p PTYConn) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			frame := make([]byte, n)
			copy(frame, buf[:n])
			if werr := ws.writeBinary(ctx, frame); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// ServeWS bridges an accepted coder/websocket connection to a PTY (§3.4). The
// server accepts the upgrade (so it controls accept options) and hands the conn
// here; ServeWS owns the conn for the rest of its life and closes it on return.
func ServeWS(ctx context.Context, conn *websocket.Conn, p PTYConn) error {
	return Bridge(ctx, coderWS{c: conn}, p)
}

// coderWS adapts a *websocket.Conn (coder/websocket) to wsConn.
type coderWS struct{ c *websocket.Conn }

func (w coderWS) read(ctx context.Context) (bool, []byte, error) {
	typ, data, err := w.c.Read(ctx)
	return typ == websocket.MessageText, data, err
}

func (w coderWS) writeBinary(ctx context.Context, data []byte) error {
	return w.c.Write(ctx, websocket.MessageBinary, data)
}

func (w coderWS) close() error {
	return w.c.Close(websocket.StatusNormalClosure, "")
}
