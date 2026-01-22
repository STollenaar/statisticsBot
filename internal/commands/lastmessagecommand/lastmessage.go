package lastmessagecommand

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
)

var (
	LastMessageCmd = LastMessageCommand{
		Name:        "last",
		Description: "Returns the last time someone used a certain word somewhere or someone.",
	}
)

type LastMessageCommand struct {
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

// LastMessage find the last message of a person
func (l LastMessageCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}
	sub := event.SlashCommandInteractionData()

	filter, values := getFilter(event.GuildID().String(), event.User().ID.String(), sub)

	response := "Something went wrong.. maybe try again with something else?"

	// Query to find the most recent message for the specified channel_id
	query := `
		WITH latest_versions AS (
			SELECT m.*
			FROM messages m
			JOIN (
				SELECT id, MAX(version) AS latest_version
				FROM messages
				GROUP BY id
			) latest
				ON m.id = latest.id AND m.version = latest.latest_version
		),
		ranked_messages AS (
			SELECT *,
				ROW_NUMBER() OVER (ORDER BY date DESC) AS rank
			FROM latest_versions
			WHERE %s
		)
		SELECT 
			id AS message_id,
			channel_id,
			content,
			date AS most_recent_date
		FROM ranked_messages
		WHERE rank = 1;
	`

	filterResult, err := database.QueryDuckDB(fmt.Sprintf(query, filter), values)
	if err != nil {
		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &response,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
		return
	}
	var messageObject []*util.MessageObject

	for filterResult.Next() {
		var channel_id, message_id, content string
		var date time.Time

		err = filterResult.Scan(&message_id, &channel_id, &content, &date)
		if err != nil {
			break
		}
		lastMessage := &util.MessageObject{
			GuildID:   event.GuildID().String(),
			ChannelID: channel_id,
			MessageID: message_id,
			Author:    event.User().ID.String(),
			Content:   content,
			Date:      date,
		}
		messageObject = append(messageObject, lastMessage)
	}

	lastMessage := messageObject[0]
	messageLink := getMessageLink(lastMessage.GuildID, lastMessage.ChannelID, lastMessage.MessageID)
	response = fmt.Sprintf("%s last has send something in %s, and %s", 	discord.UserMention(sub.Options["user"].Snowflake()),discord.ChannelMention(snowflake.MustParse(lastMessage.ChannelID)), messageLink)

		_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (l LastMessageCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "User to filter with",
			Required:    true,
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

func (l LastMessageCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
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

func getMessageLink(GuildId, ChannelId, MessageId string) string {
	return fmt.Sprintf("[here is the message](https://discordapp.com/channels/%s/%s/%s)", GuildId, ChannelId, MessageId)
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
