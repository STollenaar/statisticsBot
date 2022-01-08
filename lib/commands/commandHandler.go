package commands

import (
	"context"
	"regexp"
	"statsisticsbot/lib"
	"statsisticsbot/util"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var Bot *discordgo.Session

// CommandParsed parsed struct for count command
type CommandParsed struct {
	Word          string
	GuildID       string
	UserTarget    string
	ChannelTarget string
}

// commandArgs basic command args
type commandArgs struct {
	Word        string `description:"The word to for the command."`
	Target      string `default:"" description:"The first to filter for."`
	TargetOther string `default:"" description:"The second to filter for."`
}

// parseArguments parses the arguments from the command into an unified struct
func parseArguments(message *discordgo.Message, args commandArgs) (parsedArguments CommandParsed) {
	reTarget := regexp.MustCompile("[\\<>@#&!]")

	parsedArguments = CommandParsed{Word: args.Word, GuildID: message.GuildID}

	if args.Target != "" {
		if strings.Contains(args.Target, "@") {
			parsedArguments.UserTarget = reTarget.ReplaceAllString(args.Target, "")
		} else if strings.Contains(args.Target, "#") {
			parsedArguments.ChannelTarget = reTarget.ReplaceAllString(args.Target, "")
		}
	} else {
		parsedArguments.UserTarget = message.Author.ID
	}

	if args.TargetOther != "" {
		if strings.Contains(args.TargetOther, "@") {
			parsedArguments.UserTarget = reTarget.ReplaceAllString(args.TargetOther, "")
		} else if strings.Contains(args.TargetOther, "#") {
			parsedArguments.ChannelTarget = reTarget.ReplaceAllString(args.TargetOther, "")
		}
	}

	return parsedArguments
}

// CountFilterOccurences counting the occurences of the filter inside the database
func CountFilterOccurences(guildID string, filter bson.D) (messageObject []util.CountGrouped) {
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

	resultCursor, err := lib.GetFromAggregate(guildID, mongo.Pipeline{filter, initialGroup, unwind, unwind, wordCount, resultGroup, counted})
	if err != nil {
		panic(err)
	}

	if err = resultCursor.All(context.TODO(), &messageObject); err != nil {
		panic(err)
	}
	return messageObject
}
