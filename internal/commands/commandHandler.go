package commands

import (
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/commands/countcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand"
	"github.com/stollenaar/statisticsbot/internal/commands/maxcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/moodcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/plotcommand"
	"github.com/stollenaar/statisticsbot/internal/commands/summarizecommand"
	"github.com/stollenaar/statisticsbot/internal/util"
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
		moodcommand.MoodCmd,
		plotcommand.PlotCmd,
	}
	ApplicationCommands []*discordgo.ApplicationCommand
	CommandHandlers     = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	ModalSubmitHandlers = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
)

func init() {
	for _, cmd := range Commands {
		ApplicationCommands = append(ApplicationCommands, &discordgo.ApplicationCommand{
			Name:        reflect.ValueOf(cmd).FieldByName("Name").String(),
			Description: reflect.ValueOf(cmd).FieldByName("Description").String(),
			Options:     cmd.CreateCommandArguments(),
		})
		CommandHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = cmd.Handler

		if _, ok := reflect.TypeOf(cmd).MethodByName("ModalHandler"); ok {
			ModalSubmitHandlers[reflect.ValueOf(cmd).FieldByName("Name").String()] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
				reflect.ValueOf(cmd).MethodByName("ModalHandler").Call([]reflect.Value{
					reflect.ValueOf(s),
					reflect.ValueOf(i),
				})
			}
		}
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
			Flags:   util.ConfigFile.SetEphemeral(),
		},
	})
}
