package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/stollenaar/statisticsbot/internal/backup"
)

func addBackup(mux *http.ServeMux) {
	mux.HandleFunc("POST /backup", runBackup)
}

// runBackup snapshots the database and uploads it to Backblaze. It has to run
// in this process because DuckDB only allows a single writer, so a separate job
// cannot open the database file while the bot holds it.
func runBackup(w http.ResponseWriter, r *http.Request) {
	result, err := backup.Run(r.Context())
	if errors.Is(err, backup.ErrInProgress) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		slog.Error("backup failed", slog.Any("err", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   result.Key,
		"bytes": result.Bytes,
		"took":  result.Took.Round(time.Millisecond).String(),
	})
}
