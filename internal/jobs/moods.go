package jobs

import (
	"log/slog"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// moodLogInterval is how many messages the backfill classifies between progress
// log lines.
const moodLogInterval = 500

var moodJob = newJob(KeyMoods, "mood backfill", "msg", 200)

// StartMoodBackfill classifies the mood of every stored message that has not
// been scored yet, and returns immediately. It reports false when a backfill is
// already running.
func StartMoodBackfill() bool {
	if !moodJob.tryStart("querying") {
		return false
	}
	go runMoodBackfill()
	return true
}

// MoodBackfill returns the progress of the running or last-finished backfill.
func MoodBackfill() Status {
	return moodJob.status()
}

// runMoodBackfill scores every message that has no mood yet.
//
// Sequential for the same reason the embedding backfill is: one shared pipeline
// behind a single lock. This classifier is a larger model than the embedding one,
// so expect it to be the slower of the two per message.
func runMoodBackfill() {
	defer moodJob.recoverPanic()

	total, err := database.CountMessagesWithoutMoods()
	if err != nil {
		moodJob.fail(err)
		return
	}
	moodJob.begin("scoring", total)
	slog.Info("mood backfill started", slog.Int("messages", total))

	// Streamed in chunks, keyed on the last id seen. Materialising the whole
	// list cost hundreds of megabytes before any work started, which on a
	// memory-capped pod is the difference between finishing and being killed.
	var afterID string
	for {
		messages, err := database.GetMessagesWithoutMoods(afterID, backfillChunkSize)
		if err != nil {
			moodJob.fail(err)
			return
		}
		if len(messages) == 0 {
			break
		}

		for _, m := range messages {
			done := moodJob.record(database.MoodMessage(m.MessageID, m.Content))
			if done%moodLogInterval == 0 {
				s := moodJob.status()
				slog.Info("mood backfill progress",
					slog.Int("done", s.Done), slog.Int("total", s.Total),
					slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
			}
		}

		// Advance past the chunk whether or not every message in it succeeded, so
		// a message that always fails cannot stall the backfill on it.
		afterID = messages[len(messages)-1].MessageID
	}

	moodJob.finish(PhaseDone, nil)
	s := moodJob.status()
	slog.Info("mood backfill finished",
		slog.Int("scored", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
