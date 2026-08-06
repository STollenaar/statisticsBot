package routes

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// embeddingBackfillWorkers bounds how many embeddings are generated in parallel
// so the backfill does not overwhelm the embedding endpoint.
const embeddingBackfillWorkers = 4

func addFixEmbeddings(mux *http.ServeMux) {
	mux.HandleFunc("PUT /fixEmbeddings", addMissingEmbeddings)
}

// addMissingEmbeddings generates embeddings for every stored message that does
// not have one yet, so semantic search can cover historical messages.
func addMissingEmbeddings(w http.ResponseWriter, r *http.Request) {
	messages, err := database.GetMessagesWithoutEmbeddings(0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var waitGroup sync.WaitGroup
	sem := make(chan struct{}, embeddingBackfillWorkers)

	for _, m := range messages {
		waitGroup.Add(1)
		sem <- struct{}{}
		go func(m util.MessageObject) {
			defer waitGroup.Done()
			defer func() { <-sem }()
			database.EmbedMessage(m.MessageID, m.Content)
		}(m)
	}
	waitGroup.Wait()

	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("embedded %d messages", len(messages))})
}
