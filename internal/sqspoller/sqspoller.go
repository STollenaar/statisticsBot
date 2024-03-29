package sqspoller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
	"github.com/stollenaar/statisticsbot/internal/routes"
	"github.com/stollenaar/statisticsbot/util"
)

var (
	sqsClient        *sqs.Client
	sqsObjectChannel chan util.SQSObject

	reTarget      *regexp.Regexp
	expiredBuffer chan string
	expiredHold   bool
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
	sqsObjectChannel = make(chan util.SQSObject)
	expiredBuffer = make(chan string)

	go pollSQS()
	go pollExpiredBuffer()
}

func pollExpiredBuffer() {
	for {
		buff := <-expiredBuffer
		fmt.Println(buff)
		time.Sleep(time.Minute * 1)
		expiredHold = false
	}
}

func PollSQS() {
	for {
		sqsObject := <-sqsObjectChannel
		fmt.Printf("Handling object %v\n", sqsObject)
		switch sqsObject.Type {
		case "url":
			handleURLObject(sqsObject)
		case "user":
			handleUserObject(sqsObject)
		default:
			fmt.Printf("Unknown type has been send to queue. sqsObject is: %v", sqsObject)
		}
	}

}

func pollSQS() {

	for {
		msgResult, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			MessageAttributeNames: []string{
				string(types.QueueAttributeNameAll),
			},
			QueueUrl:            &util.ConfigFile.SQS_REQUEST,
			MaxNumberOfMessages: 1,
			VisibilityTimeout:   int32(5),
			WaitTimeSeconds:     20,
			AttributeNames:      []types.QueueAttributeName{types.QueueAttributeName(types.MessageSystemAttributeNameSentTimestamp)},
		})
		if err != nil {
			if !strings.Contains(err.Error(), "ExpiredToken") {
				fmt.Printf("Got an error receiving messages: %s\n", err)
			} else if !expiredHold {
				expiredHold = true
				expiredBuffer <- err.Error()
			}
		}
		if msgResult == nil {
			continue
		}
		for _, message := range msgResult.Messages {
			_, err = sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
				QueueUrl:      &util.ConfigFile.SQS_REQUEST,
				ReceiptHandle: message.ReceiptHandle,
			})
			if err != nil {
				fmt.Println(err)
			}

			var object util.SQSObject
			err = json.Unmarshal([]byte(*message.Body), &object)

			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Message received %v\n", object)
			sqsObjectChannel <- object
		}
	}
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

	messageObjects := routes.GetUserMessages(sqsObject.GuildID, sqsObject.Data)

	messages := mapToContent(&messageObjects)
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

func mapToContent(messages *[]util.MessageObject) (result []string) {
	for _, message := range *messages {
		if len(message.Content) == 0 {
			continue
		}
		lastWord := message.Content[len(message.Content)-1]
		if !isTerminalWord(lastWord) {
			lastWord += "."
			message.Content[len(message.Content)-1] = lastWord
		}
		result = append(result, message.Content...)
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

func isTerminalWord(word string) bool {
	compiled, err := regexp.MatchString(util.ConfigFile.TERMINAL_REGEX, word)
	if err != nil {
		fmt.Println(fmt.Errorf("error matching string with regex %s, on word %s. %w", util.ConfigFile.TERMINAL_REGEX, word, err))
	}
	return compiled
}
