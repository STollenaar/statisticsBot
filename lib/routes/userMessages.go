package routes

import (
	"encoding/json"
	"net/http"
	"statsisticsbot/lib"
	"statsisticsbot/util"

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
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{
		primitive.E{
			Key:   "Date",
			Value: -1,
		},
	})
	findOptions.SetLimit(500)

	var messageObject []util.MessageObject

	resultCursor, err := lib.GetFromFilter(guildID, filter, findOptions)
	if err != nil {
		panic(err)
	}

	err = resultCursor.Decode(&messageObject)
	if err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(messageObject)
}
