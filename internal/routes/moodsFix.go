package routes

import (
	"net/http"

	"github.com/stollenaar/statisticsbot/internal/jobs"
)

func addFixMoods(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixMoods", startMissingMoods)
	mux.HandleFunc("GET /fixMoods", getMissingMoods)
}

// startMissingMoods kicks off the mood classification of every stored message
// that has not been scored yet, and returns without waiting for it. The job runs
// for far longer than any sane HTTP timeout, so progress is read back from
// GET /fixMoods.
func startMissingMoods(w http.ResponseWriter, r *http.Request) {
	if !jobs.StartMoodBackfill() {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "a mood backfill is already running",
			"status": jobs.MoodBackfill(),
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"message": "mood backfill started, poll GET /fixMoods for progress",
		"status":  jobs.MoodBackfill(),
	})
}

// getMissingMoods reports the progress of the current or last backfill.
func getMissingMoods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, jobs.MoodBackfill())
}
