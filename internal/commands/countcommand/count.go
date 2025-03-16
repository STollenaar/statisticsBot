package countcommand

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
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
func (c CountCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading Data...",
			Flags:   util.ConfigFile.SetEphemeral(),
		},
	})

	parsedArguments := c.ParseArguments(bot, interaction).(*CommandParsed)
	if parsedArguments.UserTarget == nil {
		parsedArguments.UserTarget = interaction.Member.User
	}
	amount := parsedArguments.FindSpecificWordOccurences()

	var response string
	if parsedArguments.UserTarget != nil && parsedArguments.UserTarget.ID != interaction.Member.User.ID {
		response = fmt.Sprintf("%s has used the word \"%s\" %d time(s) \n", parsedArguments.UserTarget.Mention(), parsedArguments.Word, amount)
	} else {
		response = fmt.Sprintf("You have used the word \"%s\" %d time(s) \n", parsedArguments.Word, amount)
	}
	bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

func (c CountCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        "word",
			Description: "Word to count",
			Type:        discordgo.ApplicationCommandOptionString,
			Required:    true,
		},
		{
			Name:        "user",
			Description: "User to filter with",
			Type:        discordgo.ApplicationCommandOptionUser,
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

func (c CountCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
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

// FindSpecificWordOccurences finding the occurences of a word in the database
func (c *CommandParsed) FindSpecificWordOccurences() int {

	filter, params := c.GetFilter()

	messages, err := database.CountFilterOccurences(filter, c.Word, params)

	if err != nil {
		fmt.Println(err)
		return 0
	}
	if len(messages) == 0 {
		return 0
	}
	return messages[0].Word.Count
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
