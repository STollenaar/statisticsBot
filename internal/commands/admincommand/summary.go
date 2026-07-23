package admincommand

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/google/uuid"
	"github.com/stollenaar/statisticsbot/internal/commands/summarizecommand"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

const (
	// summaryPageSize is intentionally small: each original attempt plus its
	// nested retries costs several components, and Discord caps a message at
	// maxComponents total (nested buttons included).
	summaryPageSize = 3
	maxComponents   = 40
)

func summaryHandler(sub discord.SlashCommandInteractionData) []discord.LayoutComponent {
	switch *sub.SubCommandName {
	case "list":
		return summaryListComponents("1")
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown summary subcommand")}}
}

func summaryButtonHandler(event *events.ComponentInteractionCreate) []discord.LayoutComponent {
	// Custom ID format: admin_summary_<action>_<payload>
	parts := strings.SplitN(event.Data.CustomID(), "_", 4)
	if len(parts) < 4 {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Malformed button ID")}}
	}
	action, payload := parts[2], parts[3]

	switch action {
	case "retry":
		return summaryRetryComponents(payload)
	case "download":
		return summaryDownloadComponents(event, payload)
	case "response":
		return summaryResponseComponents(payload)
	case "post":
		return summaryPostComponents(event, payload)
	case "page":
		return summaryListComponents(payload)
	}
	return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Unknown summary action")}}
}

func summaryListComponents(payload string) []discord.LayoutComponent {
	total, err := database.CountSummaryInvocations()
	if err != nil {
		slog.Error("Failed to count summary invocations", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to fetch invocation count")}}
	}
	totalPages := (total + summaryPageSize - 1) / summaryPageSize
	if totalPages == 0 {
		totalPages = 1
	}

	page, err := strconv.Atoi(payload)
	switch payload {
	case "first":
		page = 1
	case "last":
		page = totalPages
	}

	if page > totalPages {
		page = totalPages
	}

	invocations, err := database.ListSummaryInvocations(page, summaryPageSize)
	if err != nil {
		slog.Error("Failed to list summary invocations", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to fetch invocations")}}
	}

	var rows []discord.ContainerSubComponent
	rows = append(rows, discord.TextDisplayComponent{
		Content: fmt.Sprintf("**Summary Invocations** — page %d/%d (%d total)", page, totalPages, total),
	})

	if len(invocations) == 0 {
		rows = append(rows, discord.TextDisplayComponent{Content: "No invocations found."})
		return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
	}

	// Discord caps a message at maxComponents total components (nested buttons
	// included). Reserve the container and the optional pagination row, always
	// render every original attempt, then fill whatever budget is left with
	// their retries — trimming with a note rather than overflowing the limit.
	budget := maxComponents - 1 // the enclosing container counts as one
	if totalPages > 1 {
		budget -= 6 // separator + action row + up to four nav buttons
	}

	blocks := make([][]discord.ContainerSubComponent, len(invocations))
	originalsCost := 0
	for i, inv := range invocations {
		blocks[i] = append([]discord.ContainerSubComponent{discord.SeparatorComponent{}}, invocationRows(inv, "")...)
		originalsCost += countComponents(blocks[i])
	}
	retryBudget := budget - countComponents(rows) - originalsCost

	for i, inv := range invocations {
		rows = append(rows, blocks[i]...)

		retries, err := database.ListSummaryRetries(inv.ID)
		if err != nil {
			slog.Warn("Failed to list summary retries", slog.Any("err", err))
		}
		rows, retryBudget = appendRetries(rows, retries, retryBudget)
	}

	// Pagination row — only shown when there is more than one page
	if totalPages > 1 {
		var navButtons []discord.InteractiveComponent
		if page > 1 {
			navButtons = append(navButtons,
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "⏮ First",
					CustomID: "admin_summary_page_first",
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
					CustomID: "admin_summary_page_last",
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
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Invocation not found")}}
	}

	// Unmarshal the stored messages JSON so the combined file is pretty-printed
	var messages []util.SummaryBody
	json.Unmarshal([]byte(inv.MessagesJSON), &messages)

	export := struct {
		ID          string             `json:"id"`
		GuildID     string             `json:"guild_id"`
		ChannelID   string             `json:"channel_id"`
		Unit        string             `json:"unit"`
		RequestedAt string             `json:"requested_at"`
		Status      string             `json:"status"`
		Messages    []util.SummaryBody `json:"messages"`
		RawResponse string             `json:"raw_response"`
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
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to build JSON export")}}
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
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to send file")}}
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

func summaryResponseComponents(id string) []discord.LayoutComponent {
	inv, err := database.GetSummaryInvocation(id)
	if err != nil {
		slog.Error("Failed to fetch summary invocation for download", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Invocation not found")}}
	}

	// Unmarshal the stored messages JSON so the combined file is pretty-printed
	var summaries util.SummaryResponse
	json.Unmarshal([]byte(inv.RawResponse), &summaries)

	var components []discord.ContainerSubComponent
	for _, summary := range summaries.Summaries {
		components = append(components, discord.TextDisplayComponent{
			Content: fmt.Sprintf("### %s\n%s", summary.Topic, summary.Summary),
		})
	}
	components = append(components,
		discord.SeparatorComponent{},
		discord.TextDisplayComponent{
			Content: fmt.Sprintf("Response for invocation `%s` sent above.", id),
		},
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "← Back to list",
					CustomID: "admin_summary_page_1",
				},
			},
		})

	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: components,
		},
	}
}

func summaryRetryComponents(id string) []discord.LayoutComponent {
	inv, err := database.GetSummaryInvocation(id)
	var rows []discord.ContainerSubComponent

	// Retries are always grouped under the root original attempt, so retrying a
	// retry links back to the same parent rather than nesting further.
	parentID := inv.ID
	if inv.ParentID != "" {
		parentID = inv.ParentID
	}
	// The "Post" button links back to whichever attempt owns the original
	// Discord message; that is always the root parent.
	postID := parentID

	if err != nil {
		slog.Error("Failed to fetch summary invocation", slog.Any("err", err))
		rows = errorComponents("Invocation not found")
	} else {
		var messages []util.SummaryBody

		if err := json.Unmarshal([]byte(inv.MessagesJSON), &messages); err != nil {
			slog.Error("Failed to unmarshal messages JSON", slog.Any("err", err))
			rows = errorComponents("Failed to parse stored messages")
		} else {
			summaries, rawResponse, err := summarizecommand.GetSummary(messages)

			status := "success"
			if err != nil {
				status = "failed"
			} else if len(summaries.Summaries) == 0 {
				status = "empty"
			}

			// Preserve the previous try by storing the retry as a new row linked
			// to the original attempt instead of overwriting it.
			retryID := uuid.New().String()
			if saveErr := database.SaveSummaryRetry(retryID, parentID, inv.GuildID, inv.ChannelID, inv.Unit, inv.MessagesJSON, rawResponse, status); saveErr != nil {
				slog.Warn("Failed to save summary retry", slog.Any("err", saveErr))
			}

			if err != nil {
				rows = errorComponents(fmt.Sprintf("Retry failed: %s", err.Error()))
			} else {
				rows = append(rows, discord.TextDisplayComponent{
					Content: fmt.Sprintf("**Retry succeeded** for invocation `%s`", parentID),
				})
				for _, s := range summaries.Summaries {
					rows = append(rows,
						discord.SeparatorComponent{},
						discord.TextDisplayComponent{
							Content: fmt.Sprintf("**%s**\n%s", s.Topic, s.Summary),
						},
					)
				}
			}
		}
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
				discord.ButtonComponent{
					Style:    discord.ButtonStyleSecondary,
					Label:    "Post",
					CustomID: fmt.Sprintf("admin_summary_post_%s", postID),
				},
			},
		},
	)

	return []discord.LayoutComponent{discord.ContainerComponent{Components: rows}}
}

func summaryPostComponents(event *events.ComponentInteractionCreate, id string) []discord.LayoutComponent {
	layouts := event.ComponentInteraction.Message.Components
	if len(layouts) == 0 {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Nothing to post")}}
	}
	container, ok := layouts[0].(discord.ContainerComponent)
	if !ok {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Nothing to post")}}
	}

	if len(container.Components) < 4 {
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Nothing to post")}}
	}
	// Copied rather than resliced, so appending below cannot clobber the
	// trailing separator and action row of the source message.
	body := slices.Clone(container.Components[2 : len(container.Components)-2])

	inv, err := database.GetSummaryInvocation(id)
	if err != nil {
		slog.Warn("Failed to fetch summary invocation for post", slog.Any("err", err))
	} else if inv.MessageID != "" {
		body = append(body,
			discord.SeparatorComponent{},
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("Original summary: https://discord.com/channels/%s/%s/%s", inv.GuildID, inv.ChannelID, inv.MessageID),
			},
		)
	}

	_, err = event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.MessageCreate{
		Flags:           discord.MessageFlagIsComponentsV2,
		Components:      []discord.LayoutComponent{discord.ContainerComponent{Components: body}},
		AllowedMentions: &discord.AllowedMentions{},
	})
	if err != nil {
		slog.Error("Failed to send follow-up", slog.Any("err", err))
		return []discord.LayoutComponent{discord.ContainerComponent{Components: errorComponents("Failed to post summary")}}
	}

	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{Content: "Summary posted to the channel."},
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

// invocationRows renders the header line and action buttons for a single
// invocation. The label is prefixed to the header so retries can be visually
// nested beneath the original attempt.
func invocationRows(inv database.SummaryInvocation, label string) []discord.ContainerSubComponent {
	rows := []discord.ContainerSubComponent{
		discord.TextDisplayComponent{
			Content: fmt.Sprintf("%s%s **%s** | channel `%s` | unit `%s` | `%s`",
				label, statusEmoji(inv.Status), inv.Status, inv.ChannelID, inv.Unit,
				inv.RequestedAt.Format("2006-01-02 15:04:05"),
			),
		},
	}
	actionRow := discord.ActionRowComponent{
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
	}
	if inv.Status == "success" {
		actionRow.Components = append(actionRow.Components, discord.ButtonComponent{
			Style:    discord.ButtonStyleSecondary,
			Label:    "View Response",
			CustomID: fmt.Sprintf("admin_summary_response_%s", inv.ID),
		})
	}
	return append(rows, actionRow)
}

// countComponents counts a slice of sub-components the way Discord does — every
// component plus the buttons nested inside any action rows.
func countComponents(comps []discord.ContainerSubComponent) int {
	n := 0
	for _, c := range comps {
		n++
		if ar, ok := c.(discord.ActionRowComponent); ok {
			n += len(ar.Components)
		}
	}
	return n
}

// appendRetries renders an original's retries nested beneath it, consuming from
// the shared retry budget. When the budget cannot fit every retry it shows as
// many as fit and replaces the remainder with a note, so the message never
// exceeds the component limit. It returns the updated rows and remaining budget.
func appendRetries(rows []discord.ContainerSubComponent, retries []database.SummaryInvocation, budget int) ([]discord.ContainerSubComponent, int) {
	total := 0
	for i := range retries {
		total += countComponents(invocationRows(retries[i], ""))
	}
	// Everything fits: render each retry in full.
	if total <= budget {
		for i, retry := range retries {
			rows = append(rows, invocationRows(retry, fmt.Sprintf("↳ Retry %d — ", i+1))...)
			budget -= countComponents(invocationRows(retry, ""))
		}
		return rows, budget
	}

	// Otherwise keep one slot in reserve for the "more not shown" note.
	shown := 0
	for i, retry := range retries {
		block := invocationRows(retry, fmt.Sprintf("↳ Retry %d — ", i+1))
		cost := countComponents(block)
		if cost > budget-1 {
			break
		}
		rows = append(rows, block...)
		budget -= cost
		shown = i + 1
	}
	if hidden := len(retries) - shown; hidden > 0 && budget >= 1 {
		rows = append(rows, discord.TextDisplayComponent{
			Content: fmt.Sprintf("↳ …and %d more retr%s not shown here (retry or download the original to inspect them)", hidden, plural(hidden, "y", "ies")),
		})
		budget--
	}
	return rows, budget
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func statusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "failed":
		return "❌"
	case "empty":
		return "⭕"
	default:
		return "⏳"
	}
}

func errorComponents(msg string) []discord.ContainerSubComponent {
	return []discord.ContainerSubComponent{
		discord.TextDisplayComponent{Content: msg},
	}
}
