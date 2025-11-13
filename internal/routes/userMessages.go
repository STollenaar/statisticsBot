package routes

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	reTarget *regexp.Regexp
)

func init() {
	reTarget = regexp.MustCompile(`[\<>@#&!]`)
}

func addGetUserMessages(r *gin.Engine) {
	r.POST("/userMessages", handleGetUserMessages)
}

func handleGetUserMessages(c *gin.Context) {

	var json util.SQSObject

	if c.BindJSON(&json) == nil {
		switch json.Type {
		case "user":
			resp := handleUserObject(json)
			c.JSON(http.StatusOK, resp)
		case "message":
			resp, err := handleMessageObject(json)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusOK, resp)
			}
		default:
			fmt.Printf("Unknown type has been send to queue. sqsObject is: %v", json)
			c.JSON(http.StatusBadRequest, gin.H{"status": fmt.Sprintf("Unknown type has been send to queue. sqsObject is: %v", json)})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "bad request"})
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
