package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	reTarget *regexp.Regexp
)

func init() {
	reTarget = regexp.MustCompile(`[\<>@#&!]`)
}

func addGetUserMessages(mux *http.ServeMux) {
	mux.HandleFunc("POST /userMessages", handleGetUserMessages)
}

func handleGetUserMessages(w http.ResponseWriter, r *http.Request) {

	var sqsObject util.SQSObject

	if err := json.NewDecoder(r.Body).Decode(&sqsObject); err == nil {
		switch sqsObject.Type {
		case "user":
			resp, err := handleUserObject(sqsObject)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusOK, resp)
			}
		case "message":
			resp, err := handleMessageObject(sqsObject)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusOK, resp)
			}
		default:
			slog.Warn("Unknown type has been sent to queue", slog.Any("sqsObject", sqsObject))
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": fmt.Sprintf("Unknown type has been send to queue. sqsObject is: %v", sqsObject)})
		}
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "bad request"})
	}
}

// getUserMessages returns the latest version of every message the user sent in the guild.
func getUserMessages(guildID, userID string) ([]*util.MessageObject, error) {
	query := `
		SELECT guild_id, channel_id, id, author_id, content, date
		FROM messages
		WHERE guild_id = ? AND author_id = ? AND content IS NOT NULL
		QUALIFY ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC) = 1
		ORDER BY date;
	`

	filterResult, err := database.QueryDuckDB(query, []interface{}{guildID, userID})
	if err != nil {
		return nil, err
	}
	defer filterResult.Close()

	var messageObject []*util.MessageObject
	for filterResult.Next() {
		var guild_id, channel_id, id, author_id, content string
		var date time.Time

		if err := filterResult.Scan(&guild_id, &channel_id, &id, &author_id, &content, &date); err != nil {
			return nil, err
		}
		lastMessage := &util.MessageObject{
			GuildID:   guild_id,
			ChannelID: channel_id,
			MessageID: id,
			Author:    author_id,
			Content:   content,
			Date:      date,
		}
		messageObject = append(messageObject, lastMessage)
	}

	return messageObject, filterResult.Err()
}

func handleUserObject(sqsObject util.SQSObject) (util.UserMessagesResponse, error) {
	response := util.UserMessagesResponse{
		Type:          sqsObject.Type,
		Command:       sqsObject.Command,
		GuildID:       sqsObject.GuildID,
		Token:         sqsObject.Token,
		ApplicationID: sqsObject.ApplicationID,
	}

	messageObjects, err := getUserMessages(sqsObject.GuildID, sqsObject.Data)
	if err != nil {
		return response, err
	}

	messages := mapToContent(messageObjects)
	messages = filterNonTexts(messages)
	if messages == nil {
		// A user with no stored messages should serialise as [], not null.
		messages = []string{}
	}

	response.Data = messages
	return response, nil
}

func mapToContent(messages []*util.MessageObject) (result []string) {
	for _, message := range messages {
		result = append(result, message.Content)
	}
	return result
}

func filterNonTexts(messages []string) (result []string) {
	for _, message := range messages {
		if len(reTarget.FindAllString(message, -1)) != 3 {
			result = append(result, message)
		}
	}
	return result
}

func handleMessageObject(sqsObject util.SQSObject) ([]util.MessageObject, error) {
	return database.GetMessageBlock(sqsObject.Command)
}
