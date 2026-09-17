package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// progressLogInterval is how many messages the backfill embeds between progress
// log lines. The job runs for hours, so the log is the only view of it that
// survives a restart of whatever kicked it off.
const progressLogInterval = 1000

// backfill holds the state of the one embedding backfill that may run at a
// time. Embedding the messages that currently lack vectors takes hours, far
// longer than any sane HTTP timeout, so PUT /fixEmbeddings starts the job and
// returns immediately and GET /fixEmbeddings reports how far it has got.
var backfill embedBackfill

type backfillPhase string

const (
	phaseIdle      backfillPhase = "idle"
	phaseQuerying  backfillPhase = "querying"
	phaseEmbedding backfillPhase = "embedding"
	phaseDone      backfillPhase = "done"
	phaseFailed    backfillPhase = "failed"
)

type embedBackfill struct {
	mu sync.Mutex

	phase   backfillPhase
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

// backfillStatus is the JSON view of a backfill, built under the lock so a
// reader never sees a half-updated job.
type backfillStatus struct {
	Phase      backfillPhase `json:"phase"`
	Total      int           `json:"total"`
	Done       int           `json:"done"`
	Failed     int           `json:"failed"`
	Remaining  int           `json:"remaining"`
	Elapsed    string        `json:"elapsed,omitempty"`
	Rate       string        `json:"rate,omitempty"`
	ETA        string        `json:"eta,omitempty"`
	StartedAt  string        `json:"startedAt,omitempty"`
	FinishedAt string        `json:"finishedAt,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// tryStart claims the job for this caller, reporting false if one is already
// running so a second PUT cannot start a competing backfill.
func (b *embedBackfill) tryStart() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.phase == phaseQuerying || b.phase == phaseEmbedding {
		return false
	}
	// Reset field by field rather than assigning a fresh struct, which would
	// copy a zero Mutex over the one this call is holding.
	b.phase, b.started = phaseQuerying, time.Now()
	b.embedStart, b.finished = time.Time{}, time.Time{}
	b.total, b.done, b.failed = 0, 0, 0
	b.lastErr = ""
	return true
}

func (b *embedBackfill) setTotal(total int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.phase = phaseEmbedding
	b.embedStart = time.Now()
	b.total = total
}

func (b *embedBackfill) record(err error) (done int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done++
	if err != nil {
		b.failed++
		b.lastErr = err.Error()
	}
	return b.done
}

func (b *embedBackfill) finish(phase backfillPhase, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.phase = phase
	b.finished = time.Now()
	if err != nil {
		b.lastErr = err.Error()
	}
}

func (b *embedBackfill) status() backfillStatus {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := backfillStatus{
		Phase:     b.phase,
		Total:     b.total,
		Done:      b.done,
		Failed:    b.failed,
		Remaining: b.total - b.done,
		Error:     b.lastErr,
	}
	if b.phase == phaseIdle {
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
		if s.Remaining > 0 && b.phase == phaseEmbedding {
			s.ETA = (time.Duration(float64(s.Remaining) / rate * float64(time.Second))).Round(time.Minute).String()
		}
	}
	return s
}

func addFixEmbeddings(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixEmbeddings", startMissingEmbeddings)
	mux.HandleFunc("GET /fixEmbeddings", getMissingEmbeddings)
}

// startMissingEmbeddings kicks off the backfill of every stored message that
// does not have an embedding yet, so semantic search can cover historical
// messages, and returns without waiting for it.
func startMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	if !backfill.tryStart() {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "an embedding backfill is already running",
			"status": backfill.status(),
		})
		return
	}

	go runBackfill()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"message": "embedding backfill started, poll GET /fixEmbeddings for progress",
		"status":  backfill.status(),
	})
}

// getMissingEmbeddings reports the progress of the current or last backfill.
func getMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, backfill.status())
}

// runBackfill embeds every message that is missing a vector.
//
// This runs sequentially: the embedding pipeline serializes every call on a
// single run lock, so spreading the messages over goroutines would only queue
// them up on that lock without generating anything in parallel.
func runBackfill() {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("embedding backfill panicked", slog.Any("panic", rec))
			backfill.finish(phaseFailed, fmt.Errorf("panic: %v", rec))
		}
	}()

	messages, err := database.GetMessagesWithoutEmbeddings(0)
	if err != nil {
		slog.Error("embedding backfill failed to list messages", slog.Any("err", err))
		backfill.finish(phaseFailed, err)
		return
	}
	backfill.setTotal(len(messages))
	slog.Info("embedding backfill started", slog.Int("messages", len(messages)))

	for _, m := range messages {
		done := backfill.record(database.EmbedMessage(m.MessageID, m.Content))
		if done%progressLogInterval == 0 {
			s := backfill.status()
			slog.Info("embedding backfill progress",
				slog.Int("done", s.Done), slog.Int("total", s.Total),
				slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
		}
	}

	backfill.finish(phaseDone, nil)
	s := backfill.status()
	slog.Info("embedding backfill finished",
		slog.Int("embedded", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
