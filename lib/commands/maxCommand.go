package commands

import (
	"fmt"
	"regexp"
	"statsisticsbot/util"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaxCommand counts the amount of occurences of a certain word
func MaxCommand(message *discordgo.MessageCreate, args commandArgs) {
	Bot.ChannelTyping(message.ChannelID)

	parsedArguments := parseArguments(message.Message, args)
	maxWord := FindAllWordOccurences(parsedArguments)

	if maxWord.Author != message.Author.ID {
		user, _ := Bot.User(maxWord.Author)
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", user.Mention(), maxWord.Word.Word, maxWord.Word.Count))
	} else {
		Bot.ChannelMessageSend(message.ChannelID, fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count))
	}
}

type elemFilter struct {
	array      string
	expression bson.D
}

// FindAllWordOccurences finding the occurences of a word in the database
func FindAllWordOccurences(args CommandParsed) util.CountGrouped {
	filter := getFilter(args.Word)

	messageObject := CountFilterOccurences(args.GuildID, filter)
	if len(messageObject) == 1 {
		return messageObject[0]
	} else if len(messageObject) > 1 {
		index := util.FindMaxIndexElement(messageObject)
		return messageObject[index]
	} else {
		return util.CountGrouped{}
	}
}

func getFilter(kind string) (result bson.D) {
	re := regexp.MustCompile("[\\<>@#&!]")
	id := re.ReplaceAllString(kind, "")
	if kind != "" {
		if strings.Contains(kind, "@") {
			// Filtering based on author
			result = bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key:   "Author",
							Value: id,
						},
					},
				},
			}
		} else if strings.Contains(kind, "#") {
			// Filtering based on channelID
			result = bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key:   "ChannelID",
							Value: id,
						},
					},
				},
			}
		} else {
			result = bson.D{
				// Filtering based on word
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key: "Content",
							Value: bson.D{
								primitive.E{
									Key:   "$in",
									Value: []string{kind},
								},
							},
						},
					},
				},
			}
		}
	}
	return result
}
