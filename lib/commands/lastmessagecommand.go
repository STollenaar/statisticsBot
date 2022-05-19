package commands

import (
	"fmt"
	"statsisticsbot/lib"
	"statsisticsbot/util"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LastMessage find the last message of a person
func LastMessage(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.ChannelTyping(interaction.ChannelID)

	parsedArguments := parseArguments(bot, interaction)

	var channelFilter bson.D

	if channel := parsedArguments.ChannelTarget; channel != nil {
		channelFilter = bson.D{
			primitive.E{
				Key:   "$eq",
				Value: channel.ID,
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
		"Author":    parsedArguments.UserTarget.ID,
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

	filterResult, err := lib.GetFromFilter(parsedArguments.GuildID, filter, findOptions)
	if err != nil {
		bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Something went wrong.. maybe try again with something else?",
			},
		})
		return
	}

	err = filterResult.Decode(&messageObject)

	if err != nil {
		bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Something went wrong.. maybe try again with something else?",
			},
		})
		return
	}

	channel, _ := Bot.Channel(messageObject.ChannelID)
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("%s last has send something in %s and said \"%s\"", parsedArguments.UserTarget.Mention(), channel.Mention(), strings.Join(messageObject.Content, " ")),
		},
	})
}
