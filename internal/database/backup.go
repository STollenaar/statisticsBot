package database

import (
	"fmt"
	"log/slog"
	"strings"
)

// ExportSnapshot writes a transactionally consistent snapshot of the database
// into dir using DuckDB's EXPORT DATABASE. Restore it with
// IMPORT DATABASE '<dir>' against a fresh database file.
//
// Parquet is used rather than copying statsbot.db because the DuckDB storage
// format is not guaranteed to be readable across versions, while the exported
// schema.sql/load.sql pair is. The export has to happen through this connection:
// DuckDB is single-writer, so a sidecar process cannot open the same file while
// the bot is running.
func ExportSnapshot(dir string) error {
	// Flush the WAL first so the export writes out as little pending state as
	// possible. This fails while other connections have open transactions, which
	// is not a problem: EXPORT DATABASE is consistent either way.
	if _, err := duckdbClient.Exec(`CHECKPOINT`); err != nil {
		slog.Warn("skipping checkpoint before export", slog.Any("err", err))
	}

	quoted := strings.ReplaceAll(dir, "'", "''")
	_, err := duckdbClient.Exec(fmt.Sprintf(`EXPORT DATABASE '%s' (FORMAT PARQUET, COMPRESSION ZSTD)`, quoted))
	if err != nil {
		return fmt.Errorf("failed to export database: %w", err)
	}
	return nil
}
