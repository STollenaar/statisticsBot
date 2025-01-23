package maxcommand

import (
	"fmt"
	"strings"

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
func (m MaxCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
		},
	})

	parsedArguments := m.ParseArguments(bot, interaction).(*CommandParsed)
	if parsedArguments.UserTarget != nil && parsedArguments.Word != "" {
		response := "Usage of both \"user\" and \"word\" at the same time is not correct. Please only specify either."
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &response,
		})
		return
	}

	maxWord := parsedArguments.FindAllWordOccurences()

	var response string

	targetUser, _ := bot.GuildMember(interaction.GuildID, maxWord.Author)

	if parsedArguments.UserTarget != nil {
		response = fmt.Sprintf("\"%s\" is the most common word used by %s.", maxWord.Word.Word, targetUser.Mention())
	} else {
		response = fmt.Sprintf("%s has used the word \"%s\" more than anyone else, a total of %d time(s)", targetUser.Mention(), maxWord.Word.Word, maxWord.Word.Count)
	}

	if (parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID) || maxWord.Author != interaction.Member.User.ID {
		targetUser := parsedArguments.UserTarget
		if targetUser == nil {
			t, _ := bot.GuildMember(interaction.GuildID, maxWord.Author)
			if t == nil {
				targetUser = &discordgo.User{Username: "Unknown User", ID: maxWord.Author}
				// response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Author, maxWord.Word.Word, maxWord.Word.Count)
			} else {
				targetUser = t.User
			}
		}
		response = fmt.Sprintf("%s has used the word \"%s\" the most, and is used %d time(s) \n", targetUser.Mention(), maxWord.Word.Word, maxWord.Word.Count)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" the most, and is used %d time(s) \n", maxWord.Word.Word, maxWord.Word.Count)
	}
	fmt.Printf("Max Output: %s", response)
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: []string{interaction.Member.User.ID},
		},
	})
}

func (m MaxCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        "user",
			Description: "User to filter with",
			Type:        discordgo.ApplicationCommandOptionUser,
			Required:    false,
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

// FindAllWordOccurences finding the occurences of a word in the database
func (c *CommandParsed) FindAllWordOccurences() util.CountGrouped {
	filter, params := c.GetFilter()

	messageObject, err := database.CountFilterOccurences(filter, c.Word, params)
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

func (c *CommandParsed) IsNotEmpty() bool {
	return c.UserTarget != nil || c.ChannelTarget != nil || c.Word != ""
}
