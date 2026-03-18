package database

import (
	"time"
)

type SummaryInvocation struct {
	ID           string
	GuildID      string
	ChannelID    string
	Unit         string
	RequestedAt  time.Time
	MessagesJSON string
	RawResponse  string
	Status       string
}

func SaveSummaryInvocation(id, guildID, channelID, unit, messagesJSON string) error {
	_, err := duckdbClient.Exec(
		`INSERT INTO summary_invocations (id, guild_id, channel_id, unit, requested_at, messages_json, status)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
		id, guildID, channelID, unit, time.Now(), messagesJSON,
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

func CountSummaryInvocations() (int, error) {
	var count int
	err := duckdbClient.QueryRow(`SELECT COUNT(*) FROM summary_invocations`).Scan(&count)
	return count, err
}

func ListSummaryInvocations(page, pageSize int) ([]SummaryInvocation, error) {
	offset := (page - 1) * pageSize
	rows, err := duckdbClient.Query(`
		SELECT id, guild_id, channel_id, unit, requested_at, messages_json, COALESCE(raw_response, ''), status
		FROM summary_invocations
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
		var inv SummaryInvocation
		err := rows.Scan(&inv.ID, &inv.GuildID, &inv.ChannelID, &inv.Unit, &inv.RequestedAt, &inv.MessagesJSON, &inv.RawResponse, &inv.Status)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, nil
}

func GetSummaryInvocation(id string) (SummaryInvocation, error) {
	var inv SummaryInvocation
	err := duckdbClient.QueryRow(`
		SELECT id, guild_id, channel_id, unit, requested_at, messages_json, COALESCE(raw_response, ''), status
		FROM summary_invocations WHERE id = ?`,
		id,
	).Scan(&inv.ID, &inv.GuildID, &inv.ChannelID, &inv.Unit, &inv.RequestedAt, &inv.MessagesJSON, &inv.RawResponse, &inv.Status)
	return inv, err
}

func DeleteSummaryInvocation(id string) error {
	_, err := duckdbClient.Exec(`DELETE FROM summary_invocations WHERE id = ?`, id)
	return err
}
