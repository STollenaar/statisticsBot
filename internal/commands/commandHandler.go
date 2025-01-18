package commands

import (
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/commands/countcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand"
	"github.com/stollenaar/statisticsbot/internal/commands/maxcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/summarizecommand"
)

type CommandI interface {
	Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate)
	CreateCommandArguments() []*discordgo.ApplicationCommandOption
	ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{}
}

var (
	Commands = []CommandI{
		countcommand.CountCmd,
		lastmessagecommand.LastMessageCmd,
		maxcommand.MaxCmd,
		summarizecommand.SummarizeCmd,
	}
	ApplicationCommands []*discordgo.ApplicationCommand
	CommandHandlers     = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
)

func init() {
	for _, cmd := range Commands {
		ApplicationCommands = append(ApplicationCommands, &discordgo.ApplicationCommand{
			Name:        reflect.ValueOf(cmd).FieldByName("Name").String(),
			Description: reflect.ValueOf(cmd).FieldByName("Description").String(),
			Options:     cmd.CreateCommandArguments(),
		})

		CommandHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = cmd.Handler
	}

	ApplicationCommands = append(ApplicationCommands,
		&discordgo.ApplicationCommand{
			Name:        "ping",
			Description: "pong",
		},
	)

	CommandHandlers["ping"] = PingCommand
}

// PingCommand sends back the pong
func PingCommand(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong",
		},
	})
}
