package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/words"

	"github.com/marrasen/kakel/jobs"
	"github.com/marrasen/kakel/vfs"

	"github.com/marrasen/gunim/filemanager"
)

// Copying, moving and deleting files, with kakel's jobs package, on the
// program's side.

// Intents for the jobs.
type (
	// ShowJobs opens the jobs pane, or goes to it.
	ShowJobs struct{}
	// CancelJob stops a job part way.
	CancelJob struct{ ID string }
	// ClearJobs takes the finished jobs off the jobs pane.
	ClearJobs struct{}
	// DropJob takes one finished job off the jobs pane.
	DropJob struct{ ID string }
)

// Job is one piece of file work, as the jobs pane shows it.
type Job struct {
	ID    string
	Title string
	// Machine is where it works, as its row in the sidebar is filed,
	// and Kind is copy, move or delete.
	Machine machines.ID
	Kind    string
	// From is where the names are, and To where they go. A delete has
	// no To.
	From, To JobEnd
	// Detail says how far it has got, or how it ended.
	Detail string
	// Share is how much is done, from 0 to 1, and below zero while the
	// job is still working out how much there is.
	Share float32
	// Done is set once it has stopped, Stopped when the user stopped it,
	// and Failed when it was for any other reason but finishing.
	Done, Stopped, Failed bool
	// Names are what it works on, Current the one it is on, and Ticked
	// how many of Names are done, when they are counted one by one.
	Names   []string
	Current string
	Ticked  int
	// Speeds are its speed over the last while, oldest first, for a
	// graph, and Sampled how many samples have been taken in all, so a
	// graph can tell the new ones.
	Speeds  []uint64
	Sampled int
	// Repeatable says a finished copy can be run again, and Saved that
	// it is on the saved list.
	Repeatable, Saved bool
	// Started is when it started, and Finished when it stopped, zero
	// while it runs.
	Started, Finished time.Time
}

// JobEnd is a folder a job works in.
type JobEnd struct {
	// Folder is the folder's own name, Path all of it as the machine
	// types it, and Machine what the machine is called. Local says the
	// machine is this computer.
	Folder, Path, Machine string
	Local                 bool
}

// KindJobs is the jobs pane.
const KindJobs = "jobs"

// mostFinishedJobs is how many finished jobs the pane keeps.
const mostFinishedJobs = 20

// jobKind names a kind of job for its row's icon.
func jobKind(k jobs.Kind) string {
	switch k {
	case jobs.Move:
		return "move"
	case jobs.Delete:
		return "delete"
	}
	return "copy"
}

// running is a job the program follows.
type running struct {
	id    string
	job   *jobs.Job
	title string
	// op is the work, and from and to the machines it is between, to
	// do it again; speeds are its speed, sampled as it is looked at,
	// from lastBytes at lastAt.
	op       jobs.Op
	from, to machines.ID
	// repeating says a repeat has been asked for and has not started
	// yet: a machine opened again takes as long as a connection does,
	// and a second press in that time would copy the same thing twice.
	repeating bool
	speeds    []uint64
	sampled   int
	lastBytes int64
	lastAt    time.Time
	// ended is set once its end has been reported; started is when it
	// started, and finished when it was first seen stopped.
	ended             bool
	started, finished time.Time
	// quiet says a file manager window shows how the job goes, and how
	// it ends: kakel's windows list it, and say nothing of its own.
	quiet bool
}

// follow starts a job and follows it.
func (a *app) follow(op jobs.Op, title string) { a.followOn(op, title, "", "") }

// followOn is follow, for a job between the machines from and to, which
// a repeat opens again.
func (a *app) followOn(op jobs.Op, title string, from, to machines.ID) *jobs.Job {
	return a.followAsking(op, title, from, to, &overwriteAsker{a: a}, false)
}

// followAsking is followOn, asking ask about names that are taken, and
// quiet when a file manager window shows how the job goes.
func (a *app) followAsking(op jobs.Op, title string, from, to machines.ID, ask jobs.Ask, quiet bool) *jobs.Job {
	if a.jobs == nil {
		a.jobs = jobs.New(2)
	}
	job := a.jobs.Start(a.ctx, op, jobs.Options{Ask: ask})
	a.clearJobs(false)
	a.jobSeq++
	a.running = append(a.running, &running{id: "j" + itoa(a.jobSeq), job: job, title: title, op: op, from: from, to: to, quiet: quiet,
		started: time.Now()})
	if !a.watching {
		a.watching = true
		go a.watchJobs()
	}
	a.showJobs()
	return job
}

// watchJobs looks at the jobs four times a second while any runs.
func (a *app) watchJobs() {
	t := time.NewTicker(SampleEvery)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		}
		done := make(chan bool, 1)
		a.events <- func() { done <- a.showJobs() }
		if !<-done {
			return
		}
	}
}

// showJobs brings the jobs pane and the status line up to date, and
// reports the jobs that ended. It reports whether any still runs.
func (a *app) showJobs() bool {
	var lines []string
	var rows []Job
	live := false
	for _, r := range a.running {
		p := r.job.Progress()
		r.sample(p)
		if p.Done && r.finished.IsZero() {
			r.finished = time.Now()
		}
		row := jobRow(r, p)
		row.Saved = a.isSaved(r)
		row.From = a.jobEnd(r.op.At, r.from)
		if r.op.Kind != jobs.Delete {
			row.To = a.jobEnd(r.op.Into, r.to)
		}
		rows = append(rows, row)
		if !p.Done {
			live = true
			lines = append(lines, r.title+"…")
			if row.Share >= 0 {
				lines[len(lines)-1] = fmt.Sprintf("%s, %d%%", r.title, int(100*row.Share))
			}
			continue
		}
		if r.ended {
			continue
		}
		r.ended = true
		switch {
		case r.quiet:
		case jobs.Trouble(p.Err) != nil:
			a.failed(r.title+" stopped", jobs.Outcome(p))
			a.problem()
		case p.Err == nil:
			a.worked(pastTense(r.title), row.Detail, "")
			a.done()
		}
	}
	// The newest on top.
	slices.Reverse(rows)
	a.st.Jobs = rows
	a.jobLines = lines
	a.showStatus()
	if !live {
		a.watching = false
	}
	return live
}

// jobEnd is the folder dir on the machine on, for the jobs pane.
func (a *app) jobEnd(dir string, on machines.ID) JobEnd {
	end := JobEnd{Path: typedOn(dir), Machine: "this computer", Local: on == machines.Local}
	if a.machines != nil {
		end.Machine = a.machines.Name(on)
	}
	end.Folder = folderName(end.Path)
	return end
}

// folderName is the last name in path, and the path itself for a root
// such as / or C:\. A Windows path, C:\… or \\server\…, is cut at
// either slash; any other at / alone, as a name there may hold a \.
func folderName(path string) string {
	seps := "/"
	if len(path) >= 2 && path[1] == ':' || strings.HasPrefix(path, `\\`) {
		seps = `/\`
	}
	trimmed := strings.TrimRight(path, seps)
	if trimmed == "" || strings.HasSuffix(trimmed, ":") {
		return path
	}
	return trimmed[strings.LastIndexAny(trimmed, seps)+1:]
}

// jobRow is a job as the pane shows it.
func jobRow(r *running, p jobs.Progress) Job {
	row := Job{ID: r.id, Title: r.title, Share: -1, Done: p.Done, Names: r.op.Names, Current: p.Current, Started: r.started, Finished: r.finished,
		Speeds: slices.Clone(r.speeds), Sampled: r.sampled, Repeatable: p.Done && r.op.Kind == jobs.Copy, Machine: r.to, Kind: jobKind(r.op.Kind)}
	// Filed under the machine it writes to, unless that is this one: a
	// copy down from a server stays under the server, where the files
	// it reads are. A delete is filed where it happens.
	if r.op.Kind == jobs.Delete || r.to == machines.Local {
		row.Machine = r.from
	}
	if p.Files == len(r.op.Names) {
		row.Ticked = p.FilesDone
	}
	switch {
	case p.Bytes > 0:
		row.Share = float32(p.BytesDone) / float32(p.Bytes)
	case p.Files > 0:
		row.Share = float32(p.FilesDone) / float32(p.Files)
	}
	var said []string
	switch {
	case p.Done && p.Err != nil:
		// A job the user stopped says so, and how far it got; one that
		// failed, or left half a file behind, says why.
		row.Failed = jobs.Trouble(p.Err) != nil
		row.Stopped = !row.Failed
		said = append(said, jobs.Outcome(p))
		if p.FilesDone > 0 {
			said = append(said, words.Count(p.FilesDone, "file")+" done")
		}
	case p.Done:
		row.Share = 1
		row.Title = pastTense(r.title)
		done := words.Count(p.FilesDone, "file") + " done"
		if p.Skipped > 0 {
			done += fmt.Sprintf(", %d left as they were", p.Skipped)
		}
		said = append(said, done)
		took := p.Ended.Sub(p.Started)
		if took >= time.Second {
			said = append(said, "in "+took.Round(time.Second).String())
		}
		if took > 0 && p.BytesDone > 0 {
			said = append(said, words.Size(int64(float64(p.BytesDone)/took.Seconds()))+"/s on average")
		}
	case p.Files == 0:
		said = append(said, "Counting…")
	default:
		said = append(said, fmt.Sprintf("%d of %s", p.FilesDone, words.Count(p.Files, "file")))
		if !p.Started.IsZero() {
			said = append(said, jobs.Going(time.Since(p.Started)))
		}
		if p.Bytes > 0 {
			said = append(said, words.Size(p.BytesDone)+" of "+words.Size(p.Bytes))
		}
		if speed := r.speedNow(); speed > 0 {
			said = append(said, words.Size(int64(speed))+"/s")
			if p.Bytes > p.BytesDone {
				left := time.Duration(float64(p.Bytes-p.BytesDone) / float64(speed) * float64(time.Second))
				if left < time.Second {
					said = append(said, "about a second left")
				} else {
					said = append(said, "about "+left.Round(time.Second).String()+" left")
				}
			}
		}
	}
	row.Detail = strings.Join(said, " · ")
	return row
}

// pastTense turns a job's title into what it did.
func pastTense(title string) string {
	for _, w := range [][2]string{{"Copying", "Copied"}, {"Moving", "Moved"}, {"Deleting", "Deleted"}} {
		if after, ok := strings.CutPrefix(title, w[0]); ok {
			return w[1] + after
		}
	}
	return title
}

// cancelJob stops a job part way.
func (a *app) cancelJob(id string) {
	for _, r := range a.running {
		if r.id == id {
			r.job.Cancel()
		}
	}
}

// clearJobs takes the finished jobs away, and keeps the newest few
// finished when there are too many to show.
func (a *app) clearJobs(all bool) {
	var keep []*running
	finished := 0
	for i := len(a.running) - 1; i >= 0; i-- {
		r := a.running[i]
		if r.ended {
			finished++
			if all || finished > mostFinishedJobs {
				continue
			}
		}
		keep = append(keep, r)
	}
	slices.Reverse(keep)
	a.running = keep
}

// showJobsPane opens the jobs pane, or goes to it.
func (a *app) showJobsPane() {
	for _, p := range a.st.Panes {
		if p.Kind == KindJobs {
			a.bringHere(p.ID)
			return
		}
	}
	a.next++
	a.addPane(Pane{ID: "p" + itoa(a.next), Title: "Jobs", Kind: KindJobs}, nil, Placement{})
}

// overwriteAsker asks the user about a name that is already there, as
// the file manager's own dialog does, in its words: Replace “name”?,
// what each one is, Apply to all, and Skip, Keep Both, Stop or Replace.
// Keep Both for all is kept here, as each item takes a name of its own.
type overwriteAsker struct {
	a        *app
	mu       sync.Mutex
	keepBoth bool
}

// Overwrite implements [jobs.Ask].
func (q *overwriteAsker) Overwrite(ctx context.Context, c jobs.Conflict) (jobs.Choice, error) {
	q.mu.Lock()
	keepBoth := q.keepBoth
	q.mu.Unlock()
	if keepBoth {
		return jobs.Choice{What: jobs.Rename, Name: vfs.Base(c.To, nameBeside(c.To, c.Path, c.Want.IsDir()))}, nil
	}
	name, where := vfs.Base(c.To, c.Path), vfs.Base(c.To, vfs.Dir(c.To, c.Path))
	if where == "" {
		where = vfs.Dir(c.To, c.Path)
	}
	kind := "a file"
	if c.Have.IsDir() {
		kind = "a folder"
	}
	sameKind := c.Have.IsDir() == c.Want.IsDir()
	title := "Replace “" + name + "”?"
	choose := []string{"Replace", "Skip", "Keep Both"}
	if !sameKind {
		title, choose = "“"+name+"” is already there", []string{"Keep Both", "Skip"}
	}
	ans, err := q.a.ask(ctx, Ask{Title: title, Text: where + " already has " + kind + " named “" + name + "”.",
		Small: []string{"Existing: " + filemanager.ItemFacts(c.Have.Size, c.Have.IsDir(), c.Have.Mod),
			"New: " + filemanager.ItemFacts(c.Want.Size, c.Want.IsDir(), c.Want.Mod)},
		Choose: choose, Also: "Apply to all", No: "Stop", Careful: true})
	switch {
	case errors.Is(err, errDeclined):
		// Stop is an answer, and the job ends saying it was stopped.
		return jobs.Choice{What: jobs.Stop}, nil
	case err != nil:
		return jobs.Choice{What: jobs.Stop}, err
	}
	all := len(ans.Answers) > 1 && ans.Answers[1] == "yes"
	switch {
	case len(ans.Answers) > 0 && ans.Answers[0] == "Skip":
		return jobs.Choice{What: jobs.Skip, All: all}, nil
	case len(ans.Answers) > 0 && ans.Answers[0] == "Keep Both":
		if all {
			q.mu.Lock()
			q.keepBoth = true
			q.mu.Unlock()
		}
		return jobs.Choice{What: jobs.Rename, Name: vfs.Base(c.To, nameBeside(c.To, c.Path, c.Want.IsDir()))}, nil
	}
	return jobs.Choice{What: jobs.Replace, All: all}, nil
}

// mostSpeeds is how many speed samples a job keeps for its graph, and
// sampleEvery how often one is taken: twelve seconds, ten a second.
const (
	MostSpeeds  = 120
	SampleEvery = 100 * time.Millisecond
)

// sample writes down how fast the job went since it was last looked at.
func (r *running) sample(p jobs.Progress) {
	now := time.Now()
	if !r.lastAt.IsZero() && !p.Done {
		if dt := now.Sub(r.lastAt).Seconds(); dt > 0 {
			r.speeds = append(r.speeds, uint64(max(0, float64(p.BytesDone-r.lastBytes)/dt)))
			r.sampled++
			if len(r.speeds) > MostSpeeds {
				r.speeds = r.speeds[len(r.speeds)-MostSpeeds:]
			}
		}
	}
	r.lastBytes, r.lastAt = p.BytesDone, now
}

// speedNow is the job's speed over its last second.
func (r *running) speedNow() uint64 {
	n := min(int(time.Second/SampleEvery), len(r.speeds))
	if n == 0 {
		return 0
	}
	var sum uint64
	for _, s := range r.speeds[len(r.speeds)-n:] {
		sum += s
	}
	return sum / uint64(n)
}
