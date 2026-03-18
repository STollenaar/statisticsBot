package admincommand

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/commands/summarizecommand"
	"github.com/stollenaar/statisticsbot/internal/database"
)

const summaryPageSize = 5

func summaryHandler(sub discord.SlashCommandInteractionData) []discord.LayoutComponent {
	switch *sub.SubCommandName {
	case "list":
		return summaryListComponents(1)
	}
	return errorComponents("Unknown summary subcommand")
}

func summaryButtonHandler(event *events.ComponentInteractionCreate) []discord.LayoutComponent {
	// Custom ID format: admin_summary_<action>_<payload>
	parts := strings.SplitN(event.Data.CustomID(), "_", 4)
	if len(parts) < 4 {
		return errorComponents("Malformed button ID")
	}
	action, payload := parts[2], parts[3]

	switch action {
	case "retry":
		return summaryRetryComponents(payload)
	case "download":
		return summaryDownloadComponents(event, payload)
	case "page":
		page, err := strconv.Atoi(payload)
		if err != nil || page < 1 {
			page = 1
		}
		return summaryListComponents(page)
	}
	return errorComponents("Unknown summary action")
}

func summaryListComponents(page int) []discord.LayoutComponent {
	total, err := database.CountSummaryInvocations()
	if err != nil {
		slog.Error("Failed to count summary invocations", slog.Any("err", err))
		return errorComponents("Failed to fetch invocation count")
	}

	totalPages := (total + summaryPageSize - 1) / summaryPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	invocations, err := database.ListSummaryInvocations(page, summaryPageSize)
	if err != nil {
		slog.Error("Failed to list summary invocations", slog.Any("err", err))
		return errorComponents("Failed to fetch invocations")
	}

	var rows []discord.ContainerSubComponent
	rows = append(rows, discord.TextDisplayComponent{
		Content: fmt.Sprintf("**Summary Invocations** — page %d/%d (%d total)", page, totalPages, total),
	})

	if len(invocations) == 0 {
		rows = append(rows, discord.TextDisplayComponent{Content: "No invocations found."})
		return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
	}

	for _, inv := range invocations {
		rows = append(rows,
			discord.SeparatorComponent{},
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("%s **%s** | channel `%s` | unit `%s` | `%s`",
					statusEmoji(inv.Status), inv.Status, inv.ChannelID, inv.Unit,
					inv.RequestedAt.Format("2006-01-02 15:04:05"),
				),
			},
			discord.ActionRowComponent{
				Components: []discord.InteractiveComponent{
					discord.ButtonComponent{
						Style:    discord.ButtonStylePrimary,
						Label:    "Retry",
						CustomID: fmt.Sprintf("admin_summary_retry_%s", inv.ID),
					},
					discord.ButtonComponent{
						Style:    discord.ButtonStyleSecondary,
						Label:    "Download JSON",
						CustomID: fmt.Sprintf("admin_summary_download_%s", inv.ID),
					},
				},
			},
		)
	}

	// Pagination row — only shown when there is more than one page
	if totalPages > 1 {
		var navButtons []discord.InteractiveComponent
		if page > 1 {
			navButtons = append(navButtons,
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "⏮ First",
					CustomID: "admin_summary_page_1",
				},
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "← Previous",
					CustomID: fmt.Sprintf("admin_summary_page_%d", page-1),
				},
			)
		}
		if page < totalPages {
			navButtons = append(navButtons,
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "Next →",
					CustomID: fmt.Sprintf("admin_summary_page_%d", page+1),
				},
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "Last ⏭",
					CustomID: fmt.Sprintf("admin_summary_page_%d", totalPages),
				},
			)
		}
		rows = append(rows, discord.SeparatorComponent{}, discord.ActionRowComponent{Components: navButtons})
	}

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

func summaryDownloadComponents(event *events.ComponentInteractionCreate, id string) []discord.LayoutComponent {
	inv, err := database.GetSummaryInvocation(id)
	if err != nil {
		slog.Error("Failed to fetch summary invocation for download", slog.Any("err", err))
		return errorComponents("Invocation not found")
	}

	// Unmarshal the stored messages JSON so the combined file is pretty-printed
	var messages []summarizecommand.SummaryBody
	json.Unmarshal([]byte(inv.MessagesJSON), &messages)

	export := struct {
		ID          string                         `json:"id"`
		GuildID     string                         `json:"guild_id"`
		ChannelID   string                         `json:"channel_id"`
		Unit        string                         `json:"unit"`
		RequestedAt string                         `json:"requested_at"`
		Status      string                         `json:"status"`
		Messages    []summarizecommand.SummaryBody `json:"messages"`
		RawResponse string                         `json:"raw_response"`
	}{
		ID:          inv.ID,
		GuildID:     inv.GuildID,
		ChannelID:   inv.ChannelID,
		Unit:        inv.Unit,
		RequestedAt: inv.RequestedAt.Format("2006-01-02T15:04:05Z"),
		Status:      inv.Status,
		Messages:    messages,
		RawResponse: inv.RawResponse,
	}

	fileBytes, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal invocation export", slog.Any("err", err))
		return errorComponents("Failed to build JSON export")
	}

	fileName := fmt.Sprintf("invocation_%s.json", inv.ID)
	_, err = event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.MessageCreate{
		Flags: discord.MessageFlagEphemeral,
		Files: []*discord.File{
			discord.NewFile(fileName, "Summary invocation export", bytes.NewReader(fileBytes)),
		},
	})
	if err != nil {
		slog.Error("Failed to send follow-up with JSON file", slog.Any("err", err))
		return errorComponents("Failed to send file")
	}

	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{
					Content: fmt.Sprintf("JSON for invocation `%s` sent above.", id),
				},
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						discord.ButtonComponent{
							Style:    discord.ButtonStyleSecondary,
							Label:    "← Back to list",
							CustomID: "admin_summary_page_1",
						},
					},
				},
			},
		},
	}
}

func summaryRetryComponents(id string) []discord.LayoutComponent {
	inv, err := database.GetSummaryInvocation(id)
	if err != nil {
		slog.Error("Failed to fetch summary invocation", slog.Any("err", err))
		return errorComponents("Invocation not found")
	}

	var messages []summarizecommand.SummaryBody
	if err := json.Unmarshal([]byte(inv.MessagesJSON), &messages); err != nil {
		slog.Error("Failed to unmarshal messages JSON", slog.Any("err", err))
		return errorComponents("Failed to parse stored messages")
	}

	summaries, rawResponse, err := summarizecommand.GetSummary(messages)
	if err != nil {
		database.UpdateSummaryInvocation(id, rawResponse, "failed")
		return errorComponents(fmt.Sprintf("Retry failed: %s", err.Error()))
	}
	database.UpdateSummaryInvocation(id, rawResponse, "success")

	var rows []discord.ContainerSubComponent
	rows = append(rows, discord.TextDisplayComponent{
		Content: fmt.Sprintf("**Retry succeeded** for invocation `%s`", id),
	})
	for _, s := range summaries.Summaries {
		rows = append(rows,
			discord.SeparatorComponent{},
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("**%s**\n%s", s.Topic, s.Summary),
			},
		)
	}
	rows = append(rows,
		discord.SeparatorComponent{},
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "← Back to list",
					CustomID: "admin_summary_page_1",
				},
			},
		},
	)

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

func statusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "failed":
		return "❌"
	default:
		return "⏳"
	}
}

func errorComponents(msg string) []discord.LayoutComponent {
	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{Content: msg},
			},
		},
	}
}
