package commands

import (
	"reflect"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/commands/countcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand"
	"github.com/stollenaar/statisticsbot/internal/commands/maxcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/moodcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/plotcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/summarizecommand"
	"github.com/stollenaar/statisticsbot/internal/util"
)

type CommandI interface {
	Handler(event *events.ApplicationCommandInteractionCreate)
	CreateCommandArguments() []discord.ApplicationCommandOption
}

var (
	Commands = []CommandI{
		countcommand.CountCmd,
		lastmessagecommand.LastMessageCmd,
		maxcommand.MaxCmd,
		summarizecommand.SummarizeCmd,
		moodcommand.MoodCmd,
		plotcommand.PlotCmd,
	}
	ApplicationCommands    []discord.ApplicationCommandCreate
	CommandHandlers        = make(map[string]func(e *events.ApplicationCommandInteractionCreate))
	MessageCommandHandlers = make(map[string]func(e *events.ApplicationCommandInteractionCreate))
	ModalSubmitHandlers    = make(map[string]func(e *events.ModalSubmitInteractionCreate))
	ComponentHandlers      = make(map[string]func(e *events.ComponentInteractionCreate))
)

func init() {
	for _, cmd := range Commands {
		ApplicationCommands = append(ApplicationCommands, &discord.SlashCommandCreate{
			Name:        reflect.ValueOf(cmd).FieldByName("Name").String(),
			Description: reflect.ValueOf(cmd).FieldByName("Description").String(),
			Options:     cmd.CreateCommandArguments(),
		})
		CommandHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = cmd.Handler

		if _, ok := reflect.TypeOf(cmd).MethodByName("ModalHandler"); ok {
			ModalSubmitHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = func(e *events.ModalSubmitInteractionCreate) {
				reflect.ValueOf(cmd).MethodByName("ModalHandler").Call([]reflect.Value{
					reflect.ValueOf(e),
				})
			}
		}

		if _, ok := reflect.TypeOf(cmd).MethodByName("ComponentHandler"); ok {
			ComponentHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = func(e *events.ComponentInteractionCreate) {
				reflect.ValueOf(cmd).MethodByName("ComponentHandler").Call([]reflect.Value{
					reflect.ValueOf(e),
				})
			}
		}
	}

	ApplicationCommands = append(ApplicationCommands,
		&discord.SlashCommandCreate{
			Name:        "ping",
			Description: "pong",
		},
	)

	CommandHandlers["ping"] = PingCommand
}

// PingCommand sends back the pong
func PingCommand(event *events.ApplicationCommandInteractionCreate) {
	event.CreateMessage(discord.MessageCreate{
		Content: "Pong",
		Flags:   util.ConfigFile.SetEphemeral(),
	})
}
