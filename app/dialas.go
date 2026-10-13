package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
)

// Plain ssh tunnels through a served window. ssh -L and ssh -J ask for
// a stream to an address; the user name they logged in with says where
// it goes out from: kakel for this machine, or a saved machine by name,
// as the commands spell it, "db-server" for "DB Server".

// ThisMachineUser is the user name that has plain ssh's tunnels go out
// from the machine being served.
const ThisMachineUser = "kakel"

// dialAs opens a stream to addr for a plain ssh client that logged in
// as user. A saved server not connected is connected to, when that
// needs nothing asked; a question would wait on a screen nobody may be
// at, so the stream is refused instead.
func (a *app) dialAs(ctx context.Context, user, addr string) (net.Conn, error) {
	if strings.EqualFold(user, ThisMachineUser) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	type route struct {
		conn *remote.Conn
		win  func(ctx context.Context, addr string) (net.Conn, error)
		err  error
	}
	got := make(chan route, 1)
	answer := func(r route) {
		select {
		case got <- r:
		default:
		}
	}
	find := func() {
		id, ok := a.machineCalled(user)
		if !ok {
			answer(route{err: fmt.Errorf("there is no saved machine called %s here; log in as %s@ for tunnels from this machine itself", user, ThisMachineUser)})
			return
		}
		called := a.machines.Name(id)
		if w := a.machines.Get(id).Window; w != nil {
			answer(route{win: func(ctx context.Context, addr string) (net.Conn, error) { return w.Serve.DialOn(ctx, "", addr) }})
			return
		}
		if a.machines.IsWindow(id) {
			answer(route{err: fmt.Errorf("%s is a kakel window not connected here; connect it here first", called)})
			return
		}
		take := func(err error) {
			if err != nil {
				answer(route{err: fmt.Errorf("connect to %s: %w", called, err)})
				return
			}
			c, ok, err := a.connOf(id)
			if err == nil && !ok {
				err = fmt.Errorf("%s is not connected", called)
			}
			answer(route{conn: c, err: err})
		}
		if _, ok, _ := a.connOf(id); ok {
			take(nil)
			return
		}
		log.Printf("connecting to %s for a tunnel through this window", called)
		if err := a.connectThen(ConnectTo{Server: id, Quiet: true, Only: true, NoAsk: true}, take); err != nil {
			take(err)
		}
	}
	select {
	case a.events <- find:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-a.ctx.Done():
		return nil, a.ctx.Err()
	}
	var r route
	select {
	case r = <-got:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	switch {
	case r.err != nil:
		return nil, r.err
	case r.win != nil:
		return r.win(ctx, addr)
	}
	return r.conn.Dial(ctx, addr)
}

// machineCalled is the saved machine named name, as the commands spell
// it, or by its name as it is.
func (a *app) machineCalled(name string) (machines.ID, bool) {
	if a.book == nil {
		return "", false
	}
	want := remote.CommandName(name)
	for _, h := range a.book.Hosts() {
		if remote.CommandName(h.Name) == want || strings.EqualFold(h.Name, name) {
			return machines.ID(h.ID), true
		}
	}
	return "", false
}
