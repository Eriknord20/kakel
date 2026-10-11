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
