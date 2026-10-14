package app

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
)

// Reaching a relay, or a served window, through a saved server: the
// stream goes out from that server, over the SSH connection to it, as a
// server saved behind another one is reached.

// throughRefused remembers a sign-in to a server gone through that the
// user turned down, for something that dials again by itself, as a
// relay does each time it loses its connection: asked once, and not
// again every few seconds. It is read and written on the program's
// goroutine.
type throughRefused struct {
	err error
	// noAsk connects the server gone through only when nothing has to
	// be asked, as ConnectTo.NoAsk.
	noAsk bool
}

// dialThrough dials from the saved server via, connecting to it first
// when it is not connected, for a stream that is wanted from any
// goroutine: a relay's, or one reaching a window. It asks the program's
// goroutine for the connection, and dials on its own. With refused, a
// sign-in turned down once is not asked for again.
func (a *app) dialThrough(via machines.ID, refused *throughRefused) func(ctx context.Context, network, addr string) (net.Conn, error) {
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
			if a.machines.IsWindow(via) {
				answer(dialled{err: fmt.Errorf("%s is a kakel window: a relay or a window is reached through a server", a.machines.Name(via))})
				return
			}
			if refused != nil && refused.err != nil {
				answer(dialled{err: refused.err})
				return
			}
			// Connected as any server is, with a sign-in asked for when
			// one is needed, and nothing opened on it.
			noAsk := refused != nil && refused.noAsk
			err := a.connectThen(ConnectTo{Server: via, Quiet: true, Only: true, NoAsk: noAsk}, func(err error) {
				if err != nil && refused != nil && (errors.Is(err, errDeclined) || errors.Is(err, context.Canceled) ||
					errors.Is(err, remote.ErrHostKeyRefused)) {
					refused.err = fmt.Errorf("signing in to %s was turned down; serve again to be asked again", a.machines.Name(via))
				}
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
