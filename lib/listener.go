package lib

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageListener registers a simpler handler on a discordgo session to automatically parse incoming messages for you.
func MessageListener(session *discordgo.Session, message *discordgo.MessageCreate) {
	collection := client.Database("statistics_bot").Collection(message.GuildID)
	messageObject := constructMessageObject(message.Message, message.GuildID)

	bulkOption := options.BulkWriteOptions{}
	bulkOption.SetOrdered(false)

	_, err := collection.BulkWrite(context.TODO(), []mongo.WriteModel{messageObject}, &bulkOption)

	if err != nil {
		fmt.Println("Error doing bulk operation ", err)
	}
}
