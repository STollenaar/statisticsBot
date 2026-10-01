package database

import (
	"log/slog"
	"time"
)

// SemanticInvocation is one recorded /semantic search.
type SemanticInvocation struct {
	ID          string
	GuildID     string
	ChannelID   string
	AuthorID    string
	Query       string
	Model       string
	PoolSize    int
	ResultCount int
	RequestedAt time.Time
	Status      string
	Error       string
}

const semanticInvocationColumns = `id, guild_id, channel_id, author_id, query, model, pool_size, result_count, requested_at, status, COALESCE(error, '')`

func scanSemanticInvocation(s rowScanner) (SemanticInvocation, error) {
	var inv SemanticInvocation
	err := s.Scan(&inv.ID, &inv.GuildID, &inv.ChannelID, &inv.AuthorID, &inv.Query, &inv.Model,
		&inv.PoolSize, &inv.ResultCount, &inv.RequestedAt, &inv.Status, &inv.Error)
	return inv, err
}

// SaveSemanticInvocation records a search. It is best-effort: a failure to log
// is not worth failing the user's search over, so it is logged and swallowed.
func SaveSemanticInvocation(inv SemanticInvocation) {
	_, err := duckdbClient.Exec(
		`INSERT INTO semantic_invocations
		 (id, guild_id, channel_id, author_id, query, model, pool_size, result_count, requested_at, status, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.GuildID, inv.ChannelID, inv.AuthorID, inv.Query, inv.Model,
		inv.PoolSize, inv.ResultCount, time.Now(), inv.Status, inv.Error,
	)
	if err != nil {
		slog.Error("failed to record semantic invocation", slog.String("id", inv.ID), slog.Any("err", err))
	}
}

func CountSemanticInvocations() (int, error) {
	var count int
	err := duckdbClient.QueryRow(`SELECT COUNT(*) FROM semantic_invocations`).Scan(&count)
	return count, err
}

// ListSemanticInvocations returns one page of searches, newest first.
func ListSemanticInvocations(page, pageSize int) ([]SemanticInvocation, error) {
	offset := (page - 1) * pageSize
	rows, err := duckdbClient.Query(`
		SELECT `+semanticInvocationColumns+`
		FROM semantic_invocations
		ORDER BY requested_at DESC
		LIMIT ? OFFSET ?`,
		pageSize, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SemanticInvocation
	for rows.Next() {
		inv, err := scanSemanticInvocation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, rows.Err()
}

// EmbeddingCoverage counts how many messages could have an embedding and how
// many actually do, for the model currently in use.
type EmbeddingCoverage struct {
	Messages   int
	Embedded   int
	Missing    int
	OtherModel int
}

// GetEmbeddingCoverage reports how much of the message history semantic search
// can currently reach. Embeddings from a different model are counted separately
// because SearchSimilarMessages only compares vectors from the active one.
func GetEmbeddingCoverage(model string) (EmbeddingCoverage, error) {
	var c EmbeddingCoverage
	err := duckdbClient.QueryRow(`
		WITH latest AS (
			SELECT id, content
			FROM latest_messages
			WHERE content <> ''
		)
		SELECT
			COUNT(*),
			COUNT(CASE WHEN e.model = ? THEN 1 END),
			COUNT(CASE WHEN e.id IS NOT NULL AND e.model <> ? THEN 1 END)
		FROM latest
		LEFT JOIN message_embeddings e ON e.id = latest.id`,
		model, model,
	).Scan(&c.Messages, &c.Embedded, &c.OtherModel)
	if err != nil {
		return c, err
	}
	c.Missing = c.Messages - c.Embedded - c.OtherModel
	return c, nil
}
