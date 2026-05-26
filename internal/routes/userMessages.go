package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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

	if err := json.NewDecoder(r.Body).Decode(&sqsObject); err == nil{
		switch sqsObject.Type {
		case "user":
			resp := handleUserObject(sqsObject)
			writeJSON(w, http.StatusOK, resp)
		case "message":
			resp, err := handleMessageObject(sqsObject)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusOK, resp)
			}
		default:
			fmt.Printf("Unknown type has been send to queue. sqsObject is: %v", sqsObject)
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": fmt.Sprintf("Unknown type has been send to queue. sqsObject is: %v", sqsObject)})
		}
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "bad request"})
	}
}

func getUserMessages(guildID, userID string) (messageObject []*util.MessageObject) {

	filterResult, err := database.QueryDuckDB("guild_id==? AND author_id==?", []interface{}{guildID, userID})
	if err != nil {
		panic(err)
	}

	for filterResult.Next() {
		var guild_id, channel_id, id, author_id, content string
		var date time.Time

		err = filterResult.Scan(&guild_id, &channel_id, &id, &author_id, &content, &date)
		if err != nil {
			break
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

	return
}

func handleUserObject(sqsObject util.SQSObject) util.SQSObject {
	response := util.SQSObject{
		Type:          sqsObject.Type,
		Command:       sqsObject.Command,
		GuildID:       sqsObject.GuildID,
		Token:         sqsObject.Token,
		ApplicationID: sqsObject.ApplicationID,
	}

	messageObjects := getUserMessages(sqsObject.GuildID, sqsObject.Data)

	messages := mapToContent(messageObjects)
	messages = filterNonTexts(messages)

	response.Data = strings.Join(messages, " ")
	return response
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
