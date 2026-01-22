package countcommand

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	CountCmd = CountCommand{
		Name:        "count",
		Description: "Returns the amount of times a word is used.",
	}
)

type CountCommand struct {
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

// CountCommand counts the amount of occurences of a certain word
func (c CountCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}
	sub := event.SlashCommandInteractionData()
	amount := findSpecificWordOccurences(event.GuildID().String(), event.User().ID.String(), sub)

	var response string
	if sub.Options["user"].Snowflake().String() != event.User().ID.String() {
		response = fmt.Sprintf("%s has used the word \"%s\" %d time(s) \n", discord.UserMention(sub.Options["user"].Snowflake()), sub.Options["word"].String(), amount)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" %d time(s) \n", sub.Options["word"].String(), amount)
	}
	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (c CountCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "word",
			Description: "Word to count",
			Required:    true,
		},
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "User to filter with",
			Required:    false,
		},
		discord.ApplicationCommandOptionChannel{
			Name:        "channel",
			Description: "Channel to filter with",
			Required:    false,
		},
	}
}

// findSpecificWordOccurences finding the occurences of a word in the database
func findSpecificWordOccurences(guildID, authorID string, sub discord.SlashCommandInteractionData) int {

	filter, params := getFilter(guildID, authorID, sub)

	messages, err := database.CountFilterOccurences(filter, sub.Options["word"].String(), params)

	if err != nil {
		fmt.Println(err)
		return 0
	}
	if len(messages) == 0 {
		return 0
	}
	return messages[0].Word.Count
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
