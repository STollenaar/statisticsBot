package routes

import (
	"net/http"

	"github.com/stollenaar/statisticsbot/internal/embedbackfill"
)

func addFixEmbeddings(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixEmbeddings", startMissingEmbeddings)
	mux.HandleFunc("GET /fixEmbeddings", getMissingEmbeddings)
}

// startMissingEmbeddings kicks off the backfill of every stored message that
// does not have an embedding yet, so semantic search can cover historical
// messages, and returns without waiting for it. The job takes hours, far longer
// than any sane HTTP timeout, so progress is read back from GET /fixEmbeddings.
func startMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	if !embedbackfill.Start() {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "an embedding backfill is already running",
			"status": embedbackfill.Current(),
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"message": "embedding backfill started, poll GET /fixEmbeddings for progress",
		"status":  embedbackfill.Current(),
	})
}

// getMissingEmbeddings reports the progress of the current or last backfill.
func getMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, embedbackfill.Current())
}
