package jobs

import (
	"log/slog"

	"github.com/stollenaar/statisticsbot/internal/database"
)

// moodLogInterval is how many messages the backfill classifies between progress
// log lines.
const moodLogInterval = 500

var moodJob = &Job{name: "mood backfill", unit: "msg"}

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

	messages, err := database.GetMessagesWithoutMoods(0)
	if err != nil {
		moodJob.fail(err)
		return
	}
	moodJob.begin("scoring", len(messages))
	slog.Info("mood backfill started", slog.Int("messages", len(messages)))

	for _, m := range messages {
		done := moodJob.record(database.MoodMessage(m.MessageID, m.Content))
		if done%moodLogInterval == 0 {
			s := moodJob.status()
			slog.Info("mood backfill progress",
				slog.Int("done", s.Done), slog.Int("total", s.Total),
				slog.Int("failed", s.Failed), slog.String("rate", s.Rate), slog.String("eta", s.ETA))
		}
	}

	moodJob.finish(PhaseDone, nil)
	s := moodJob.status()
	slog.Info("mood backfill finished",
		slog.Int("scored", s.Done-s.Failed), slog.Int("failed", s.Failed), slog.String("took", s.Elapsed))
}
