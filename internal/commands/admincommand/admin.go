package admincommand

import (
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/jobs"
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
	case "semantic":
		components = semanticHandler(sub)
	case "messages":
		components = messagesHandler(event, sub)
	case "mood":
		components = moodHandler(sub)
	}
	if len(components) != 0 {
		util.UpdateInteractionResponse(event, components)
	}
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
	case "semantic":
		components = semanticButtonHandler(event)
	case "messages":
		components = messagesButtonHandler(event)
	case "mood":
		components = moodButtonHandler(event)
	default:
		components = append(components, discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{
					Content: "Unknown button interaction",
				},
			},
		})
	}
	if len(components) != 0 {
		util.UpdateComponentInteractionResponse(event, components)
	}
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
		discord.ApplicationCommandOptionSubCommandGroup{
			Name:        "semantic",
			Description: "Manage semantic search",
			Options: []discord.ApplicationCommandOptionSubCommand{
				{
					Name:        "status",
					Description: "Show embedding coverage and backfill progress",
				},
				{
					Name:        "backfill",
					Description: "Embed every message that does not have an embedding yet",
				},
				{
					Name:        "list",
					Description: "List recent semantic searches",
				},
			},
		},
		discord.ApplicationCommandOptionSubCommandGroup{
			Name:        "mood",
			Description: "Manage mood classification",
			Options: []discord.ApplicationCommandOptionSubCommand{
				{
					Name:        "status",
					Description: "Show mood classification coverage and backfill progress",
				},
				{
					Name:        "backfill",
					Description: "Classify the mood of every message that has not been scored yet",
				},
			},
		},
		discord.ApplicationCommandOptionSubCommandGroup{
			Name:        "messages",
			Description: "Manage message ingestion",
			Options: []discord.ApplicationCommandOptionSubCommand{
				{
					Name:        "status",
					Description: "Show message sync progress",
				},
				{
					Name:        "sync",
					Description: "Scan every channel and store messages the database is missing",
				},
			},
		},
	}
}

// phaseLabel renders a job's phase for display, preferring the job's own word
// for what it is doing ("embedding", "scanning") over the generic phase name.
func phaseLabel(s jobs.Status) string {
	if s.Step != "" {
		return s.Step
	}
	return string(s.Phase)
}
