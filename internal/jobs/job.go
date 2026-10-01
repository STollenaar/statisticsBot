// Package jobs runs the bot's long-lived background work: the embedding
// backfill and the message sync. Both take far longer than any sane HTTP
// timeout, so each outlives the request that starts it, and both the HTTP routes
// and the admin command drive the same shared state rather than each holding
// their own — two entry points cannot start competing runs of the same job.
//
// Job below is the state machine they share. Each concrete job supplies only its
// own work loop and the labels it reports progress under.
package jobs

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Phase string

const (
	PhaseIdle Phase = "idle"
	// PhasePreparing covers the setup a job does before it can report a total:
	// listing the messages to embed, loading the ids already stored.
	PhasePreparing Phase = "preparing"
	PhaseRunning   Phase = "running"
	PhaseDone      Phase = "done"
	PhaseFailed    Phase = "failed"
)

// Job is the state of one background job. Every field is guarded by mu, and the
// only way to read them is status(), which builds a consistent snapshot.
type Job struct {
	mu sync.Mutex

	// name and unit are set at construction and never change: name goes in log
	// lines, unit is what the rate is measured in ("msg", "channels").
	name string
	unit string

	phase Phase
	// step is the job's own word for what it is doing right now ("embedding",
	// "scanning"), shown alongside the generic phase.
	step string

	started time.Time
	// workStart is when the real work began, after preparation. Rate and ETA are
	// measured from here so the setup time does not drag the reported rate down.
	workStart time.Time
	finished  time.Time

	total, done, failed int
	// counters holds job-specific tallies that do not fit done/total, such as the
	// message sync's added-messages and added-reactions counts.
	counters map[string]int

	lastErr string
}

// Status is a snapshot of a job, safe to render or marshal.
type Status struct {
	Phase      Phase          `json:"phase"`
	Step       string         `json:"step,omitempty"`
	Total      int            `json:"total"`
	Done       int            `json:"done"`
	Failed     int            `json:"failed"`
	Remaining  int            `json:"remaining"`
	Counters   map[string]int `json:"counters,omitempty"`
	Elapsed    string         `json:"elapsed,omitempty"`
	Rate       string         `json:"rate,omitempty"`
	ETA        string         `json:"eta,omitempty"`
	StartedAt  string         `json:"startedAt,omitempty"`
	FinishedAt string         `json:"finishedAt,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Running reports whether the job is still working.
func (s Status) Running() bool {
	return s.Phase == PhasePreparing || s.Phase == PhaseRunning
}

// Counter returns a named tally, or 0 when the job does not keep one.
func (s Status) Counter(name string) int {
	return s.Counters[name]
}

// tryStart claims the job for this caller and resets it, reporting false if a
// run is already in flight.
func (j *Job) tryStart(step string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.phase == PhasePreparing || j.phase == PhaseRunning {
		return false
	}
	// Reset field by field rather than assigning a fresh struct, which would copy
	// a zero Mutex over the one this call is holding.
	j.phase, j.step, j.started = PhasePreparing, step, time.Now()
	j.workStart, j.finished = time.Time{}, time.Time{}
	j.total, j.done, j.failed = 0, 0, 0
	j.counters = make(map[string]int)
	j.lastErr = ""
	return true
}

// begin moves the job from preparation into its work loop, now that the size of
// the work is known. Rate and ETA are measured from this moment.
func (j *Job) begin(step string, total int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.phase, j.step = PhaseRunning, step
	j.workStart = time.Now()
	j.total = total
}

// record folds one completed unit of work into the job and returns the new done
// count, so callers can log on an interval without a second lock round-trip.
func (j *Job) record(err error) (done int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done++
	if err != nil {
		j.failed++
		j.lastErr = err.Error()
	}
	return j.done
}

// add bumps a named job-specific tally.
func (j *Job) add(name string, n int) {
	if n == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.counters[name] += n
}

func (j *Job) finish(phase Phase, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.phase, j.step = phase, ""
	j.finished = time.Now()
	if err != nil {
		j.lastErr = err.Error()
	}
}

// fail records a setup error and ends the run. Returns the error so callers can
// `return j.fail(err)` out of a run function.
func (j *Job) fail(err error) {
	slog.Error(j.name+" failed", slog.Any("err", err))
	j.finish(PhaseFailed, err)
}

// recover turns a panic in a work loop into a failed job rather than a dead
// process. Deferred at the top of every run function.
func (j *Job) recoverPanic() {
	if rec := recover(); rec != nil {
		slog.Error(j.name+" panicked", slog.Any("panic", rec))
		j.finish(PhaseFailed, fmt.Errorf("panic: %v", rec))
	}
}

func (j *Job) status() Status {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.phase == "" {
		j.phase = PhaseIdle
	}
	s := Status{
		Phase:     j.phase,
		Step:      j.step,
		Total:     j.total,
		Done:      j.done,
		Failed:    j.failed,
		Remaining: j.total - j.done,
		Error:     j.lastErr,
	}
	if len(j.counters) > 0 {
		s.Counters = make(map[string]int, len(j.counters))
		for k, v := range j.counters {
			s.Counters[k] = v
		}
	}
	if j.phase == PhaseIdle {
		return s
	}

	s.StartedAt = j.started.Format(time.RFC3339)
	end := j.finished
	if end.IsZero() {
		end = time.Now()
	} else {
		s.FinishedAt = j.finished.Format(time.RFC3339)
	}

	s.Elapsed = end.Sub(j.started).Round(time.Second).String()
	if j.workStart.IsZero() {
		return s
	}
	if rate := float64(j.done) / end.Sub(j.workStart).Seconds(); rate > 0 {
		s.Rate = fmt.Sprintf("%.2f %s/s", rate, j.unit)
		if s.Remaining > 0 && j.phase == PhaseRunning {
			s.ETA = time.Duration(float64(s.Remaining) / rate * float64(time.Second)).Round(time.Second).String()
		}
	}
	return s
}
