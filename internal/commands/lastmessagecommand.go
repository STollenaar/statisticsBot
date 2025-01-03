package commands

import (
	"fmt"
	"time"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"

	"github.com/bwmarrin/discordgo"
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

	filter, values := getFilter(parsedArguments)

	response := "Something went wrong.. maybe try again with something else?"

	// Query to find the most recent message for the specified channel_id
	query := `
		WITH ranked_messages AS (
			SELECT *,
				   ROW_NUMBER() OVER (ORDER BY date DESC) AS rank
			FROM messages
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

func getMessageLink(GuildId, ChannelId, MessageId string) string {
	return fmt.Sprintf("[here is the message](https://discordapp.com/channels/%s/%s/%s)", GuildId, ChannelId, MessageId)
}
