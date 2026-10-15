package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
)

// JobRun is one background job's persisted state. Times are zero when the job
// has not reached that point yet.
type JobRun struct {
	Name          string
	Phase         string
	Step          string
	Total         int
	Done          int
	Failed        int
	Counters      map[string]int
	StartedAt     time.Time
	WorkStartedAt time.Time
	UpdatedAt     time.Time
	FinishedAt    time.Time
	Error         string
}

// PhaseInterrupted is what a run left mid-flight by a restart becomes. It is
// written only by MarkInterruptedJobRuns, never by a running job.
const PhaseInterrupted = "interrupted"

const jobRunColumns = `name, phase, COALESCE(step, ''), total, done, failed, COALESCE(counters, ''),
	started_at, work_started_at, updated_at, finished_at, COALESCE(error, '')`

func scanJobRun(s rowScanner) (JobRun, error) {
	var j JobRun
	var counters string
	var started, workStarted, updated, finished sql.NullTime
	if err := s.Scan(&j.Name, &j.Phase, &j.Step, &j.Total, &j.Done, &j.Failed, &counters,
		&started, &workStarted, &updated, &finished, &j.Error); err != nil {
		return j, err
	}
	j.StartedAt, j.WorkStartedAt, j.UpdatedAt, j.FinishedAt = started.Time, workStarted.Time, updated.Time, finished.Time
	if counters != "" {
		if err := json.Unmarshal([]byte(counters), &j.Counters); err != nil {
			// A malformed tally is not worth failing the read over; the phase and
			// progress are the parts that matter.
			slog.Warn("ignoring unreadable job counters", slog.String("job", j.Name), slog.Any("err", err))
		}
	}
	return j, nil
}

// SaveJobRun upserts a job's state. Called on every phase change and on a
// throttled interval while a job runs, so it must stay cheap.
func SaveJobRun(j JobRun) error {
	var counters string
	if len(j.Counters) > 0 {
		b, err := json.Marshal(j.Counters)
		if err != nil {
			return err
		}
		counters = string(b)
	}

	_, err := duckdbClient.Exec(`
		INSERT INTO job_runs
			(name, phase, step, total, done, failed, counters,
			 started_at, work_started_at, updated_at, finished_at, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET
			phase = EXCLUDED.phase,
			step = EXCLUDED.step,
			total = EXCLUDED.total,
			done = EXCLUDED.done,
			failed = EXCLUDED.failed,
			counters = EXCLUDED.counters,
			started_at = EXCLUDED.started_at,
			work_started_at = EXCLUDED.work_started_at,
			updated_at = EXCLUDED.updated_at,
			finished_at = EXCLUDED.finished_at,
			error = EXCLUDED.error`,
		j.Name, j.Phase, j.Step, j.Total, j.Done, j.Failed, counters,
		nullTime(j.StartedAt), nullTime(j.WorkStartedAt), nullTime(time.Now()), nullTime(j.FinishedAt), j.Error,
	)
	return err
}

// nullTime keeps a zero time out of the column, so "never happened" reads back
// as NULL rather than year 1.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// GetJobRun returns one job's persisted state, reporting false when the job has
// never run.
func GetJobRun(name string) (JobRun, bool, error) {
	j, err := scanJobRun(duckdbClient.QueryRow(
		`SELECT `+jobRunColumns+` FROM job_runs WHERE name = ?`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return JobRun{}, false, nil
	}
	if err != nil {
		return JobRun{}, false, err
	}
	return j, true, nil
}

// MarkInterruptedJobRuns rewrites every run still recorded as in-flight to
// interrupted, returning their names. Called once at startup: a job is the only
// writer of its own row, so a row left mid-flight means the process that owned
// it died.
func MarkInterruptedJobRuns(inFlight []string) ([]string, error) {
	rows, err := duckdbClient.Query(
		`SELECT name FROM job_runs WHERE phase IN (?, ?)`, inFlight[0], inFlight[1])
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}

	// step is cleared because it described work no longer happening, and the
	// views prefer step over phase when showing what a job is doing. finished_at
	// is backdated to the last progress write rather than set to now: the run
	// ended when the process died, so timing it to the restart would inflate its
	// elapsed by however long the bot was down.
	if _, err := duckdbClient.Exec(`
		UPDATE job_runs
		SET phase = ?,
			step = '',
			finished_at = COALESCE(finished_at, updated_at),
			updated_at = ?
		WHERE phase IN (?, ?)`,
		PhaseInterrupted, time.Now(), inFlight[0], inFlight[1]); err != nil {
		return nil, err
	}
	return names, nil
}
