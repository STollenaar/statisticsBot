package commands

import (
	"context"
	"fmt"
	"sort"
	"statsisticsbot/lib"
	"statsisticsbot/util"

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

// CountFilterOccurences counting the occurences of the filter inside the database
func CountFilterOccurences(guildID string, filter bson.D, wordFilter string) (messageObject []util.CountGrouped) {
	initialGroup := bson.D{
		primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id": "$Author",
				"Words": bson.D{
					primitive.E{
						Key:   "$push",
						Value: "$Content",
					},
				},
			},
		},
	}
	unwind := bson.D{
		primitive.E{
			Key:   "$unwind",
			Value: "$Words",
		},
	}

	refilter := bson.D{
		primitive.E{
			Key:   "$skip",
			Value: 0,
		},
	}
	if wordFilter != "" {
		refilter = bson.D{
			primitive.E{
				Key: "$match",
				Value: bson.M{
					"Words": bson.D{
						primitive.E{
							Key:   "$in",
							Value: []string{fmt.Sprintf("/%s/i", wordFilter)},
						},
					},
				},
			},
		}
	}

	wordCount := bson.D{
		primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id": bson.M{
					"Author": "$_id",
					"Word":   "$Words",
				},
				"wordCount": bson.D{
					primitive.E{
						Key:   "$sum",
						Value: 1,
					},
				},
			},
		},
	}

	resultGroup := bson.D{
		primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id": "$_id.Author",
				"Words": bson.D{
					primitive.E{
						Key: "$push",
						Value: bson.M{
							"Word": "$_id.Word",
							"wordCount": bson.D{
								primitive.E{
									Key:   "$sum",
									Value: "$wordCount",
								},
							},
						},
					},
				},
			},
		},
	}

	counted := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{
					Key:   "_id",
					Value: "$_id",
				},
				primitive.E{
					Key: "Word",
					Value: bson.M{
						"$arrayElemAt": []interface{}{"$Words", bson.M{
							"$indexOfArray": []interface{}{"$Words.wordCount", bson.D{
								primitive.E{
									Key:   "$max",
									Value: "$Words.wordCount",
								},
							}},
						}},
					},
				},
			},
		},
	}

	resultCursor, err := lib.GetFromAggregate(guildID, mongo.Pipeline{filter, initialGroup, unwind, unwind, refilter, wordCount, resultGroup, counted})
	if err != nil {
		panic(err)
	}

	if err = resultCursor.All(context.TODO(), &messageObject); err != nil {
		panic(err)
	}
	return messageObject
}
