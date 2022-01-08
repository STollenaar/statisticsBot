package commands

import (
	"fmt"
	"regexp"
	"statsisticsbot/lib"
	"statsisticsbot/util"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LastMessage find the last message of a person
func LastMessage(message *discordgo.MessageCreate, args commandArgs) {
	if !strings.Contains(args.Word, "<@!") {
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Not specifying a user, please use a user as the reference"))
		return
	}
	re := regexp.MustCompile("[\\<>@#&!]")
	author := re.ReplaceAllString(args.Word, "")
	commandParsed := parseArguments(message.Message, args)

	var channelFilter bson.D

	if commandParsed.ChannelTarget != "" {
		channelFilter = bson.D{
			primitive.E{
				Key:   "$eq",
				Value: commandParsed.ChannelTarget,
			},
		}
	} else {
		channelFilter = bson.D{
			primitive.E{
				Key:   "$exists",
				Value: true,
			},
		}
	}

	filter := bson.M{
		"Author":    author,
		"ChannelID": channelFilter,
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{
		primitive.E{
			Key:   "Date",
			Value: -1,
		},
	})
	findOptions.SetLimit(1)

	var messageObject util.MessageObject

	filterResult, err := lib.GetFromFilter(message.GuildID, filter, findOptions)
	if err != nil {
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintln("Something went wrong.. maybe try again with something else?"))
		return
	}

	err = filterResult.Decode(&messageObject)

	if err != nil {
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintln("Something went wrong.. maybe try again with something else?"))
		return
	}

	channel, _ := Bot.Channel(messageObject.ChannelID)
	user, _ := Bot.User(messageObject.Author)
	Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s last has send something in %s and said \"%s\"", user.Mention(), channel.Mention(), strings.Join(messageObject.Content, " ")))
}
