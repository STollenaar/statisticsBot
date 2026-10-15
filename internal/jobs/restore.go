package jobs

import (
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/stollenaar/statisticsbot/internal/database"
)

// Persistence keys. These name rows in job_runs, so changing one orphans the
// old row and loses the ability to resume whatever it held.
const (
	KeyEmbeddings = "embeddings"
	KeyMoods      = "moods"
	KeyMessages   = "messages"
)

// Restore loads each job's last known state from the database and returns the
// jobs the previous process left unfinished.
//
// A row still recorded as preparing or running means the process died mid-run:
// a job is the only writer of its own row, so nothing else could have left it
// that way. Those are rewritten to interrupted, which is a terminal phase — the
// admin views offer to start them again and Running() reports false — so a
// restart never looks like it has a job in flight when it does not.
func Restore() ([]string, error) {
	interrupted, err := database.MarkInterruptedJobRuns(
		[]string{string(PhasePreparing), string(PhaseRunning)})
	if err != nil {
		return nil, err
	}

	for _, job := range registry {
		job.loadPersisted()
	}

	if len(interrupted) > 0 {
		slog.Warn("jobs were interrupted by a restart", slog.Any("jobs", interrupted))
	}
	return interrupted, nil
}

// ResumeInterrupted starts every job left interrupted by a restart, returning
// the names of those it started.
//
// Resuming is just starting: each job re-derives its work from the database —
// the messages still missing an embedding, a mood, or a stored id — so a new run
// naturally picks up where the old one stopped. Nothing is checkpointed beyond
// the progress figures, and the new run's total is the work that remains.
func ResumeInterrupted(client *bot.Client) []string {
	var resumed []string
	for key, job := range registry {
		if job.status().Phase != PhaseInterrupted {
			continue
		}

		var started bool
		switch key {
		case KeyEmbeddings:
			started = StartEmbeddingBackfill()
		case KeyMoods:
			started = StartMoodBackfill()
		case KeyMessages:
			started = StartMessageSync(client)
		default:
			slog.Warn("no resume path for job", slog.String("job", key))
			continue
		}

		if started {
			slog.Info("resumed interrupted job", slog.String("job", job.name))
			resumed = append(resumed, job.name)
		}
	}
	return resumed
}
