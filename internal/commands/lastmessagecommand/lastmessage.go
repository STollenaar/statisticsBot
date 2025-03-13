package lastmessagecommand

import (
	"fmt"
	"strings"
	"time"

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
func (l LastMessageCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := l.ParseArguments(bot, interaction).(*CommandParsed)

	filter, values := parsedArguments.GetFilter()

	response := "Something went wrong.. maybe try again with something else?"

	// Query to find the most recent message for the specified channel_id
	query := `
		WITH latest_versions AS (
			SELECT *
			FROM messages
			WHERE (id, version) IN (
				SELECT id, MAX(version)
				FROM messages
				GROUP BY id
			)
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
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &response,
		})
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
			GuildID:   parsedArguments.GuildID,
			ChannelID: channel_id,
			MessageID: message_id,
			Author:    parsedArguments.UserTarget.ID,
			Content:   content,
			Date:      date,
		}
		messageObject = append(messageObject, lastMessage)
	}

	lastMessage := messageObject[0]
	channel, _ := bot.Channel(lastMessage.ChannelID)
	messageLink := getMessageLink(lastMessage.GuildID, lastMessage.ChannelID, lastMessage.MessageID)
	response = fmt.Sprintf("%s last has send something in %s, and %s", parsedArguments.UserTarget.Mention(), channel.Mention(), messageLink)
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

func (l LastMessageCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        "user",
			Description: "User to filter with",
			Type:        discordgo.ApplicationCommandOptionUser,
			Required:    true,
		},
		{
			Name:        "word",
			Description: "Word to count",
			Type:        discordgo.ApplicationCommandOptionString,
			Required:    false,
		},
		{
			Name:        "channel",
			Description: "Channel to filter with",
			Type:        discordgo.ApplicationCommandOptionChannel,
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

func (c *CommandParsed) GetFilter() (string, []interface{}) {
	filters := []string{"guild_id = ?"}
	values := []interface{}{c.GuildID}

	if c.ChannelTarget != nil {
		filters = append(filters, "channel_id = ?")
		values = append(values, c.ChannelTarget.ID)
	}

	if c.UserTarget != nil {
		filters = append(filters, "author_id = ?")
		values = append(values, c.UserTarget.ID)
	}

	return strings.Join(filters, " AND "), values
}
