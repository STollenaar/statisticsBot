package routes

import (
	"context"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/util"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetUserMessages(guildID, userID string) []util.MessageObject {
	filter := bson.M{
		"Author": userID,
		"Content.10": bson.D{
			primitive.E{
				Key:   "$exists",
				Value: true,
			},
		},
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{
		primitive.E{
			Key:   "Date",
			Value: -1,
		},
	})
	findOptions.SetLimit(1000)

	var messageObject []util.MessageObject

	resultCursor, err := database.GetFromFilter(guildID, filter, findOptions)
	if err != nil {
		panic(err)
	}

	err = resultCursor.All(context.TODO(), &messageObject)
	if err != nil {
		panic(err)
	}
	return messageObject
}
