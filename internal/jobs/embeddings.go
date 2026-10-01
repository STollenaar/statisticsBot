package jobs

import (
	"log/slog"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// embedLogInterval is how many messages the backfill embeds between progress log
// lines. The job runs for hours, so the log is the only view of it that survives
// a restart of whatever kicked it off.
const embedLogInterval = 1000

var embedJob = &Job{name: "embedding backfill", unit: "msg"}

// StartEmbeddingBackfill embeds every stored message that does not have a vector
// yet, so semantic search can cover historical messages, and returns
// immediately. It reports false when a backfill is already running.
func StartEmbeddingBackfill() bool {
	if !embedJob.tryStart("querying") {
		return false
	}
	go runEmbeddingBackfill()
	return true
}

// EmbeddingBackfill returns the progress of the running or last-finished backfill.
func EmbeddingBackfill() Status {
	return embedJob.status()
}

// runEmbeddingBackfill embeds every message that is missing a vector.
//
// This runs sequentially: the embedding pipeline serializes every call on a
// single run lock, so spreading the messages over goroutines would only queue
// them up on that lock without generating anything in parallel.
func runEmbeddingBackfill() {
	defer embedJob.recoverPanic()

	messages, err := database.GetMessagesWithoutEmbeddings(0)
	if err != nil {
		embedJob.fail(err)
		return
	}
	embedJob.begin("embedding", len(messages))
	slog.Info("embedding backfill started", slog.Int("messages", len(messages)))

	for _, m := range messages {
		done := embedJob.record(database.EmbedMessage(m.MessageID, m.Content))
		if done%embedLogInterval == 0 {
			s := embedJob.status()
			slog.Info("embedding backfill progress",
				slog.Int("done", s.Done), slog.Int("total", s.Total),
				slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
		}
	}

	embedJob.finish(PhaseDone, nil)
	s := embedJob.status()
	slog.Info("embedding backfill finished",
		slog.Int("embedded", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
