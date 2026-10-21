# statisticsBot

A Discord bot that records every message in a guild into a local DuckDB database
and answers questions about it: word counts, who said something most, charts,
LLM summaries, semantic search and mood analysis.

Everything runs locally against the DuckDB file. The only external services are
Discord itself, an Ollama endpoint for the LLM commands, AWS Parameter Store for
secrets, and Backblaze B2 for backups.

## How it works

The bot joins a guild and listens on the gateway. Every created or edited
message is written to `messages` (append-only — an edit inserts a new `version`
rather than updating in place), and in the background it is also embedded for
semantic search and classified for mood.

History is handled separately. On startup the bot walks each channel back to the
newest message it already has, and three long-running jobs fill the gaps:
missing messages, missing embeddings, missing moods. Those jobs take hours to
days, so they run detached, record progress in the database, and resume if the
process restarts.

| Package | Responsibility |
|---|---|
| `cmd/statisticsbot` | startup, gateway, command registration, signal handling |
| `internal/commands` | one package per slash command |
| `internal/database` | DuckDB access, schema migrations, ingestion |
| `internal/embeddings` | local ONNX inference — embeddings and mood, one hugot session |
| `internal/jobs` | the three background jobs and their shared state machine |
| `internal/routes` | internal HTTP API on `:8080` |
| `internal/backup` | DuckDB export, tar.zst, upload to B2 |

## Commands

| Command | What it does |
|---|---|
| `/count <word> [user] [channel]` | how many times a word was used |
| `/max [user] [word] [channel]` | who used a word the most. `user` and `word` are mutually exclusive |
| `/last <user> [word] [channel]` | when someone last said something |
| `/semantic <query> [limit]` | semantic search over message embeddings, paginated |
| `/summarize <unit>` | LLM summary of the last *unit* of messages in the channel |
| `/mood <unit>` | LLM mood analysis, grouped by topic |
| `/plot` | interactive chart builder (select menus plus a date-range modal) |
| `/help` | lists the commands |
| `/admin …` | operator-only, see below |

`unit` is a duration like `2h` or `3d`, parsed by `util.ParseTimeArg`.

### `/admin`

Gated on `ADMIN_USER_ID` matching the invoking user — one global operator, not a
per-guild role. (A role mask via `default_member_permissions` cannot express
that; see TODO.md for why.)

| Subcommand | What it does |
|---|---|
| `summary list` | page through recorded `/summarize` invocations, retry or re-post one |
| `semantic status` | embedding coverage and backfill progress |
| `semantic backfill` | start the embedding backfill |
| `semantic list` | page through recorded `/semantic` searches |
| `mood status` | mood classification coverage and backfill progress |
| `mood backfill` | start the mood backfill |
| `messages status` | message sync progress |
| `messages sync` | scan every channel for messages the database is missing |

## Background jobs

Three jobs, all driven by the same state machine in `internal/jobs`:

| Job | Work | Bound by |
|---|---|---|
| embedding backfill | embed messages with no vector | CPU |
| mood backfill | classify messages with no mood | CPU |
| message sync | walk every channel's history for missing messages | Discord REST rate limits |

Properties worth knowing:

- **Only one run of each job at a time.** A second start returns HTTP 409, or
  says so in Discord.
- **Progress is persisted** to `job_runs`, so a restart knows how far a job got.
- **A job interrupted by a restart is resumed automatically** at startup. This
  is safe because nothing is checkpointed: each job re-derives its remaining
  work from the database, so a resumed run simply processes what is still
  missing.
- **Throughput depends on the host.** Inference runs on hugot's pure-Go
  backend, so both backfills are CPU-bound and sequential behind a single
  pipeline lock. The mood classifier is a much larger model than the embedding
  one and is correspondingly slower per message. Over a large history these are
  long jobs — check `/admin … status` for the rate and ETA this deployment is
  actually achieving rather than assuming a figure.

## Internal HTTP API

Listens on `:8080`. Intended to stay inside the cluster; the CronJobs in
`terraform/kubernetes/cronjobs.tf` are its only callers.

| Route | Purpose |
|---|---|
| `GET /ping` | reachability check. The only route exempt from auth |
| `PUT /fixMessages` | start the message sync (202, returns immediately) |
| `GET /fixMessages` | message sync progress |
| `DELETE /fixMessages` | delete rows with a null date, empty content or empty guild |
| `PUT /fixEmbeddings` | start the embedding backfill (202) |
| `GET /fixEmbeddings` | embedding backfill progress |
| `PUT /fixMoods` | start the mood backfill (202) |
| `GET /fixMoods` | mood backfill progress |
| `PUT /fixEmojis` | fetch and store guild emoji images |
| `POST /backup` | export the database and upload it to B2 |
| `POST /userMessages` | look up a user or message; body is `{"type": "user"\|"message", …}` |
| `GET /debug` | runtime, container, DuckDB and job diagnostics. **Only registered when `DEBUG` is set**; 404 otherwise |
| `GET /debug/pprof/…` | Go pprof profiles (`heap`, `goroutine`, `allocs`, `profile`, `trace`). Same `DEBUG` gate |

Liveness and readiness are on a **separate** port (`HEALTH_PORT`), serving
`/health` and `/ready`. `/ready` only reports OK once the gateway is connected.

### Authentication

Every route except `GET /ping` requires the shared token, as either header:

```
Authorization: Bearer <token>
X-Auth-Token: <token>
```

Three states, and the third is the important one:

| Config | Behaviour |
|---|---|
| no token set | serves unauthenticated, with a startup `WARN` |
| token resolved | 401 without a valid token |
| token configured but unreadable | **503 on everything** — fails closed rather than silently serving open |

## Configuration

All configuration is environment variables. Secrets follow a pair convention:
`X` holds a literal value, `AWS_X` holds an AWS Parameter Store path read at
runtime. Set one or the other.

### Required

| Variable | Notes |
|---|---|
| `DISCORD_TOKEN` or `AWS_PARAMETER_NAME` | bot token, or the SSM path holding it. Startup fails without one |
| `DUCKDB_PATH` | directory holding `statsbot.db` |

### General

| Variable | Default | Notes |
|---|---|---|
| `ADMIN_USER_ID` | — | the single user allowed to run `/admin` |
| `HEALTH_PORT` | — | port for `/health` and `/ready` |
| `AWS_REGION` | — | used by the SSM and B2 clients |
| `TERMINAL_REGEX` | `(\.\|,\|:\|;\|\?\|!)$` | trailing punctuation stripped during word counting |
| `DEBUG` | `false` | enables `GET /debug`; also settable with the `-debug` flag |

### Internal API

| Variable | Notes |
|---|---|
| `HTTP_AUTH_TOKEN` / `AWS_HTTP_AUTH_TOKEN` | shared token for every route but `GET /ping` |

### Ollama

| Variable | Default | Notes |
|---|---|---|
| `OLLAMA_URL` | — | **include the path**, e.g. `host:11434/v1/chat/completions`. The client does not append one |
| `OLLAMA_MODEL` | `llama3.2:3b` | |
| `OLLAMA_AUTH_TYPE` | — | `basic` or `api_key`; unset means no auth |
| `OLLAMA_AUTH_USERNAME` / `AWS_OLLAMA_AUTH_USERNAME` | — | for `basic` |
| `OLLAMA_AUTH_PASSWORD` / `AWS_OLLAMA_AUTH_PASSWORD` | — | for `basic` |
| `OLLAMA_API_KEY` / `AWS_OLLAMA_API_KEY` | — | for `api_key` |

The request and response shapes are OpenAI-style (`max_tokens`,
`choices[].message.content`), which is why the URL must point at
`/v1/chat/completions` and not Ollama's native `/api/chat`.

### Backups

| Variable | Default | Notes |
|---|---|---|
| `B2_BUCKET` | — | destination bucket |
| `B2_PREFIX` | `statisticsbot` | key prefix |
| `B2_REGION` | — | |
| `B2_ENDPOINT` | derived from `B2_REGION` | `https://s3.<region>.backblazeb2.com` |
| `B2_KEY_ID` / `AWS_B2_KEY_ID` | — | |
| `B2_APPLICATION_KEY` / `AWS_B2_APPLICATION_KEY` | — | |

Credentials are read per run, so rotating them in SSM takes effect without a
restart.

## Flags

| Flag | Default | Notes |
|---|---|---|
| `-guild <id>` | — | register commands to one guild instead of globally |
| `-debug` | `false` | debug logging |
| `-rmcmd` | `true` | deregister commands on shutdown |
| `-purgecmd` | `false` | delete all registered commands and exit |

## Running locally

```sh
export DISCORD_TOKEN=...          # or AWS_PARAMETER_NAME
export DUCKDB_PATH=./             # statsbot.db is created here
export ADMIN_USER_ID=...
go run ./cmd/statisticsbot -guild <test-guild-id>
```

Schema migrations in `internal/database/changelog/*.sql` are applied in
filename order on startup and recorded in `database_changelog` with a checksum —
editing an applied migration is refused rather than silently diverging. Add a
new numbered file instead.

The embedding and mood models are downloaded from HuggingFace into `./models/`
relative to the working directory on first use (~90MB and ~500MB).

Running with `-guild` is strongly preferred while developing: global command
registration propagates slowly and is shared with the real bot.

## Deployment

GitHub Actions builds a static binary, packages it with `Dockerfile`, pushes to
ECR and applies the terraform. Two terraform states:

- `terraform/kubernetes` — service, PVC and the CronJobs that call the HTTP API
- `terraform/kubernetes_statisticsbot` — the deployment itself

Apply `terraform/kubernetes` first: the deployment reads values from that
state's outputs.

Secrets reach the bot through Vault. The pod is annotated for the Vault agent,
which injects AWS credentials at `/vault/secrets/aws/credentials`; the bot then
reads each `AWS_*` path from Parameter Store itself.

The CronJobs are mostly `suspend = true` — they exist to be triggered manually
or enabled temporarily. Only the backup runs on a schedule, daily at 03:00.

## Backup and restore

`POST /backup` runs `EXPORT DATABASE` to Parquet, tars it with zstd compression
and uploads it to B2 under `<B2_PREFIX>/statsbot-<timestamp>.tar.zst`. Nothing
is ever overwritten; pruning old backups is not automated.

Parquet rather than a copy of `statsbot.db`, because DuckDB's storage format is
not guaranteed readable across versions while the exported `schema.sql` and
`load.sql` pair is.

To restore, replace the database wholesale — there is no partial or incremental
restore path:

```sh
# 1. stop the bot, so nothing holds the DuckDB file
kubectl scale deployment/statisticsbot --replicas=0

# 2. fetch and unpack the backup
aws s3 cp --endpoint-url "$B2_ENDPOINT" \
    "s3://$B2_BUCKET/$B2_PREFIX/statsbot-<timestamp>.tar.zst" .
tar --zstd -xf statsbot-<timestamp>.tar.zst

# 3. import into a fresh database file, then put it where DUCKDB_PATH points
duckdb statsbot.db "IMPORT DATABASE '<unpacked-dir>'"

# 4. start the bot again
kubectl scale deployment/statisticsbot --replicas=1
```

The import must target a **fresh** file. Importing over an existing database
will collide on tables that already exist.

Two things the bot repairs by itself afterwards, so a slightly stale backup is
usually fine: the startup sync walks each channel for messages newer than what
was restored, and the embedding and mood backfills refill whatever the restored
snapshot is missing.

## Notes and known gaps

`TODO.md` is the working list, including items deliberately not being done and
why. The ones most likely to surprise:

- Inference is CPU-only, so backfilling a large history is slow. How slow
  depends entirely on the host; the status views report the live rate and ETA.
- There are no tests and no `go vet` gate in CI, by decision rather than
  oversight — see TODO.md.
- `DELETE /fixMessages` deletes rows. Read it before running it.

### Debugging memory

With `DEBUG` set, `GET /debug` reports the container limit and usage, the Go
heap broken down (live, fragmentation, held-but-idle, non-heap), process RSS,
DuckDB's usage per allocation tag, and every job's state. `processRSS` is the
figure the container limit is enforced against; Go's own accounting excludes the
ONNX models.

For *what* is holding memory rather than how much, the pprof endpoints are
behind the same gate:

```sh
go tool pprof http://<host>:8080/debug/pprof/heap     # add the auth header via -http-header
```

Inference is the usual answer. hugot's pure-Go backend caches a compiled graph
per distinct input shape and never evicts, so the retained graphs are live Go
heap that no amount of GC pressure will reclaim. The mood pipeline bounds this
with `WithFixedPadding` (one shape, one graph); the embedding pipeline has no
such option and grows with the variety of message lengths it has seen.
