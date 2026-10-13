package relay

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// pingEvery is how long a stream may go quiet before the relay or a
// window asks the other end whether it is still there. It also keeps a
// proxy from closing a connection that has gone quiet: most close one
// idle for a minute or more.
var pingEvery = 25 * time.Second

// pingWait is how long the answer to a ping may take. Long, because a
// ping waits behind whatever is queued on the way: a large copy over a
// slow link can hold megabytes ahead of it.
var pingWait = 2 * time.Minute

// seen is when a stream last moved, in either direction, as Unix
// nanoseconds. A stream moving is a stream whose other end is there, so
// it needs no ping.
type seen struct{ at atomic.Int64 }

func (s *seen) now()                          { s.at.Store(time.Now().UnixNano()) }
func (s *seen) since() time.Duration          { return time.Since(time.Unix(0, s.at.Load())) }
func newSeen() *seen                          { s := &seen{}; s.now(); return s }
func (s *seen) quietFor(d time.Duration) bool { return s.since() >= d }

// keepUp pings ws whenever it has been quiet for pingEvery, until ctx
// ends, and closes it when a ping goes unanswered: the other end has
// gone without saying so. moved is when the stream last moved, nil for
// a stream that carries nothing but pings.
func keepUp(ctx context.Context, ws *websocket.Conn, moved *seen) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if moved != nil && !moved.quietFor(pingEvery) {
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, pingWait)
		err := ws.Ping(pctx)
		cancel()
		if err != nil && ctx.Err() == nil {
			_ = ws.CloseNow()
			return
		}
	}
}

// movingConn notes each read and write that completes on a stream.
type movingConn struct {
	net.Conn
	moved *seen
}

func (c movingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.moved.now()
	}
	return n, err
}

func (c movingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.moved.now()
	}
	return n, err
}

// carried is ws as a stream of bytes, pinged while quiet until ctx ends.
func carried(ctx context.Context, ws *websocket.Conn) net.Conn {
	moved := newSeen()
	go keepUp(ctx, ws, moved)
	return movingConn{Conn: websocket.NetConn(ctx, ws, websocket.MessageBinary), moved: moved}
}
