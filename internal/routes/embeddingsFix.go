package routes

import (
	"fmt"
	"net/http"

	"github.com/stollenaar/statisticsbot/internal/database"
)

func addFixEmbeddings(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixEmbeddings", addMissingEmbeddings)
}

// addMissingEmbeddings generates embeddings for every stored message that does
// not have one yet, so semantic search can cover historical messages.
//
// This runs sequentially: the embedding pipeline serializes every call on a
// single run lock, so spreading the messages over goroutines would only queue
// them up on that lock without generating anything in parallel.
func addMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	messages, err := database.GetMessagesWithoutEmbeddings(0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	for _, m := range messages {
		database.EmbedMessage(m.MessageID, m.Content)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("embedded %d messages", len(messages))})
}
