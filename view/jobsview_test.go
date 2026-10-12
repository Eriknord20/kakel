package view

import (
	"testing"
	"time"

	"github.com/marrasen/kakel/app"
)

// A job says when it ran: when it started while it runs, and when it
// started and how long it took once it has stopped, with the day where
// that was not today.
func TestAJobSaysWhenItRan(t *testing.T) {
	now := time.Date(2026, 10, 11, 17, 30, 0, 0, time.Local)
	at := func(d, h, m, s int) time.Time { return time.Date(2026, 10, d, h, m, s, 0, time.Local) }
	for _, c := range []struct {
		j    app.Job
		want string
	}{
		{app.Job{}, ""},
		{app.Job{Started: at(11, 17, 2, 0)}, "started 17:02"},
		{app.Job{Started: at(11, 17, 2, 0), Finished: at(11, 17, 2, 4)}, "17:02, took 4 s"},
		{app.Job{Started: at(11, 9, 5, 0), Finished: at(11, 9, 8, 30)}, "09:05, took 3 min 30 s"},
		{app.Job{Started: at(10, 23, 50, 0), Finished: at(11, 1, 20, 0)}, "yesterday 23:50, took 1 h 30 min"},
		{app.Job{Started: at(3, 8, 0, 0), Finished: at(3, 8, 0, 0)}, "3 Oct 08:00, took under a second"},
	} {
		if got := jobWhen(c.j, now); got != c.want {
			t.Errorf("a job %v to %v says %q, want %q", c.j.Started, c.j.Finished, got, c.want)
		}
	}
}

// A card's heading says what the job does, or did, to what, and how
// it ended where that was not by finishing.
func TestAJobCardSaysWhatItDoes(t *testing.T) {
	two := []string{"a.txt", "b.txt"}
	for _, c := range []struct {
		j    app.Job
		want string
	}{
		{app.Job{Kind: "copy", Names: two}, "Copying 2 items"},
		{app.Job{Kind: "move", Names: []string{"report.pdf"}, Done: true}, "Moved report.pdf"},
		{app.Job{Kind: "delete", Names: two, Done: true, Stopped: true}, "Stopped deleting 2 items"},
		{app.Job{Kind: "copy", Names: two, Done: true, Failed: true}, "Couldn’t copy 2 items"},
	} {
		if got := jobHeading(c.j); got != c.want {
			t.Errorf("%+v is headed %q, want %q", c.j, got, c.want)
		}
	}
}
