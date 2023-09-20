package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/stollenaar/statisticsbot/lib"
	"github.com/stollenaar/statisticsbot/util"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CommandParsed parsed struct for count command
type CommandParsed struct {
	Word          string
	GuildID       string
	UserTarget    *discordgo.User
	ChannelTarget *discordgo.Channel
}

func (cmd *CommandParsed) isNotEmpty() bool {
	return cmd.UserTarget != nil || cmd.ChannelTarget != nil || cmd.Word != ""
}

// parseArguments parses the arguments from the command into an unified struct
func parseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) (parsedArguments *CommandParsed) {
	parsedArguments = new(CommandParsed)

	// Access options in the order provided by the user.
	options := interaction.ApplicationCommandData().Options
	parsedArguments.GuildID = interaction.GuildID
	// Or convert the slice into a map
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if option, ok := optionMap["word"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.Word = option.StringValue()
	}
	if option, ok := optionMap["user"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.UserTarget = option.UserValue(bot)
	}
	if option, ok := optionMap["channel"]; ok {
		// Option values must be type asserted from interface{}.
		// Discordgo provides utility functions to make this simple.
		parsedArguments.ChannelTarget = option.ChannelValue(bot)
	}

	return parsedArguments
}

func CreateCommandArguments(wordRequired, userRequired, channelRequired bool) (args []*discordgo.ApplicationCommandOption) {
	args = append(args,
		&discordgo.ApplicationCommandOption{
			Name:        "word",
			Description: "Word to count",
			Type:        discordgo.ApplicationCommandOptionString,
			Required:    wordRequired,
		},
		&discordgo.ApplicationCommandOption{
			Name:        "user",
			Description: "User to filter with",
			Type:        discordgo.ApplicationCommandOptionUser,
			Required:    userRequired,
		},
		&discordgo.ApplicationCommandOption{
			Name:        "channel",
			Description: "Channel to filter with",
			Type:        discordgo.ApplicationCommandOptionChannel,
			Required:    channelRequired,
		},
	)
	sort.Slice(args, func(i, j int) bool {
		return args[i].Required
	})

	return args
}

func CountFilterOccurences(guildID string, filter bson.D, wordFilter string) (messageObject []util.CountGrouped,err error) {
	pipeline := mongo.Pipeline{
		filter,
	}

	if wordFilter != "" {
		pipeline = append(pipeline, bson.D{
			bson.E{
				Key: "$match",
				Value: bson.M{
					"GuildID": guildID,
					"Content": bson.M{
						"$regex": primitive.Regex{
							Pattern: fmt.Sprintf("^%s$", wordFilter),
							Options: "i",
						},
					},
				},
			},
		})
	}

	pipeline = append(pipeline,
		bson.D{
			bson.E{
				Key:   "$unwind",
				Value: "$Content",
			},
		},
		bson.D{
			bson.E{
				Key: "$group",
				Value: bson.M{
					"_id": bson.M{
						"Author": "$Author",
						"Word":   "$Content",
					},
					"wordCount": bson.M{
						"$sum": 1,
					},
				},
			},
		},
		bson.D{
			bson.E{
				Key: "$group",
				Value: bson.M{
					"_id": "$_id.Author",
					"Words": bson.M{
						"$push": bson.M{
							"Word":      "$_id.Word",
							"wordCount": "$wordCount",
						},
					},
				},
			},
		},
		bson.D{
			bson.E{
				Key: "$project",
				Value: bson.M{
					"_id": 1,
					"Word": bson.M{
						"$arrayElemAt": []interface{}{
							"$Words",
							bson.M{
								"$indexOfArray": []interface{}{
									"$Words.wordCount",
									bson.M{
										"$max": "$Words.wordCount",
									},
								},
							},
						},
					},
				},
			},
		},
	)

	cursor, err := lib.GetFromAggregate(guildID, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err := cursor.All(context.TODO(), &messageObject); err != nil {
		return nil, err
	}

	return messageObject, nil
}
