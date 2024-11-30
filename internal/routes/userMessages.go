package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/util"
)

var (
	sqsClient *sqs.Client

	reTarget *regexp.Regexp
)

func init() {
	reTarget = regexp.MustCompile(`[\<>@#&!]`)

	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		provider := filecreds.NewFilecredentialsProvider(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"))
		sqsClient = sqs.New(sqs.Options{
			Credentials: provider,
			Region:      os.Getenv("AWS_REGION"),
		})
	} else {
		// Create a config with the credentials provider.
		cfg, err := config.LoadDefaultConfig(context.TODO())

		if err != nil {
			panic("configuration error, " + err.Error())
		}

		sqsClient = sqs.NewFromConfig(cfg)
	}
}

func addGetUserMessages(r *gin.Engine) {
	r.POST("/userMessages", handleGetUserMessages)
}

func handleGetUserMessages(c *gin.Context) {

	var json struct {
		Value util.SQSObject
	}

	if c.Bind(&json) == nil {
		switch json.Value.Type {
		case "url":
			handleURLObject(json.Value)
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		case "user":
			handleUserObject(json.Value)
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		default:
			fmt.Printf("Unknown type has been send to queue. sqsObject is: %v", json.Value)
			c.JSON(http.StatusBadRequest, gin.H{"status": fmt.Sprintf("Unknown type has been send to queue. sqsObject is: %v", json.Value)})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "bad request"})
	}
}

func getUserMessages(guildID, userID string) (messageObject []*util.MessageObject) {

	filterResult, err := database.GetFromFilter("guild_id==? AND author_id==?", []interface{}{guildID, userID})
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

func handleURLObject(sqsObject util.SQSObject) {

	data, err := json.Marshal(sqsObject)
	if err != nil {
		fmt.Printf("Error marshalling response object: %v", err)
		return
	}
	_, err = sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(data)),
		QueueUrl:    &util.ConfigFile.SQS_RESPONSE,
	})
	if err != nil {
		fmt.Printf("Error sending message response object: %v", err)
		return
	}
}

func handleUserObject(sqsObject util.SQSObject) {
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

	data, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("Error marshalling response object: %v", err)
		return
	}
	_, err = sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(data)),
		QueueUrl:    &util.ConfigFile.SQS_RESPONSE,
	})
	if err != nil {
		fmt.Printf("Error sending message response object: %v", err)
		return
	}
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
