package relay

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestAPipeCarriesStandardInputToTheWindow(t *testing.T) {
	_, u := started(t)
	code := NewCode()
	l, _ := hosted(t, u, code, NewToken())
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		_, _ = io.Copy(c, c)
		_ = c.Close()
	}()
	in, feed := io.Pipe()
	back, out := io.Pipe()
	link := Link{Relay: u, Code: code}.String()
	done := make(chan error, 1)
	go func() {
		done <- Pipe(context.Background(), []string{"-relay-pipe", "-proxy", "none", link}, in, out, io.Discard)
	}()
	go func() { _, _ = feed.Write([]byte("hello")) }()
	got := make([]byte, 5)
	if _, err := io.ReadFull(back, got); err != nil || string(got) != "hello" {
		t.Fatalf("came back %q, %v", got, err)
	}
	// Standard input ending, as ssh ending does, ends the pipe.
	_ = feed.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestAPipeWithNoLinkSaysHowToGiveOne(t *testing.T) {
	var says bytes.Buffer
	err := Pipe(context.Background(), []string{"-relay-pipe"}, strings.NewReader(""), io.Discard, &says)
	if err == nil || !strings.Contains(err.Error(), "#XXXX-XXXX") {
		t.Fatalf("got %v", err)
	}
}
