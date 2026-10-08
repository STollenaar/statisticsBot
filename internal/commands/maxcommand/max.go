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

	// Whose count to report: the explicitly requested user when there is one,
	// otherwise whoever the query turned up. `user` is optional, so it must be
	// read with the two-value form — indexing a missing key yields a zero
	// SlashCommandOption whose Snowflake() panics on its nil Value.
	target := maxWord.Author
	if user, ok := sub.Options["user"]; ok {
		target = user.Snowflake().String()
	}

	response := maxResponse(maxWord, target, event.User().ID.String())

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

// maxResponse renders the reply for a /max result. target is the user whose
// count is being reported, invoker is whoever ran the command, so the wording
// can address them directly.
func maxResponse(maxWord util.CountGrouped, target, invoker string) string {
	switch {
	case maxWord.Author == "":
		// findAllWordOccurences returns a zero CountGrouped both when nothing
		// matched and when the query failed, so there is no author to name and
		// no count worth reporting.
		return "No messages matched those parameters."

	case target == invoker:
		return fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)

	default:
		targetUser, err := snowflake.Parse(target)
		if err != nil {
			// A stored author_id that is not a snowflake. Report the count
			// without a mention rather than taking the handler down.
			slog.Error("unparseable author id", slog.String("author", target), slog.Any("err", err))
			return fmt.Sprintf("The word \"%s\" has been used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)
		}
		return fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", discord.UserMention(targetUser), maxWord.Word.Word, maxWord.Word.Count)
	}
}

// findAllWordOccurences finding the occurences of a word in the database
func findAllWordOccurences(guildID, authorID string, sub discord.SlashCommandInteractionData) util.CountGrouped {
	filter, params := getFilter(guildID, authorID, sub)

	messageObject, err := database.CountFilterOccurences(filter, sub.Options["word"].String(), params)
	if err != nil {
		slog.Error("error counting word occurrences", slog.Any("err", err))
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
