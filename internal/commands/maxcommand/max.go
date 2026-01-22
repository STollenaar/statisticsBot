package maxcommand

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
)

var (
	MaxCmd = MaxCommand{
		Name:        "max",
		Description: "Returns who used a certain word the most. In a certain channel, or of a user",
	}
)

type MaxCommand struct {
	Name        string
	Description string
}

// CommandParsed parsed struct for count command
type CommandParsed struct {
	Word          string
	GuildID       string
	UserTarget    *discordgo.User
	ChannelTarget *discordgo.Channel
}

// MaxCommand counts the amount of occurences of a certain word
func (m MaxCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}
	sub := event.SlashCommandInteractionData()

	keys := slices.Collect(maps.Keys(sub.Options))
	if slices.Contains(keys, "user") && slices.Contains(keys, "word") {
		response := "Usage of both \"user\" and \"word\" at the same time is not correct. Please only specify either."
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &response,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}

	maxWord := findAllWordOccurences(event.GuildID().String(), event.User().ID.String(), sub)

	var response string

	if _, ok := sub.Options["user"]; ok {
		response = fmt.Sprintf("%s has used the word \"%s\" more than anyone else, a total of %d time(s)", discord.UserMention(snowflake.MustParse(maxWord.Author)), maxWord.Word.Word, maxWord.Word.Count)
	} else {
		response = fmt.Sprintf("\"%s\" is the most common word used by %s.", maxWord.Word.Word, discord.UserMention(snowflake.MustParse(maxWord.Author)))
	}

	if (sub.Options["user"].Snowflake().String() != event.User().ID.String()) || maxWord.Author != event.User().ID.String() {
		var targetUser snowflake.ID
		tgtUser, ok := sub.Options["user"]
		if !ok {
			targetUser = snowflake.MustParse(maxWord.Author)
		} else {
			targetUser = tgtUser.Snowflake()
		}
		response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", discord.UserMention(targetUser), maxWord.Word.Word, maxWord.Word.Count)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
		AllowedMentions: &discord.AllowedMentions{
			Users: []snowflake.ID{event.User().ID},
		},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (m MaxCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "User to filter with",
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "word",
			Description: "Word to count",
			Required:    false,
		},
		discord.ApplicationCommandOptionChannel{
			Name:        "channel",
			Description: "Channel to filter with",
			Required:    false,
		},
	}
}

func (m MaxCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
	parsedArguments := new(CommandParsed)

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

// findAllWordOccurences finding the occurences of a word in the database
func findAllWordOccurences(guildID, authorID string, sub discord.SlashCommandInteractionData) util.CountGrouped {
	filter, params := getFilter(guildID, authorID, sub)

	messageObject, err := database.CountFilterOccurences(filter, sub.Options["word"].String(), params)
	if err != nil {
		fmt.Println(err)
		return util.CountGrouped{}
	}

	if len(messageObject) != 0 {
		return messageObject[0]
	} else {
		return util.CountGrouped{}
	}
}

func getFilter(guildID, authorID string, sub discord.SlashCommandInteractionData) (string, []interface{}) {
	filters := []string{"guild_id = ?"}
	values := []interface{}{guildID}

	if channel, ok := sub.Options["channel"]; ok {
		filters = append(filters, "channel_id = ?")
		values = append(values, channel.Snowflake().String())
	}

	if user, ok := sub.Options["user"]; ok {
		filters = append(filters, "author_id = ?")
		values = append(values, user.Snowflake().String())
	} else {
		filters = append(filters, "author_id = ?")
		values = append(values, authorID)
	}

	return strings.Join(filters, " AND "), values
}

func (c *CommandParsed) IsNotEmpty() bool {
	return c.UserTarget != nil || c.ChannelTarget != nil || c.Word != ""
}
