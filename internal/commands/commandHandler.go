package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
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

func CountFilterOccurences(filter, word string, params []interface{}) (messageObjects []util.CountGrouped, err error) {
	query := `
		WITH tokenized_messages AS (
			SELECT 
				author_id,
				guild_id,
				LOWER(unnest(string_split(regexp_replace(content, '[^a-zA-Z0-9'' ]', '', 'g'), ' '))) AS word
			FROM messages
			%s
		)
		SELECT 
            guild_id,
			author_id,
			word,
			COUNT(*) AS word_count
		FROM tokenized_messages
		%s
		GROUP BY author_id, guild_id, word
		ORDER BY word_count DESC;
	`

	tokenFilter := `WHERE %s`
	wordFilter := `WHERE word = LOWER(?)`
	var q string
	if word != "" {
		q = fmt.Sprintf(query, fmt.Sprintf(tokenFilter, filter), wordFilter)
	} else {
		q = fmt.Sprintf(query, fmt.Sprintf(tokenFilter, filter), "")
	}

	messages, err := database.GetFromFilter(q, append(params, word))
	if err != nil {
		return nil, err
	}

	for messages.Next() {
		var guild_id, author_id, word string
		var word_count int

		err = messages.Scan(&guild_id, &author_id, &word, &word_count)
		if err != nil {
			break
		}

		messageObject := util.CountGrouped{
			Author: author_id,
			Word: util.WordCounted{
				Word:  word,
				Count: word_count,
			},
		}
		messageObjects = append(messageObjects, messageObject)
	}
	return
}

func getFilter(arguments *CommandParsed) (string, []interface{}) {
	filters := []string{"guild_id = ?"}
	values := []interface{}{arguments.GuildID}

	if arguments.ChannelTarget != nil {
		filters = append(filters, "channel_id = ?")
		values = append(values, arguments.ChannelTarget.ID)
	}

	if arguments.UserTarget != nil {
		filters = append(filters, "author_id = ?")
		values = append(values, arguments.UserTarget.ID)
	}

	return strings.Join(filters, " AND "), values
}
