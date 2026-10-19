package database

import (
	"database/sql"
	"errors"
)

// DebugInfo is a cheap snapshot of DuckDB's state, for the debug endpoint. Only
// settings and counters DuckDB already tracks — nothing here scans a table, so
// it stays safe to poll on a loaded instance.
type DebugInfo struct {
	MemoryLimit      string `json:"memoryLimit"`
	Threads          int    `json:"threads"`
	TempDirectory    string `json:"tempDirectory"`
	MemoryUsedBytes  int64  `json:"memoryUsedBytes"`
	TempStoredBytes  int64  `json:"tempStoredBytes"`
	MessageCount     int64  `json:"messageCount"`
	EmbeddingCount   int64  `json:"embeddingCount"`
	MoodCount        int64  `json:"moodCount"`
	WALCheckpointErr string `json:"walCheckpointError,omitempty"`
}

// GetDebugInfo gathers DuckDB's own view of itself.
//
// MemoryLimit is the interesting one: DuckDB sizes it from detected *system*
// memory, which in a container is the host's rather than the cgroup's, so a
// value far above the pod's limit means DuckDB will keep allocating instead of
// spilling to disk.
func GetDebugInfo() (DebugInfo, error) {
	var info DebugInfo

	if err := duckdbClient.QueryRow(`
		SELECT current_setting('memory_limit'),
		       current_setting('threads'),
		       current_setting('temp_directory')`,
	).Scan(&info.MemoryLimit, &info.Threads, &info.TempDirectory); err != nil {
		return info, err
	}

	// duckdb_memory() reports one row per allocation tag; the totals are what
	// matter here.
	var used, temp sql.NullInt64
	if err := duckdbClient.QueryRow(
		`SELECT SUM(memory_usage_bytes), SUM(temporary_storage_bytes) FROM duckdb_memory()`,
	).Scan(&used, &temp); err != nil {
		return info, err
	}
	info.MemoryUsedBytes, info.TempStoredBytes = used.Int64, temp.Int64

	// Row counts come from the tables directly rather than latest_messages, so
	// this stays a metadata-cheap query instead of a grouped scan.
	if err := duckdbClient.QueryRow(`
		SELECT (SELECT COUNT(*) FROM messages),
		       (SELECT COUNT(*) FROM message_embeddings),
		       (SELECT COUNT(*) FROM message_moods)`,
	).Scan(&info.MessageCount, &info.EmbeddingCount, &info.MoodCount); err != nil {
		return info, errors.Join(err, nil)
	}

	return info, nil
}
