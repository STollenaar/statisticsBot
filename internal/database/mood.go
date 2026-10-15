package database

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/stollenaar/statisticsbot/internal/embeddings"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// SaveMessageMood upserts the emotion classification for a message.
func SaveMessageMood(id, model, label string, score float32, scores []float32) error {
	// The distribution is inlined as a numeric list literal; the driver does not
	// bind Go slices as DuckDB lists.
	query := fmt.Sprintf(`
		INSERT INTO message_moods (id, model, label, score, scores)
		VALUES (?, ?, ?, ?, %s)
		ON CONFLICT (id) DO UPDATE SET
			model = EXCLUDED.model,
			label = EXCLUDED.label,
			score = EXCLUDED.score,
			scores = EXCLUDED.scores`, floatSliceToList(scores))
	_, err := duckdbClient.Exec(query, id, model, label, score)
	return err
}

// MoodMessage classifies and stores the mood of a single message. Empty content
// is skipped. Failures are logged here so fire-and-forget ingestion callers stay
// one-liners, and also returned so the backfill can count them.
func MoodMessage(id, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	mood, err := embeddings.ScoreMood(content)
	if err != nil {
		slog.Error("failed to score message mood", slog.String("id", id), slog.Any("err", err))
		return err
	}

	if err := SaveMessageMood(id, embeddings.MoodModelName(), mood.Label, mood.Score, mood.Scores); err != nil {
		slog.Error("failed to store message mood", slog.String("id", id), slog.Any("err", err))
		return err
	}
	return nil
}

// CountMessagesWithoutMoods counts the messages still to be classified, so a
// backfill can report a total without holding them all in memory.
func CountMessagesWithoutMoods() (int, error) {
	var count int
	err := duckdbClient.QueryRow(`
		SELECT COUNT(*)
		FROM latest_messages m
		LEFT JOIN message_moods mo ON mo.id = m.id
		WHERE mo.id IS NULL AND m.content <> ''`).Scan(&count)
	return count, err
}

// GetMessagesWithoutMoods returns up to limit messages that have not been
// classified, ordered by id and starting after afterID. Pass an empty afterID
// for the first chunk and the last id of each chunk for the next.
//
// Paginating on the id rather than taking the whole set is what keeps a backfill
// of a million messages from loading a million rows up front. A cursor rather
// than LIMIT alone is deliberate: a message that fails to classify keeps
// matching this query, so re-running it from the start would hand back the same
// failures forever. Advancing past the last id seen cannot loop.
//
// The ordering is lexicographic on the VARCHAR id, which is not numeric order
// because ids differ in length. That does not matter — it only has to be a
// stable total order, and the comparison below uses the same one.
func GetMessagesWithoutMoods(afterID string, limit int) ([]util.MessageObject, error) {
	rows, err := duckdbClient.Query(fmt.Sprintf(`
		SELECT m.id, m.guild_id, m.channel_id, m.author_id, m.content, m.date
		FROM latest_messages m
		LEFT JOIN message_moods mo ON mo.id = m.id
		WHERE mo.id IS NULL AND m.content <> '' AND m.id > ?
		ORDER BY m.id
		LIMIT %d`, limit), afterID)
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

// MoodCoverage counts how many messages could be classified and how many are.
type MoodCoverage struct {
	Messages   int
	Scored     int
	Missing    int
	OtherModel int
}

// GetMoodCoverage reports how much of the message history has a usable mood
// classification. Rows scored by a different model are counted separately
// because their label set and vector order do not match the active one.
func GetMoodCoverage(model string) (MoodCoverage, error) {
	var c MoodCoverage
	err := duckdbClient.QueryRow(`
		WITH latest AS (
			SELECT id FROM latest_messages WHERE content <> ''
		)
		SELECT
			COUNT(*),
			COUNT(CASE WHEN mo.model = ? THEN 1 END),
			COUNT(CASE WHEN mo.id IS NOT NULL AND mo.model <> ? THEN 1 END)
		FROM latest
		LEFT JOIN message_moods mo ON mo.id = latest.id`,
		model, model,
	).Scan(&c.Messages, &c.Scored, &c.OtherModel)
	if err != nil {
		return c, err
	}
	c.Missing = c.Messages - c.Scored - c.OtherModel
	return c, nil
}
