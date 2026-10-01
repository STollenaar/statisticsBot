// Package embedbackfill runs the one embedding backfill the bot may have in
// flight at a time. Embedding every message that lacks a vector takes hours, so
// the job outlives the request that starts it and both the HTTP route and the
// admin command drive this shared state rather than each holding their own.
package embedbackfill

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// progressLogInterval is how many messages the backfill embeds between progress
// log lines. The job runs for hours, so the log is the only view of it that
// survives a restart of whatever kicked it off.
const progressLogInterval = 1000

// job holds the state of the single backfill. Callers reach it through the
// package-level functions so there is exactly one job process-wide.
var job Job

type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhaseQuerying  Phase = "querying"
	PhaseEmbedding Phase = "embedding"
	PhaseDone      Phase = "done"
	PhaseFailed    Phase = "failed"
)

type Job struct {
	mu sync.Mutex

	phase   Phase
	started time.Time
	// embedStart is when the message list was in hand and embedding actually
	// began. Rate and ETA are measured from here, so the seconds spent querying
	// and loading the model do not drag the reported rate down.
	embedStart time.Time
	finished   time.Time
	total      int
	done       int
	failed     int
	lastErr    string
}

// Status is the snapshot of a backfill, built under the lock so a reader never
// sees a half-updated job.
type Status struct {
	Phase      Phase  `json:"phase"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Failed     int    `json:"failed"`
	Remaining  int    `json:"remaining"`
	Elapsed    string `json:"elapsed,omitempty"`
	Rate       string `json:"rate,omitempty"`
	ETA        string `json:"eta,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Running reports whether the job is still working.
func (s Status) Running() bool {
	return s.Phase == PhaseQuerying || s.Phase == PhaseEmbedding
}

// Start kicks off a backfill of every message missing an embedding and returns
// immediately. It reports false when one is already running, so a second caller
// cannot start a competing job.
func Start() bool {
	if !job.tryStart() {
		return false
	}
	go run()
	return true
}

// Current returns the progress of the running or last-finished backfill.
func Current() Status {
	return job.status()
}

func (b *Job) tryStart() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.phase == PhaseQuerying || b.phase == PhaseEmbedding {
		return false
	}
	// Reset field by field rather than assigning a fresh struct, which would
	// copy a zero Mutex over the one this call is holding.
	b.phase, b.started = PhaseQuerying, time.Now()
	b.embedStart, b.finished = time.Time{}, time.Time{}
	b.total, b.done, b.failed = 0, 0, 0
	b.lastErr = ""
	return true
}

func (b *Job) setTotal(total int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.phase = PhaseEmbedding
	b.embedStart = time.Now()
	b.total = total
}

func (b *Job) record(err error) (done int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done++
	if err != nil {
		b.failed++
		b.lastErr = err.Error()
	}
	return b.done
}

func (b *Job) finish(phase Phase, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.phase = phase
	b.finished = time.Now()
	if err != nil {
		b.lastErr = err.Error()
	}
}

func (b *Job) status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.phase == "" {
		b.phase = PhaseIdle
	}
	s := Status{
		Phase:     b.phase,
		Total:     b.total,
		Done:      b.done,
		Failed:    b.failed,
		Remaining: b.total - b.done,
		Error:     b.lastErr,
	}
	if b.phase == PhaseIdle {
		return s
	}

	s.StartedAt = b.started.Format(time.RFC3339)
	end := b.finished
	if end.IsZero() {
		end = time.Now()
	} else {
		s.FinishedAt = b.finished.Format(time.RFC3339)
	}

	s.Elapsed = end.Sub(b.started).Round(time.Second).String()
	if b.embedStart.IsZero() {
		return s
	}
	if rate := float64(b.done) / end.Sub(b.embedStart).Seconds(); rate > 0 {
		s.Rate = fmt.Sprintf("%.1f msg/s", rate)
		if s.Remaining > 0 && b.phase == PhaseEmbedding {
			s.ETA = (time.Duration(float64(s.Remaining) / rate * float64(time.Second))).Round(time.Minute).String()
		}
	}
	return s
}

// run embeds every message that is missing a vector.
//
// This runs sequentially: the embedding pipeline serializes every call on a
// single run lock, so spreading the messages over goroutines would only queue
// them up on that lock without generating anything in parallel.
func run() {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("embedding backfill panicked", slog.Any("panic", rec))
			job.finish(PhaseFailed, fmt.Errorf("panic: %v", rec))
		}
	}()

	messages, err := database.GetMessagesWithoutEmbeddings(0)
	if err != nil {
		slog.Error("embedding backfill failed to list messages", slog.Any("err", err))
		job.finish(PhaseFailed, err)
		return
	}
	job.setTotal(len(messages))
	slog.Info("embedding backfill started", slog.Int("messages", len(messages)))

	for _, m := range messages {
		done := job.record(database.EmbedMessage(m.MessageID, m.Content))
		if done%progressLogInterval == 0 {
			s := job.status()
			slog.Info("embedding backfill progress",
				slog.Int("done", s.Done), slog.Int("total", s.Total),
				slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
		}
	}

	job.finish(PhaseDone, nil)
	s := job.status()
	slog.Info("embedding backfill finished",
		slog.Int("embedded", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
