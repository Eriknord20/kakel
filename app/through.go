package app

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/marrasen/kakel/machines"
)

// Reaching a relay, or a served window, through a saved server: the
// stream goes out from that server, over the SSH connection to it, as a
// server saved behind another one is reached.

// dialThrough dials from the saved server via, connecting to it first
// when it is not connected, for a stream that is wanted from any
// goroutine: a relay's, or one reaching a window. It asks the program's
// goroutine for the connection, and dials on its own.
func (a *app) dialThrough(via machines.ID) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type dialled struct {
			c   net.Conn
			err error
		}
		got := make(chan dialled, 1)
		// Only the first answer is waited for; another is dropped
		// rather than holding the program's goroutine.
		answer := func(d dialled) {
			select {
			case got <- d:
			default:
				if d.c != nil {
					_ = d.c.Close()
				}
			}
		}
		dial := func() {
			c, ok, err := a.connOf(via)
			if err == nil && !ok {
				err = fmt.Errorf("%s is not connected", a.machines.Name(via))
			}
			if err != nil {
				answer(dialled{err: err})
				return
			}
			go func() {
				nc, err := c.Dial(ctx, addr)
				answer(dialled{nc, err})
			}()
		}
		ask := func() {
			if _, ok, _ := a.connOf(via); ok {
				dial()
				return
			}
			// Connected as any server is, with a sign-in asked for when
			// one is needed, and nothing opened on it.
			err := a.connectThen(ConnectTo{Server: via, Quiet: true, Only: true}, func(err error) {
				if err != nil {
					answer(dialled{err: fmt.Errorf("connect to %s: %w", a.machines.Name(via), err)})
					return
				}
				dial()
			})
			if err != nil {
				answer(dialled{err: fmt.Errorf("connect to %s: %w", a.machines.Name(via), err)})
			}
		}
		select {
		case a.events <- ask:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-a.ctx.Done():
			return nil, errors.New("kakel is closing")
		}
		select {
		case d := <-got:
			return d.c, d.err
		case <-ctx.Done():
			// The stream may yet open; whoever opens it closes it.
			go func() {
				if d := <-got; d.c != nil {
					_ = d.c.Close()
				}
			}()
			return nil, ctx.Err()
		}
	}
}
