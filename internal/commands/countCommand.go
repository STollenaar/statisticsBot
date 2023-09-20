package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CountCommand counts the amount of occurences of a certain word
func CountCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := parseArguments(bot, interaction)
	if parsedArguments.UserTarget == nil {
		parsedArguments.UserTarget = interaction.Member.User
	}
	amount := FindSpecificWordOccurences(parsedArguments)

	var response string
	if parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID {
		response = fmt.Sprintf("%s has used the word \"%s\" %d time(s) \n", parsedArguments.UserTarget.Mention(), parsedArguments.Word, amount)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" %d time(s) \n", parsedArguments.Word, amount)
	}
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

// FindSpecificWordOccurences finding the occurences of a word in the database
func FindSpecificWordOccurences(args *CommandParsed) int {

	var channelFilter bson.D

	if args.ChannelTarget != nil {
		channelFilter = bson.D{
			primitive.E{
				Key:   "$eq",
				Value: args.ChannelTarget.ID,
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
						Key: "$regex",
						Value: primitive.Regex{
							Pattern: fmt.Sprintf("^%s$", args.Word),
							Options: "i",
						},
					},
				},
				"Author":    args.UserTarget.ID,
				"ChannelID": channelFilter,
			},
		},
	}

	messages, err := CountFilterOccurences(args.GuildID, filter, "")

	if err != nil {
		fmt.Println(err)
		return 0
	}
	if len(messages) == 0 {
		return 0
	}
	return messages[0].Word.Count
}
