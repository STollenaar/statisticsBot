package admincommand

import (
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var (
	AdminCmd = AdminCommand{
		Name:        "admin",
		Description: "Admin command to manage to statsbot",
	}
)

type AdminCommand struct {
	Name        string
	Description string
}

func (a AdminCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	if event.Member().User.ID.String() != util.ConfigFile.ADMIN_USER_ID {
		event.CreateMessage(discord.MessageCreate{
			Content: "You are not the boss of me",
			Flags:   discord.MessageFlagEphemeral | discord.MessageFlagIsComponentsV2,
		})
		return
	}
	err := event.DeferCreateMessage(true)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	var components []discord.LayoutComponent
	switch *sub.SubCommandGroupName {
	case "summary":
		components = summaryHandler(sub)
	}
	util.UpdateInteractionResponse(event, components)
}

func (a AdminCommand) ComponentHandler(event *events.ComponentInteractionCreate) {
	if event.Member().User.ID.String() != util.ConfigFile.ADMIN_USER_ID {
		return
	}

	err := event.DeferUpdateMessage()

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	var components []discord.LayoutComponent
	switch strings.Split(event.Data.CustomID(), "_")[1] {
	case "summary":
		components = summaryButtonHandler(event)
	default:
		components = append(components, discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{
					Content: "Unknown button interaction",
				},
			},
		})
	}
	util.UpdateComponentInteractionResponse(event, components)
}

func (a AdminCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommandGroup{
			Name:        "summary",
			Description: "Manage summary invocations",
			Options: []discord.ApplicationCommandOptionSubCommand{
				{
					Name:        "list",
					Description: "List recent summary invocations",
				},
			},
		},
	}
}
