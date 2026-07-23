package database

import (
	"time"
)

type SummaryInvocation struct {
	ID           string
	ParentID     string
	GuildID      string
	ChannelID    string
	MessageID    string
	Unit         string
	RequestedAt  time.Time
	MessagesJSON string
	RawResponse  string
	Status       string
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

const summaryInvocationColumns = `id, COALESCE(parent_id, ''), guild_id, channel_id, COALESCE(message_id, ''), unit, requested_at, messages_json, COALESCE(raw_response, ''), status`

func scanSummaryInvocation(s rowScanner) (SummaryInvocation, error) {
	var inv SummaryInvocation
	err := s.Scan(&inv.ID, &inv.ParentID, &inv.GuildID, &inv.ChannelID, &inv.MessageID, &inv.Unit, &inv.RequestedAt, &inv.MessagesJSON, &inv.RawResponse, &inv.Status)
	return inv, err
}

func SaveSummaryInvocation(id, guildID, channelID, unit, messagesJSON string) error {
	_, err := duckdbClient.Exec(
		`INSERT INTO summary_invocations (id, guild_id, channel_id, unit, requested_at, messages_json, status)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
		id, guildID, channelID, unit, time.Now(), messagesJSON,
	)
	return err
}

// SaveSummaryRetry stores a retry as a new invocation linked to the original
// attempt through parentID, leaving the original row untouched so previous
// tries are preserved.
func SaveSummaryRetry(id, parentID, guildID, channelID, unit, messagesJSON, rawResponse, status string) error {
	_, err := duckdbClient.Exec(
		`INSERT INTO summary_invocations (id, parent_id, guild_id, channel_id, unit, requested_at, messages_json, raw_response, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, parentID, guildID, channelID, unit, time.Now(), messagesJSON, rawResponse, status,
	)
	return err
}

func UpdateSummaryInvocation(id, rawResponse, status string) error {
	_, err := duckdbClient.Exec(
		`UPDATE summary_invocations SET raw_response = ?, status = ? WHERE id = ?`,
		rawResponse, status, id,
	)
	return err
}

// SetSummaryInvocationMessage records the Discord message the summary was
// delivered in, so it can be linked back to later.
func SetSummaryInvocationMessage(id, messageID string) error {
	_, err := duckdbClient.Exec(
		`UPDATE summary_invocations SET message_id = ? WHERE id = ?`,
		messageID, id,
	)
	return err
}

// CountSummaryInvocations counts only original attempts; retries are grouped
// beneath their parent and are not paginated on their own.
func CountSummaryInvocations() (int, error) {
	var count int
	err := duckdbClient.QueryRow(`SELECT COUNT(*) FROM summary_invocations WHERE parent_id IS NULL`).Scan(&count)
	return count, err
}

func ListSummaryInvocations(page, pageSize int) ([]SummaryInvocation, error) {
	offset := (page - 1) * pageSize
	rows, err := duckdbClient.Query(`
		SELECT `+summaryInvocationColumns+`
		FROM summary_invocations
		WHERE parent_id IS NULL
		ORDER BY requested_at DESC
		LIMIT ? OFFSET ?`,
		pageSize, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SummaryInvocation
	for rows.Next() {
		inv, err := scanSummaryInvocation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, nil
}

// ListSummaryRetries returns every retry linked to parentID, oldest first.
func ListSummaryRetries(parentID string) ([]SummaryInvocation, error) {
	rows, err := duckdbClient.Query(`
		SELECT `+summaryInvocationColumns+`
		FROM summary_invocations
		WHERE parent_id = ?
		ORDER BY requested_at ASC`,
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SummaryInvocation
	for rows.Next() {
		inv, err := scanSummaryInvocation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, nil
}

func GetSummaryInvocation(id string) (SummaryInvocation, error) {
	return scanSummaryInvocation(duckdbClient.QueryRow(`
		SELECT `+summaryInvocationColumns+`
		FROM summary_invocations WHERE id = ?`,
		id,
	))
}

func DeleteSummaryInvocation(id string) error {
	_, err := duckdbClient.Exec(`DELETE FROM summary_invocations WHERE id = ?`, id)
	return err
}
