package admincommand

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/embedbackfill"
	"github.com/stollenaar/statisticsbot/internal/embeddings"
)

const (
	// semanticPageSize fits comfortably under maxComponents: each search is a
	// single text line with no nested buttons of its own.
	semanticPageSize = 10
	// semanticQueryPreview caps how much of a query is echoed into the list.
	semanticQueryPreview = 80
)

func semanticHandler(sub discord.SlashCommandInteractionData) []discord.LayoutComponent {
	switch *sub.SubCommandName {
	case "status":
		return semanticStatusComponents()
	case "backfill":
		return semanticBackfillComponents()
	case "list":
		return semanticListComponents("1")
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown semantic subcommand")}}
}

// parseSemanticCustomID splits a button ID of the form
// admin_semantic_<action>_<payload>. The payload is optional, so the buttons
// that carry no state ("status", "backfill") parse the same way as "page_3".
func parseSemanticCustomID(customID string) (action, payload string, ok bool) {
	parts := strings.SplitN(customID, "_", 4)
	if len(parts) < 3 {
		return "", "", false
	}
	if len(parts) == 4 {
		payload = parts[3]
	}
	return parts[2], payload, true
}

func semanticButtonHandler(event *events.ComponentInteractionCreate) []discord.LayoutComponent {
	action, payload, ok := parseSemanticCustomID(event.Data.CustomID())
	if !ok {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Malformed button ID")}}
	}

	switch action {
	case "status":
		return semanticStatusComponents()
	case "backfill":
		return semanticBackfillComponents()
	case "page":
		return semanticListComponents(payload)
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown semantic action")}}
}

// semanticStatusComponents reports how much of the history semantic search can
// reach, plus the state of any backfill closing that gap.
func semanticStatusComponents() []discord.LayoutComponent {
	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: "**Semantic search status**"},
	}

	coverage, err := database.GetEmbeddingCoverage(embeddings.ModelName())
	if err != nil {
		slog.Error("Failed to read embedding coverage", slog.Any("err", err))
		rows = append(rows, discord.TextDisplayComponent{Content: "⚠️ Failed to read embedding coverage"})
	} else {
		pct := 0.0
		if coverage.Messages > 0 {
			pct = float64(coverage.Embedded) / float64(coverage.Messages) * 100
		}
		rows = append(rows, discord.TextDisplayComponent{
			Content: fmt.Sprintf("Model `%s`\nEmbedded **%d** of **%d** messages (%.1f%%) — **%d** missing",
				embeddings.ModelName(), coverage.Embedded, coverage.Messages, pct, coverage.Missing),
		})
		if coverage.OtherModel > 0 {
			rows = append(rows, discord.TextDisplayComponent{
				Content: fmt.Sprintf("ℹ️ **%d** messages have embeddings from a different model and are not searchable until re-embedded.", coverage.OtherModel),
			})
		}
	}

	rows = append(rows, discord.SeparatorComponent{}, backfillStatusRow())

	buttons := []discord.InteractiveComponent{
		discord.ButtonComponent{
			Style:    discord.ButtonStyleSecondary,
			Label:    "Refresh",
			CustomID: "admin_semantic_status",
		},
	}
	if !embedbackfill.Current().Running() {
		buttons = append(buttons, discord.ButtonComponent{
			Style:    discord.ButtonStylePrimary,
			Label:    "Start backfill",
			CustomID: "admin_semantic_backfill",
		})
	}
	rows = append(rows, discord.ActionRowComponent{Components: buttons})

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// semanticBackfillComponents starts a backfill, or reports the running one when
// a job is already in flight.
func semanticBackfillComponents() []discord.LayoutComponent {
	started := embedbackfill.Start()
	header := "**Embedding backfill started**"
	if !started {
		header = "**A backfill is already running**"
	}

	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: header},
		backfillStatusRow(),
		discord.ActionRowComponent{Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Refresh",
				CustomID: "admin_semantic_backfill",
			},
			discord.ButtonComponent{
				Style:    discord.ButtonStyleSecondary,
				Label:    "Status",
				CustomID: "admin_semantic_status",
			},
		}},
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

// backfillStatusRow renders the current job state as one text component.
func backfillStatusRow() discord.ContainerSubComponent {
	s := embedbackfill.Current()
	if s.Phase == embedbackfill.PhaseIdle {
		return discord.TextDisplayComponent{Content: "Backfill: `idle` — no job has run since the last restart."}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Backfill: `%s` — %d/%d done", s.Phase, s.Done, s.Total)
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

// semanticListComponents pages through recorded /semantic searches.
func semanticListComponents(payload string) []discord.LayoutComponent {
	total, err := database.CountSemanticInvocations()
	if err != nil {
		slog.Error("Failed to count semantic invocations", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to fetch search count")}}
	}
	totalPages := (total + semanticPageSize - 1) / semanticPageSize
	if totalPages == 0 {
		totalPages = 1
	}

	page, err := strconv.Atoi(payload)
	if err != nil {
		page = 1
	}
	switch payload {
	case "first":
		page = 1
	case "last":
		page = totalPages
	}
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}

	invocations, err := database.ListSemanticInvocations(page, semanticPageSize)
	if err != nil {
		slog.Error("Failed to list semantic invocations", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to fetch searches")}}
	}

	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{
			Content: fmt.Sprintf("**Semantic Searches** — page %d/%d (%d total)", page, totalPages, total),
		},
	}
	if len(invocations) == 0 {
		rows = append(rows, discord.TextDisplayComponent{Content: "No searches recorded yet."})
		return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
	}

	for _, inv := range invocations {
		author := inv.AuthorID
		if id, parseErr := snowflake.Parse(inv.AuthorID); parseErr == nil {
			author = discord.UserMention(id)
		}
		line := fmt.Sprintf("%s `%s` — %q → **%d** results · %s · %s",
			semanticStatusEmoji(inv.Status), inv.Status,
			truncate(inv.Query, semanticQueryPreview), inv.ResultCount,
			author, inv.RequestedAt.Format("2006-01-02 15:04:05"),
		)
		if inv.Error != "" {
			line += fmt.Sprintf("\n└ `%s`", truncate(inv.Error, 150))
		}
		rows = append(rows, discord.TextDisplayComponent{Content: line})
	}

	if totalPages > 1 {
		var navButtons []discord.InteractiveComponent
		if page > 1 {
			navButtons = append(navButtons,
				discord.ButtonComponent{Style: discord.ButtonStyleSecondary, Label: "⏮ First", CustomID: "admin_semantic_page_first"},
				discord.ButtonComponent{Style: discord.ButtonStyleSecondary, Label: "← Previous", CustomID: fmt.Sprintf("admin_semantic_page_%d", page-1)},
			)
		}
		if page < totalPages {
			navButtons = append(navButtons,
				discord.ButtonComponent{Style: discord.ButtonStyleSecondary, Label: "Next →", CustomID: fmt.Sprintf("admin_semantic_page_%d", page+1)},
				discord.ButtonComponent{Style: discord.ButtonStyleSecondary, Label: "Last ⏭", CustomID: "admin_semantic_page_last"},
			)
		}
		rows = append(rows, discord.SeparatorComponent{}, discord.ActionRowComponent{Components: navButtons})
	}

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

func semanticStatusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "empty":
		return "🕳️"
	case "failed":
		return "❌"
	}
	return "❔"
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
