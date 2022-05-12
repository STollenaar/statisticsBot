package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CountCommand counts the amount of occurences of a certain word
func CountCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.ChannelTyping(interaction.ChannelID)

	// Access options in the order provided by the user.
	options := interaction.ApplicationCommandData().Options

	// Or convert the slice into a map
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var parsedArguments CommandParsed
	if option, ok := optionMap["word"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.Word = option.StringValue()
	}
	if option, ok := optionMap["user"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.UserTarget = option.StringValue()
	}
	if option, ok := optionMap["channel"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.ChannelTarget = option.StringValue()
	}

	amount := FindSpecificWordOccurences(parsedArguments)

	var response string
	if parsedArguments.UserTarget != interaction.Member.User.ID {
		user, _ := Bot.User(parsedArguments.UserTarget)
		response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", user.Mention(), parsedArguments.Word, amount)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", parsedArguments.Word, amount)
	}
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
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
