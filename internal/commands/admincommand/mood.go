package admincommand

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/embeddings"
	"github.com/stollenaar/statisticsbot/internal/jobs"
)

func moodHandler(sub discord.SlashCommandInteractionData) []discord.LayoutComponent {
	switch *sub.SubCommandName {
	case "status":
		return moodStatusComponents()
	case "backfill":
		return moodBackfillComponents()
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown mood subcommand")}}
}

func moodButtonHandler(event *events.ComponentInteractionCreate) []discord.LayoutComponent {
	action, _, ok := parseAdminCustomID(event.Data.CustomID())
	if !ok {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Malformed button ID")}}
	}

	switch action {
	case "status":
		return moodStatusComponents()
	case "backfill":
		return moodBackfillComponents()
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown mood action")}}
}

// moodStatusComponents reports how much of the history has been classified,
// plus the state of any backfill closing that gap.
func moodStatusComponents() []discord.LayoutComponent {
	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: "**Mood classification status**"},
	}

	coverage, err := database.GetMoodCoverage(embeddings.MoodModelName())
	if err != nil {
		slog.Error("Failed to read mood coverage", slog.Any("err", err))
		rows = append(rows, discord.TextDisplayComponent{Content: "⚠️ Failed to read mood coverage"})
	} else {
		pct := 0.0
		if coverage.Messages > 0 {
			pct = float64(coverage.Scored) / float64(coverage.Messages) * 100
		}
		rows = append(rows, discord.TextDisplayComponent{
			Content: fmt.Sprintf("Model `%s`\nScored **%d** of **%d** messages (%.1f%%) — **%d** missing",
				embeddings.MoodModelName(), coverage.Scored, coverage.Messages, pct, coverage.Missing),
		})
		if coverage.OtherModel > 0 {
			rows = append(rows, discord.TextDisplayComponent{
				Content: fmt.Sprintf("ℹ️ **%d** messages were scored by a different model; their labels and vector order do not match the active one.", coverage.OtherModel),
			})
		}
	}

	rows = append(rows, discord.SeparatorComponent{}, moodStatusRow())

	buttons := []discord.InteractiveComponent{
		discord.ButtonComponent{
			Style:    discord.ButtonStyleSecondary,
			Label:    "Refresh",
			CustomID: "admin_mood_status",
		},
	}
	if !jobs.MoodBackfill().Running() {
		buttons = append(buttons, discord.ButtonComponent{
			Style:    discord.ButtonStylePrimary,
			Label:    "Start backfill",
			CustomID: "admin_mood_backfill",
		})
	}
	rows = append(rows, discord.ActionRowComponent{Components: buttons})

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// moodBackfillComponents starts a backfill, or reports the running one.
func moodBackfillComponents() []discord.LayoutComponent {
	header := "**Mood backfill started**"
	if !jobs.StartMoodBackfill() {
		header = "**A mood backfill is already running**"
	}

	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: header},
		moodStatusRow(),
		discord.ActionRowComponent{Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Refresh",
				CustomID: "admin_mood_backfill",
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Status",
				CustomID: "admin_mood_status",
			},
		}},
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// moodStatusRow renders the current job state as one text component.
func moodStatusRow() discord.ContainerSubComponent {
	s := jobs.MoodBackfill()
	if s.Phase == jobs.PhaseIdle {
		return discord.TextDisplayComponent{Content: "Backfill: `idle` — no job has run since the last restart."}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Backfill: `%s` — %d/%d done", phaseLabel(s), s.Done, s.Total)
	if s.Failed > 0 {
		fmt.Fprintf(&b, ", **%d failed**", s.Failed)
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
