package jobs

import (
	"log/slog"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// embedLogInterval is how many messages the backfill embeds between progress log
// lines. The job runs for hours, so the log is the only view of it that survives
// a restart of whatever kicked it off.
const embedLogInterval = 1000

var embedJob = newJob(KeyEmbeddings, "embedding backfill", "msg", 200)

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

	total, err := database.CountMessagesWithoutEmbeddings()
	if err != nil {
		embedJob.fail(err)
		return
	}
	embedJob.begin("embedding", total)
	slog.Info("embedding backfill started", slog.Int("messages", total))

	// Streamed in chunks, keyed on the last id seen. Materialising the whole
	// list cost hundreds of megabytes before any work started, which on a
	// memory-capped pod is the difference between finishing and being killed.
	var afterID string
	for {
		messages, err := database.GetMessagesWithoutEmbeddings(afterID, backfillChunkSize)
		if err != nil {
			embedJob.fail(err)
			return
		}
		if len(messages) == 0 {
			break
		}

		for _, m := range messages {
			done := embedJob.record(database.EmbedMessage(m.MessageID, m.Content))
			if done%embedLogInterval == 0 {
				s := embedJob.status()
				slog.Info("embedding backfill progress",
					slog.Int("done", s.Done), slog.Int("total", s.Total),
					slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
			}
		}

		// Advance past the chunk whether or not every message in it succeeded, so
		// a message that always fails cannot stall the backfill on it.
		afterID = messages[len(messages)-1].MessageID
	}

	embedJob.finish(PhaseDone, nil)
	s := embedJob.status()
	slog.Info("embedding backfill finished",
		slog.Int("embedded", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
