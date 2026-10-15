package embeddings

import (
	"context"
	"log/slog"
	"sync"

	"github.com/knights-analytics/hugot"
)

// Pipeline names. hugot keys its registry on these: NewPipeline stores a
// pipeline under its name and refuses a second one with the same name, and
// GetPipeline is how it is read back.
const (
	embedPipelineName = "semantic-embeddings"
	moodPipelineName  = "mood-classification"
)

var (
	sessionOnce sync.Once
	sessionErr  error
	session     *hugot.Session
)

// ensureSession returns the one hugot session this package uses, creating it on
// first call.
//
// Everything goes through a single session on purpose. The session is hugot's
// registry: it owns every pipeline created on it, deduplicates the underlying
// models by path so two pipelines over one model load it once, and releases the
// lot in a single Destroy. Holding pipelines in package variables instead would
// put their lifetime outside the thing that is supposed to manage it.
func ensureSession() (*hugot.Session, error) {
	sessionOnce.Do(func() {
		session, sessionErr = hugot.NewGoSession(context.Background())
		if sessionErr != nil {
			slog.Error("failed to create hugot session", slog.Any("err", sessionErr))
		}
	})
	return session, sessionErr
}

// Close releases the session and every pipeline and model on it. Safe to call
// when nothing was ever initialized, and safe to call twice — hugot's Destroy is
// idempotent — so a caller can defer it unconditionally.
//
// Without this the compiled graphs and loaded weights are only reclaimed when
// the process exits.
//
// Both run locks are taken first. Destroy closes the models, and a backfill
// goroutine mid-call would otherwise have them freed underneath it. Neither
// caller ever holds both locks, so acquiring them here in a fixed order cannot
// deadlock, and each is only held for one message at a time.
func Close() error {
	if session == nil {
		return nil
	}

	runMu.Lock()
	defer runMu.Unlock()
	moodMu.Lock()
	defer moodMu.Unlock()

	slog.Info("Closing embedding session")
	return session.Destroy()
}
