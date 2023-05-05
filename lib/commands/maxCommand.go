package commands

import (
	"fmt"
	"statsisticsbot/util"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaxCommand counts the amount of occurences of a certain word
func MaxCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := parseArguments(bot, interaction)
	maxWord := FindAllWordOccurences(parsedArguments)

	var response string
	if parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID {
		response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", parsedArguments.UserTarget.Mention(), maxWord.Word.Word, maxWord.Word.Count)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)
	}

	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: response,
	})
}

// FindAllWordOccurences finding the occurences of a word in the database
func FindAllWordOccurences(arguments *CommandParsed) util.CountGrouped {
	filter, wordFilter := getFilter(arguments)

	messageObject := CountFilterOccurences(arguments.GuildID, filter, wordFilter)
	if len(messageObject) == 1 {
		return messageObject[0]
	} else if len(messageObject) > 1 {
		index := util.FindMaxIndexElement(messageObject)
		return messageObject[index]
	} else {
		return util.CountGrouped{}
	}
}

func getFilter(arguments *CommandParsed) (result bson.D, wordFilter string) {
	if arguments.isNotEmpty() {

		if user := arguments.UserTarget; user != nil {
			// Filtering based on author
			result = append(result,
				bson.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key:   "Author",
							Value: user.ID,
						},
					},
				})
		}
		if channel := arguments.ChannelTarget; channel != nil {
			// Filtering based on channelID
			result = append(result,
				bson.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key:   "ChannelID",
							Value: channel.ID,
						},
					},
				})
		}

		if word := arguments.Word; word != "" {
			wordFilter = word
			result = append(result,
				bson.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key: "Content",
							Value: bson.D{
								primitive.E{
									Key:   "$in",
									Value: []string{word},
								},
							},
						},
					},
				})
		}
	}

	return result, wordFilter
}
