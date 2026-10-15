-- job_runs holds the latest run of each background job, so progress survives a
-- restart. One row per job: the jobs themselves are resumable because each
-- re-derives its work from the database (the messages still missing an
-- embedding, a mood, or a stored id), so there is nothing to checkpoint beyond
-- what the run was doing and how far it got.
--
-- counters is the job-specific tally map as json, because which keys exist
-- depends on the job (the message sync counts added messages and reactions;
-- the backfills count neither).
--
-- A row left in 'preparing' or 'running' means the process died mid-run: the
-- only writer is the job itself, so nothing else could have cleared it. Startup
-- rewrites those to 'interrupted'.
CREATE TABLE IF NOT EXISTS job_runs (
    name VARCHAR PRIMARY KEY,
    phase VARCHAR NOT NULL,
    step VARCHAR,
    total INTEGER NOT NULL DEFAULT 0,
    done INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    counters VARCHAR,
    started_at TIMESTAMP,
    work_started_at TIMESTAMP,
    updated_at TIMESTAMP,
    finished_at TIMESTAMP,
    error VARCHAR
);
