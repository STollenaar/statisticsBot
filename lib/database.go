package lib

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"statsisticsbot/util"
	"sync"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Bot main reference to the bot
var Bot *discordgo.Session
var re *regexp.Regexp
var client *mongo.Client

// getClient gets the mongo client on the first load
func getClient() {
	c, err := mongo.NewClient(options.Client().ApplyURI("mongodb://" + os.Getenv("DATABASE_HOST") + ":27017"))
	client = c
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

}

// Init doing the initialization of all the messages
func Init() {
	getClient()
	re = regexp.MustCompile("\\s|\\.|\\\"")
	guilds, err := Bot.UserGuilds(100, "", "")
	if err != nil {
		fmt.Println(err)
	}
	var waitGroup sync.WaitGroup
	for _, guild := range guilds {
		channels, err := Bot.GuildChannels(guild.ID)
		if err != nil {
			fmt.Println("Error loading channels ", err)
			return
		}

		// Async checking the channels of guild for new messages
		waitGroup.Add(1)
		go func(channels []*discordgo.Channel, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			initChannels(channels, waitGroup)
		}(channels, &waitGroup)
	}

	// Waiting for all async calls to complete
	waitGroup.Wait()
	fmt.Println("Done loading guilds")
}

// initChannels loading all the channels of the guild
func initChannels(channels []*discordgo.Channel, waitGroup *sync.WaitGroup) {
	for _, channel := range channels {
		fmt.Printf("Checking %s \n", channel.Name)
		// Check if channel is a guild text channel and not a voice or DM channel
		if channel.Type != discordgo.ChannelTypeGuildText {
			continue
		}

		// Async loading of the messages in that channnel
		waitGroup.Add(1)
		go func(channel *discordgo.Channel) {
			defer waitGroup.Done()
			loadMessages(channel)
		}(channel)
	}
}

// getLastMessage gets the last message in provided channel from the database
func getLastMessage(channel *discordgo.Channel) util.MessageObject {

	collection := client.Database("statistics_bot").Collection(channel.GuildID)
	findOpts := options.FindOneOptions{
		Sort: bson.D{
			primitive.E{
				Key:   "Date",
				Value: -1,
			},
		},
	}

	var lastMessage util.MessageObject

	if err := collection.FindOne(context.TODO(), bson.M{"ChannelID": channel.ID}, &findOpts).Decode(&lastMessage); err != nil {
		fmt.Println("Error fetching last message: ", err)
	}
	return lastMessage
}

// loadMessages loading messages from the channel
func loadMessages(channel *discordgo.Channel) {
	fmt.Println("Loading ", channel.Name)
	defer util.Elapsed(channel.Name)() // timing how long it took to collect the messages
	collection := client.Database("statistics_bot").Collection(channel.GuildID)
	var operations []mongo.WriteModel

	// Getting last message and first 100
	lastMessage := getLastMessage(channel)
	messages, _ := Bot.ChannelMessages(channel.ID, int(100), "", "", "")
	messages = util.FilterDiscordMessages(messages, func(message *discordgo.Message) bool {
		messageTime, _ := message.Timestamp.Parse()
		lastMessageTime, _ := lastMessage.Date.Parse()
		return messageTime.After(lastMessageTime)
	})

	// Constructing operations for first 100
	for _, message := range messages {
		operations = append(operations, constructMessageObject(message, channel.GuildID))
	}

	// Loading more messages if got 100 message the first time
	if len(messages) == 100 {
		lastMessageCollected := messages[len(messages)-1]
		// Loading more messages, 100 at a time
		for lastMessageCollected != nil {
			moreMes, _ := Bot.ChannelMessages(channel.ID, int(100), lastMessageCollected.ID, "", "")
			moreMes = util.FilterDiscordMessages(moreMes, func(message *discordgo.Message) bool {
				messageTime, _ := message.Timestamp.Parse()
				lastMessageTime, _ := lastMessage.Date.Parse()
				return messageTime.After(lastMessageTime)
			})

			for _, message := range moreMes {
				operations = append(operations, constructMessageObject(message, channel.GuildID))
			}
			if len(moreMes) != 0 {
				lastMessageCollected = moreMes[len(moreMes)-1]
			} else {
				break
			}
		}
	}

	fmt.Printf("Done collecting messages for %s, found %d messages. Now inserting \n", channel.Name, len(operations))

	// Doing actual insertion
	if len(operations) > 0 {
		bulkOption := options.BulkWriteOptions{}
		bulkOption.SetOrdered(false)

		_, err := collection.BulkWrite(context.TODO(), operations, &bulkOption)

		if err != nil {
			fmt.Println("Error doing bulk operation ", err)
		}
	}
}

// constructing the message object from the received discord message, ready for inserting into database
func constructMessageObject(message *discordgo.Message, guildID string) mongo.WriteModel {
	messageModel := mongo.NewUpdateOneModel()

	content := re.Split(message.Content, -1)
	filter := bson.D{
		primitive.E{
			Key:   "_id",
			Value: message.ID,
		},
	}

	// Actual message object
	object := bson.D{
		primitive.E{
			Key: "$set",
			Value: util.MessageObject{
				GuildID:   guildID,
				ChannelID: message.ChannelID,
				MessageID: message.ID,
				Author:    message.Author.ID,
				Content:   util.DeleteEmpty(content),
				Date:      message.Timestamp,
			},
		},
	}

	messageModel.SetFilter(filter)
	messageModel.SetUpdate(object)
	messageModel.SetUpsert(true)

	return messageModel
}
