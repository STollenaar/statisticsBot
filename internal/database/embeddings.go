package database

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/stollenaar/statisticsbot/internal/embeddings"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// SemanticSearchResult is a single message returned from a similarity search,
// ordered by descending cosine similarity Score.
type SemanticSearchResult struct {
	MessageID string
	ChannelID string
	AuthorID  string
	Content   string
	Date      time.Time
	Score     float64
}

// floatSliceToList renders a vector as a DuckDB list literal (e.g. "[0.1,0.2]").
// The values are numeric and formatted by us, so this is safe to inline in SQL.
func floatSliceToList(vec []float32) string {
	var b strings.Builder
	b.Grow(len(vec) * 8)
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// SaveMessageEmbedding upserts the embedding vector for a message. Only the
// vector is stored; content and metadata live in the messages table and are
// joined back via the message id.
func SaveMessageEmbedding(id, model string, vec []float32) error {
	// The embedding is inlined as a numeric list literal; the driver does not
	// bind Go slices as DuckDB lists.
	query := fmt.Sprintf(`
		INSERT INTO message_embeddings (id, model, embedding)
		VALUES (?, ?, %s)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			model = EXCLUDED.model`, floatSliceToList(vec))
	_, err := duckdbClient.Exec(query, id, model)
	return err
}

// SearchSimilarMessages returns the messages in a guild most similar to the
// given query vector, ordered by descending cosine similarity. Content and
// metadata are read from the latest version of each message. Only embeddings
// produced by the given model are compared, so a model change (with a different
// vector dimension) does not break the similarity function.
func SearchSimilarMessages(guildID string, vec []float32, model string, limit int) ([]SemanticSearchResult, error) {
	query := fmt.Sprintf(`
		SELECT m.id, m.channel_id, COALESCE(m.author_id, ''), m.content, m.date,
		       list_cosine_similarity(e.embedding, %s::FLOAT[]) AS score
		FROM message_embeddings e
		JOIN (
			SELECT m.id, m.guild_id, m.channel_id, m.author_id, m.content, m.date
			FROM messages m
			JOIN (
				SELECT id, MAX(version) AS latest_version
				FROM messages
				GROUP BY id
			) latest ON m.id = latest.id AND m.version = latest.latest_version
			WHERE m.guild_id = ?
		) m ON m.id = e.id
		WHERE e.model = ?
		ORDER BY score DESC
		LIMIT ?`, floatSliceToList(vec))

	rows, err := duckdbClient.Query(query, guildID, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SemanticSearchResult
	for rows.Next() {
		var r SemanticSearchResult
		if err := rows.Scan(&r.MessageID, &r.ChannelID, &r.AuthorID, &r.Content, &r.Date, &r.Score); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetMessagesWithoutEmbeddings returns the latest version of every stored
// message that does not yet have an embedding. A limit <= 0 means no limit.
func GetMessagesWithoutEmbeddings(limit int) ([]util.MessageObject, error) {
	query := `
		SELECT m.id, m.guild_id, m.channel_id, m.author_id, m.content, m.date
		FROM messages m
		JOIN (
			SELECT id, MAX(version) AS latest_version
			FROM messages
			GROUP BY id
		) latest ON m.id = latest.id AND m.version = latest.latest_version
		LEFT JOIN message_embeddings e ON e.id = m.id
		WHERE e.id IS NULL AND m.content <> ''`
	if limit > 0 {
		query += fmt.Sprintf("\n\t\tLIMIT %d", limit)
	}

	rows, err := duckdbClient.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []util.MessageObject
	for rows.Next() {
		var m util.MessageObject
		if err := rows.Scan(&m.MessageID, &m.GuildID, &m.ChannelID, &m.Author, &m.Content, &m.Date); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// EmbedMessage generates and stores an embedding for a single message. It is
// best-effort: empty content is skipped and failures are logged, not returned,
// so it can be fired off from ingestion and backfill paths.
func EmbedMessage(id, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	vec, err := embeddings.Embed(content)
	if err != nil {
		slog.Error("failed to embed message", slog.String("id", id), slog.Any("err", err))
		return
	}

	if err := SaveMessageEmbedding(id, embeddings.ModelName(), vec); err != nil {
		slog.Error("failed to store message embedding", slog.String("id", id), slog.Any("err", err))
	}
}
