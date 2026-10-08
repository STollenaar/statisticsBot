package plotcommand

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/util"
	"github.com/stollenaar/statisticsbot/internal/util/charts"
)

const (
	// trackerTTL is how long a /plot form's selections stay cached. Discord stops
	// accepting an interaction token after 15 minutes, so a tracker that outlives
	// that can never be acted on again and is only holding memory.
	trackerTTL = 15 * time.Minute

	plotExpiredMessage = "This chart form has expired, please run `/plot` again"
)

var (
	PlotCmd = PlotCommand{
		Name:        "plot",
		Description: "Returns a plotted chart",
	}

	// cache holds each open form's selections, keyed by the id of the
	// interaction that opened it. Entries are pruned on access rather than by a
	// timer, which is the pattern the semantic command's sessions use.
	cache   = make(map[string]*cacheEntry)
	cacheMu sync.Mutex
)

type cacheEntry struct {
	tracker *charts.ChartTracker
	created time.Time
}

// storeTracker caches a form's state and prunes any that have expired.
func storeTracker(id string, tracker *charts.ChartTracker) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	pruneLocked()
	cache[id] = &cacheEntry{tracker: tracker, created: time.Now()}
}

// getTracker returns a cached form, pruning expired ones first. It reports false
// when the form has expired or the bot restarted since it was opened — both of
// which used to return a nil tracker that the handlers then dereferenced.
func getTracker(id string) (*charts.ChartTracker, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	pruneLocked()
	entry, ok := cache[id]
	if !ok {
		return nil, false
	}
	return entry.tracker, true
}

// pruneLocked drops trackers older than trackerTTL. Callers must hold cacheMu.
func pruneLocked() {
	cutoff := time.Now().Add(-trackerTTL)
	for k, v := range cache {
		if v.created.Before(cutoff) {
			delete(cache, k)
		}
	}
}

// trackerKey is the cache key for the interaction a component or modal belongs
// to.
//
// Both pointers really can be nil: a modal submitted from a slash command rather
// than from a component carries no message at all, and Message.Interaction is
// nil on any message that did not originate from an interaction. An empty key
// simply misses the cache, which the callers already handle.
func trackerKey(msg *discord.Message) string {
	if msg == nil || msg.Interaction == nil {
		return ""
	}
	return msg.Interaction.ID.String()
}

type PlotCommand struct {
	Name        string
	Description string
}

// CommandParsed parsed struct for count command
type CommandParsed struct {
}

func (p PlotCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	if event.Data.Type() == discord.ApplicationCommandTypeSlash {
		p.interactionHandler(event)
	}
}

func (p PlotCommand) ModalHandler(event *events.ModalSubmitInteractionCreate) {

	chartTracker, ok := getTracker(trackerKey(event.Message))
	if !ok {
		if err := event.CreateMessage(discord.MessageCreate{
			Content: plotExpiredMessage,
			Flags:   discord.MessageFlagEphemeral,
		}); err != nil {
			slog.Error("plot error", slog.Any("err", err))
		}
		return
	}
	submittedData := extractModalSubmitData(event.ModalSubmitInteraction.Data.Components)
	errorCode := 0xff3300
	errors := make(map[string][]discord.LayoutComponent)
	errContainer := discord.ContainerComponent{
		AccentColor: errorCode,
	}
	beforeStart, beforeEnd := chartTracker.CustomDateRange.Start, chartTracker.CustomDateRange.End

	if start, err := time.Parse("2006-01-02", submittedData["start_date"]); err == nil {
		chartTracker.CustomDateRange.Start = &start
	} else {
		errContainer.Components = append(errContainer.Components,
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("Error setting start date: %s", err),
			},
		)
	}

	if end, err := time.Parse("2006-01-02", submittedData["end_date"]); err == nil {
		chartTracker.CustomDateRange.End = &end
	} else {
		errContainer.Components = append(errContainer.Components,
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("Error setting end date: %s", err),
			},
		)
	}
	if chartTracker.CustomDateRange.Start != nil && chartTracker.CustomDateRange.End != nil && chartTracker.CustomDateRange.Start.After(*chartTracker.CustomDateRange.End) {
		errContainer.Components = append(errContainer.Components,
			discord.TextDisplayComponent{
				Content: "Error setting start and end date. Start date cannot be after end",
			},
		)
		chartTracker.CustomDateRange.End = beforeEnd
		chartTracker.CustomDateRange.Start = beforeStart
	}
	event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if len(errContainer.Components) > 0 {
		errors["custom_date"] = append(errors["custom_date"], errContainer)
	}
	p.displayPlotSelection(event.GenericEvent, event.Token(), chartTracker, errors)
}

func (p PlotCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{}
}

func (p PlotCommand) interactionHandler(event *events.ApplicationCommandInteractionCreate) {
	chartTracker := &charts.ChartTracker{
		GuildID:       event.GuildID().String(),
		InteractionID: event.ID().String(),
		UserID:        event.User().ID.String(),
	}
	storeTracker(chartTracker.InteractionID, chartTracker)

	title := discord.TextDisplayComponent{
		Content: "# Create a Chart\n Select the chart type, users to include, and provide a date range.",
	}
	components := []discord.LayoutComponent{
		title,
		util.GetSeparator(),
	}
	components = append(components, chartTracker.BuildComponents(make(map[string][]discord.LayoutComponent))...)

	err := event.CreateMessage(discord.MessageCreate{
		Flags: discord.MessageFlagEphemeral | discord.MessageFlagIsComponentsV2,
		// CustomID:   "plot_modal",
		// Title:      "Plotting modal form",
		Components: components,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
	if err != nil {
		slog.Error("plot error", slog.Any("err", err))
	}
}

func (p PlotCommand) ComponentHandler(event *events.ComponentInteractionCreate) {

	cID := strings.Split(event.ComponentInteraction.Data.CustomID(), ";")
	chartTracker, ok := getTracker(trackerKey(&event.Message))
	if !ok {
		// Answer before any other branch responds: an interaction may only be
		// responded to once.
		if err := event.CreateMessage(discord.MessageCreate{
			Content: plotExpiredMessage,
			Flags:   discord.MessageFlagEphemeral,
		}); err != nil {
			slog.Error("plot error", slog.Any("err", err))
		}
		return
	}

	if cID[0] == "custom_date_range" {
		var startDate, endDate string
		if chartTracker.CustomDateRange.Start != nil {
			startDate = chartTracker.CustomDateRange.Start.Format("2006-01-02")
		}
		if chartTracker.CustomDateRange.End != nil {
			endDate = chartTracker.CustomDateRange.End.Format("2006-01-02")
		}

		err := event.Modal(discord.ModalCreate{
			CustomID: "custom_date",
			Title:    "Submit Custom Start and End Date",
			Components: []discord.LayoutComponent{
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						discord.TextInputComponent{
							CustomID: "start_date",
							// Label:       "Start Date (YYYY-MM-DD)",
							Style:       discord.TextInputStyleParagraph,
							Placeholder: "e.g., 2024-01-01",
							Value:       startDate,
							Required:    true,
							MinLength:   util.Pointer(10),
							MaxLength:   10,
						},
					},
				},
				// 4. End Date Input
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						discord.TextInputComponent{
							CustomID: "end_date",
							// Label:       "End Date (YYYY-MM-DD)",
							Style:       discord.TextInputStyleParagraph,
							Placeholder: "e.g., 2024-12-31",
							Value:       endDate,
							Required:    true,
							MinLength:   util.Pointer(10),
							MaxLength:   10,
						},
					},
				},
			},
		})
		if err != nil {
			slog.Error("plot error", slog.Any("err", err))
		}
	} else if cID[0] != "submit_chart_form" {
		event.DeferUpdateMessage()

		switch cID[0] {
		case "chart_type":
			chartTracker.ChartType = charts.GetChartType(event.StringSelectMenuInteractionData().Values[0])
		case "metric_type":
			chartTracker.Metric = charts.GetMetricType(event.StringSelectMenuInteractionData().Values[0])
		case "user_select":
			chartTracker.Users = toString(event.UserSelectMenuInteractionData().Values)
		case "channel_select":
			chartTracker.Channels = toString(event.ChannelSelectMenuInteractionData().Values)
		case "date_range_select":
			chartTracker.DateRange = event.StringSelectMenuInteractionData().Values[0]
		case "group_by":
			chartTracker.GroupBy = charts.GetMetricType(event.StringSelectMenuInteractionData().Values[0])
		}

		p.displayPlotSelection(event.GenericEvent, event.Token(), chartTracker, make(map[string][]discord.LayoutComponent))
	} else if !chartTracker.CanGenerate() {
		event.DeferUpdateMessage()
		e := "Not all required options have been selected"
		_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Content: &e,
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err))
		}
	} else {
		event.DeferUpdateMessage()
		chart, err := chartTracker.GenerateChart(event.Client())
		if err != nil {
			slog.Error("plot error", slog.Any("err", err))
			e := "Error happened while processing selection"
			_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
				Content: &e,
			})
			if err != nil {
				slog.Error("Error editing the response:", slog.Any("err", err))
			}

		} else {
			err = event.Client().Rest.DeleteInteractionResponse(event.Client().ApplicationID, event.Token())
			if err != nil {
				slog.Error("plot error", slog.Any("err", err))
			}
			_, err = event.Client().Rest.CreateFollowupMessage(event.Client().ApplicationID, event.Token(), discord.MessageCreate{
				Files: []*discord.File{chart},
				Flags: util.ConfigFile.SetEphemeral(),
			})
			if err != nil {
				slog.Error("plot error", slog.Any("err", err))
			}
		}
	}
}

func extractModalSubmitData(components []discord.LayoutComponent) map[string]string {
	formData := make(map[string]string)
	for _, component := range components {
		input := component.(*discord.ActionRowComponent).Components[0].(*discord.TextInputComponent)
		formData[input.CustomID] = input.Value
	}
	return formData
}

func (p PlotCommand) displayPlotSelection(event *events.GenericEvent, token string, chartTracker *charts.ChartTracker, errors map[string][]discord.LayoutComponent) {
	var files []*discord.File
	title := discord.TextDisplayComponent{
		Content: "# Create a Chart\n Select the chart type, users to include, and provide a date range.",
	}

	components := append(errors["main"],
		title,
		util.GetSeparator(),
	)
	if chartTracker.CanGenerate() {
		chart, err := chartTracker.GenerateChart(event.Client())
		if err != nil {
			slog.Error("plot error", slog.Any("err", err))
			components = append(components, discord.TextDisplayComponent{
				Content: "Error happened while processing selection",
			})
		} else {
			files = append(files, chart)
			components = []discord.LayoutComponent{
				discord.SectionComponent{
					Components: []discord.SectionSubComponent{
						title,
					},
					Accessory: discord.ThumbnailComponent{
						Media: discord.UnfurledMediaItem{
							URL: fmt.Sprintf("attachment://%s", chart.Name),
						},
					},
				},
				util.GetSeparator(),
			}
		}
	}

	components = append(components, chartTracker.BuildComponents(errors)...)

	_, err := event.Client().Rest.UpdateInteractionResponse(event.Client().ApplicationID, token, discord.MessageUpdate{
		Components:  &components,
		Files:       files,
		Attachments: &[]discord.AttachmentUpdate{},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func toString(in []snowflake.ID) (out []string) {
	for _, i := range in {
		out = append(out, i.String())
	}
	return
}
