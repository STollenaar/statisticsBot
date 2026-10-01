package admincommand

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/jobs"
)

func messagesHandler(event *events.ApplicationCommandInteractionCreate, sub discord.SlashCommandInteractionData) []discord.LayoutComponent {
	switch *sub.SubCommandName {
	case "status":
		return messagesStatusComponents()
	case "sync":
		return messagesSyncComponents(event.Client())
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown messages subcommand")}}
}

func messagesButtonHandler(event *events.ComponentInteractionCreate) []discord.LayoutComponent {
	action, _, ok := parseAdminCustomID(event.Data.CustomID())
	if !ok {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Malformed button ID")}}
	}

	switch action {
	case "status":
		return messagesStatusComponents()
	case "sync":
		return messagesSyncComponents(event.Client())
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown messages action")}}
}

// messagesStatusComponents reports the state of the message sync.
func messagesStatusComponents() []discord.LayoutComponent {
	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: "**Message sync status**"},
		syncStatusRow(),
	}

	buttons := []discord.InteractiveComponent{
		discord.ButtonComponent{
			Style:    discord.ButtonStyleSecondary,
			Label:    "Refresh",
			CustomID: "admin_messages_status",
		},
	}
	if !jobs.MessageSync().Running() {
		buttons = append(buttons, discord.ButtonComponent{
			Style:    discord.ButtonStylePrimary,
			Label:    "Start sync",
			CustomID: "admin_messages_sync",
		})
	}
	rows = append(rows, discord.ActionRowComponent{Components: buttons})

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// messagesSyncComponents starts a sync, or reports the running one when a job is
// already in flight.
func messagesSyncComponents(client *bot.Client) []discord.LayoutComponent {
	header := "**Message sync started**"
	if !jobs.StartMessageSync(client) {
		header = "**A message sync is already running**"
	}

	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: header},
		syncStatusRow(),
		discord.ActionRowComponent{Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Refresh",
				CustomID: "admin_messages_sync",
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Status",
				CustomID: "admin_messages_status",
			},
		}},
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// syncStatusRow renders the current job state as one text component.
func syncStatusRow() discord.ContainerSubComponent {
	s := jobs.MessageSync()
	if s.Phase == jobs.PhaseIdle {
		return discord.TextDisplayComponent{Content: "Sync: `idle` — no job has run since the last restart."}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Sync: `%s` — %d/%d channels", phaseLabel(s), s.Done, s.Total)
	fmt.Fprintf(&b, "\nAdded **%d** messages, **%d** reactions",
		s.Counter(jobs.CounterAdded), s.Counter(jobs.CounterReactions))
	if s.Failed > 0 {
		fmt.Fprintf(&b, ", **%d** channels failed", s.Failed)
	}
	if s.Rate != "" {
		fmt.Fprintf(&b, "\nRate %s", s.Rate)
	}
	if s.ETA != "" {
		fmt.Fprintf(&b, " · ETA %s", s.ETA)
	}
	if s.Elapsed != "" {
		fmt.Fprintf(&b, " · elapsed %s", s.Elapsed)
	}
	if s.Error != "" {
		fmt.Fprintf(&b, "\nLast error: `%s`", truncate(s.Error, 200))
	}
	return discord.TextDisplayComponent{Content: b.String()}
}
