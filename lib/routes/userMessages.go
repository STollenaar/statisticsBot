package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/stollenaar/statisticsbot/lib"
	"github.com/stollenaar/statisticsbot/util"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetUserMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	guildID := vars["guildID"]
	userID := vars["userID"]

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

	resultCursor, err := lib.GetFromFilter(guildID, filter, findOptions)
	if err != nil {
		panic(err)
	}

	err = resultCursor.All(context.TODO(), &messageObject)
	if err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(messageObject)
}
