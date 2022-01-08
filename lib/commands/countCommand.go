package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CountCommand counts the amount of occurences of a certain word
func CountCommand(message *discordgo.MessageCreate, args commandArgs) {
	Bot.ChannelTyping(message.ChannelID)

	parsedArguments := parseArguments(message.Message, args)
	amount := FindSpecificWordOccurences(parsedArguments)

	if parsedArguments.UserTarget != message.Author.ID {
		user, _ := Bot.User(parsedArguments.UserTarget)
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", user.Mention(), parsedArguments.Word, amount))
	} else {
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", parsedArguments.Word, amount))
	}
}

// FindSpecificWordOccurences finding the occurences of a word in the database
func FindSpecificWordOccurences(args CommandParsed) int {

	var channelFilter bson.D

	if args.ChannelTarget != "" {
		channelFilter = bson.D{
			primitive.E{
				Key:   "$eq",
				Value: args.ChannelTarget,
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

	filter := bson.D{
		primitive.E{
			Key: "$match",
			Value: bson.M{
				"Content": bson.D{
					primitive.E{
						Key:   "$in",
						Value: []string{args.Word},
					},
				},
				"Author":    args.UserTarget,
				"ChannelID": channelFilter,
			},
		},
	}

	messages := CountFilterOccurences(args.GuildID, filter)

	if len(messages) == 0 {
		return 0
	}
	return messages[0].Word.Count
}
