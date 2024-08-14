package database

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
	"github.com/stollenaar/statisticsbot/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	sqsClient *sqs.Client

	userCooldown map[string]time.Time
	dunceChannel string
)

func init() {
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
	userCooldown = make(map[string]time.Time)

	dC, err := util.ConfigFile.GetDunceChannel()
	if err != nil {
		log.Fatal(err)
	}
	dunceChannel = dC
}

// MessageListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageListener(session *discordgo.Session, message *discordgo.MessageCreate) {
	// if message.ChannelID == "544911814886948867" && util.Contains(message.Member.Roles, "1256012662370992219") {
	if message.Member != nil && message.ChannelID == dunceChannel {
		dunceRole, err := util.ConfigFile.GetDunceRole()
		if err == nil && util.Contains(message.Member.Roles, dunceRole) {
			go duncePunish(message)
		}
	}
	collection := client.Database("statistics_bot").Collection(message.GuildID)
	messageObject := constructMessageObject(message.Message, message.GuildID)

	bulkOption := options.BulkWriteOptions{}
	bulkOption.SetOrdered(false)

	_, err := collection.BulkWrite(context.TODO(), []mongo.WriteModel{messageObject}, &bulkOption)

	if err != nil {
		fmt.Println("Error doing bulk operation ", err)
	}
}

func duncePunish(message *discordgo.MessageCreate) {
	if lastMessage, ok := userCooldown[message.Author.ID]; !ok || time.Now().After(lastMessage.Add(17*time.Hour)) {
		userCooldown[message.Author.ID] = time.Now()
		response := util.SQSObject{
			Type:      message.Author.ID,
			Command:   "dunce",
			GuildID:   message.GuildID,
			ChannelID: message.ChannelID,
			Token: fmt.Sprintf("%s;%s",
				cmp.Or(
					message.Member.Nick,
					message.Author.Username,
				),
				message.Author.AvatarURL(""),
			),
			Data:          message.Content,
			ApplicationID: "",
		}

		data, err := json.Marshal(response)
		if err != nil {
			fmt.Printf("Error marshalling response object: %v", err)
			return
		}
		_, err = sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			MessageBody: aws.String(string(data)),
			QueueUrl:    &util.ConfigFile.SQS_REQUEST,
		})
		if err != nil {
			fmt.Printf("Error sending message to dunce queue: %v", err)
			return
		}
	}
}
