package commands

import (
	"context"
	"fmt"

	"github.com/stollenaar/statisticsbot/lib"
	"github.com/stollenaar/statisticsbot/util"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LastMessage find the last message of a person
func LastMessage(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

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

	var messageObjects []util.MessageObject

	filterResult, err := lib.GetFromFilter(parsedArguments.GuildID, filter, findOptions)
	if err != nil {
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: "Something went wrong.. maybe try again with something else?",
		})

		return
	}

	err = filterResult.All(context.TODO(), &messageObjects)

	if err != nil {
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: "Something went wrong.. maybe try again with something else?",
		})
		return
	}
	messageObject := messageObjects[0]

	channel, _ := bot.Channel(messageObject.ChannelID)
	messageLink := getMessageLink(messageObject.GuildID, messageObject.ChannelID, messageObject.MessageID)
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: fmt.Sprintf("%s last has send something in %s, and %s", parsedArguments.UserTarget.Mention(), channel.Mention(), messageLink),
	})
}

func getMessageLink(GuildId, ChannelId, MessageId string) string {
	return fmt.Sprintf("[here is the message](https://discordapp.com/channels/%s/%s/%s)", GuildId, ChannelId, MessageId)
}
