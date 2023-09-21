package sqspoller

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stollenaar/statisticsbot/interal/routes"
	"github.com/stollenaar/statisticsbot/util"
)

var (
	sqsClient        *sqs.Client
	sqsObjectChannel chan util.SQSObject

	sqsRequestURL  *string
	sqsResponseURL *string
	reTarget       *regexp.Regexp
)

func init() {
	reTarget = regexp.MustCompile(`[\<>@#&!]`)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	sqsClient = sqs.NewFromConfig(cfg)
	sqsObjectChannel = make(chan util.SQSObject)

	// Get URL of request queue
	urlResult, err := sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: &util.ConfigFile.SQS_REQUEST,
	})
	if err != nil {
		fmt.Println("Got an error getting the queue URL:")
		panic(err)
	}

	sqsRequestURL = urlResult.QueueUrl

	// Get URL of response queue
	urlResult, err = sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: &util.ConfigFile.SQS_RESPONSE,
	})
	if err != nil {
		fmt.Println("Got an error getting the queue URL:")
		panic(err)
	}

	sqsResponseURL = urlResult.QueueUrl

	go pollSQS(sqsObjectChannel)
}

func PollSQS() {
	for sqsObject := range sqsObjectChannel {
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

func pollSQS(chl chan<- util.SQSObject) {
	for {
		msgResult, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			MessageAttributeNames: []string{
				string(types.QueueAttributeNameAll),
			},
			QueueUrl:            sqsRequestURL,
			MaxNumberOfMessages: 1,
			VisibilityTimeout:   int32(5),
		})
		if err != nil {
			fmt.Println("Got an error receiving messages:")
			fmt.Println(err)
		}

		for _, message := range msgResult.Messages {
			var object util.SQSObject
			json.Unmarshal([]byte(*message.Body), &object)
			chl <- object
		}
	}
}

func handleURLObject(sqsObject util.SQSObject) {

	data, err := json.Marshal(sqsObject)
	if err != nil {
		fmt.Printf("Error marshalling response object: %v", err)
		return
	}
	sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(data)),
		QueueUrl:    sqsResponseURL,
	})
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
	sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(data)),
		QueueUrl:    sqsResponseURL,
	})
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