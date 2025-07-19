package plotcommand

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/util"
	"github.com/stollenaar/statisticsbot/internal/util/charts"
)

var (
	PlotCmd = PlotCommand{
		Name:        "plot",
		Description: "Returns a plotted chart",
	}
	cache = make(map[string]*charts.ChartTracker)
)

type PlotCommand struct {
	Name        string
	Description string
}

// CommandParsed parsed struct for count command
type CommandParsed struct {
}

func (p PlotCommand) Handler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type == discordgo.InteractionType(discordgo.InteractionApplicationCommand) {
		p.interactionHandler(bot, interaction)
	} else {
		p.embedHandler(bot, interaction)
	}
}

func (p PlotCommand) ModalHandler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {

	chartTracker := cache[interaction.Interaction.Message.Interaction.ID]
	submittedData := extractModalSubmitData(interaction.ModalSubmitData().Components)
	errorCode := 0xff3300
	errors := make(map[string][]discordgo.MessageComponent)
	errContainer := discordgo.Container{
		AccentColor: &errorCode,
	}
	beforeStart, beforeEnd := chartTracker.CustomDateRange.Start, chartTracker.CustomDateRange.End

	if start, err := time.Parse("2006-01-02", submittedData["start_date"]); err == nil {
		chartTracker.CustomDateRange.Start = &start
	} else {
		errContainer.Components = append(errContainer.Components,
			discordgo.TextDisplay{
				Content: fmt.Sprintf("Error setting start date: %s", err),
			},
		)
	}

	if end, err := time.Parse("2006-01-02", submittedData["end_date"]); err == nil {
		chartTracker.CustomDateRange.End = &end
	} else {
		errContainer.Components = append(errContainer.Components,
			discordgo.TextDisplay{
				Content: fmt.Sprintf("Error setting end date: %s", err),
			},
		)
	}
	if chartTracker.CustomDateRange.Start != nil && chartTracker.CustomDateRange.End != nil && chartTracker.CustomDateRange.Start.After(*chartTracker.CustomDateRange.End) {
		errContainer.Components = append(errContainer.Components,
			discordgo.TextDisplay{
				Content: "Error setting start and end date. Start date cannot be after end",
			},
		)
		chartTracker.CustomDateRange.End = beforeEnd
		chartTracker.CustomDateRange.Start = beforeStart
	}
	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading...",
		},
	})
	if len(errContainer.Components) > 0 {
		errors["custom_date"] = append(errors["custom_date"], errContainer)
	}
	p.displayPlotSelection(bot, interaction, chartTracker, errors)
}

func (p PlotCommand) CreateCommandArguments() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{}
}
func (p PlotCommand) ParseArguments(bot *discordgo.Session, interaction *discordgo.InteractionCreate) interface{} {
	// parsedArguments := new(CommandParsed)
	return nil
}

func (p PlotCommand) interactionHandler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	chartTracker := &charts.ChartTracker{
		GuildID:       interaction.GuildID,
		InteractionID: interaction.Interaction.ID,
		UserID:        interaction.Member.User.ID,
	}
	cache[chartTracker.InteractionID] = chartTracker

	title := discordgo.TextDisplay{
		Content: "# Create a Chart\n Select the chart type, users to include, and provide a date range.",
	}
	components := []discordgo.MessageComponent{
		title,
		util.GetSeparator(),
	}
	components = append(components, chartTracker.BuildComponents(make(map[string][]discordgo.MessageComponent))...)
	err := bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			CustomID:   "plot_modal",
			Title:      "Plotting modal form",
			Components: components,
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func (p PlotCommand) embedHandler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	cID := strings.Split(interaction.Interaction.MessageComponentData().CustomID, ";")
	chartTracker := cache[interaction.Interaction.Message.Interaction.ID]

	fmt.Println(cID[0])
	if cID[0] == "custom_date_range" {
		var startDate, endDate string
		if chartTracker.CustomDateRange.Start != nil {
			startDate = chartTracker.CustomDateRange.Start.Format("2006-01-02")
		}
		if chartTracker.CustomDateRange.End != nil {
			endDate = chartTracker.CustomDateRange.End.Format("2006-01-02")
		}

		err := bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "custom_date",
				Title:    "Submit Custom Start and End Date",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "start_date",
								Label:       "Start Date (YYYY-MM-DD)",
								Style:       discordgo.TextInputParagraph,
								Placeholder: "e.g., 2024-01-01",
								Value:       startDate,
								Required:    true,
								MinLength:   10,
								MaxLength:   10,
							},
						},
					},
					// 4. End Date Input
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "end_date",
								Label:       "End Date (YYYY-MM-DD)",
								Style:       discordgo.TextInputParagraph,
								Placeholder: "e.g., 2024-12-31",
								Value:       endDate,
								Required:    true,
								MinLength:   10,
								MaxLength:   10,
							},
						},
					},
				},
			},
		})
		if err != nil {
			fmt.Println(err)
		}
	} else if cID[0] != "submit_chart_form" {
		bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Content: "Loading...",
			},
		})

		switch cID[0] {
		case "chart_type":
			chartTracker.ChartType = charts.GetChartType(interaction.Interaction.MessageComponentData().Values[0])
		case "metric_type":
			chartTracker.Metric = charts.GetMetricType(interaction.Interaction.MessageComponentData().Values[0])
		case "user_select":
			chartTracker.Users = interaction.Interaction.MessageComponentData().Values
		case "channel_select":
			chartTracker.Channels = interaction.Interaction.MessageComponentData().Values
		case "date_range_select":
			chartTracker.DateRange = interaction.Interaction.MessageComponentData().Values[0]
		case "group_by":
			chartTracker.GroupBy = charts.GetMetricType(interaction.Interaction.MessageComponentData().Values[0])
		}

		p.displayPlotSelection(bot, interaction, chartTracker, make(map[string][]discordgo.MessageComponent))
	} else if !chartTracker.CanGenerate() {
		bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Content: "Loading...",
			},
		})

		e := "Not all required options have been selected"
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &e,
		})
	} else {
		bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Content: "Loading...",
			},
		})

		chart, err := chartTracker.GenerateChart(bot)
		if err != nil {
			fmt.Println(err)
			e := "Error happened while processing selection"
			bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
				Content: &e,
			})
		} else {
			_, err = bot.FollowupMessageCreate(interaction.Interaction, false, &discordgo.WebhookParams{
				Files: []*discordgo.File{chart},
				Flags: util.ConfigFile.SetEphemeral(),
			})
			if err != nil {
				fmt.Println(err)
			}
			err = bot.InteractionResponseDelete(interaction.Interaction)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func extractModalSubmitData(components []discordgo.MessageComponent) map[string]string {
	formData := make(map[string]string)
	for _, component := range components {
		input := component.(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
		formData[input.CustomID] = input.Value
	}
	return formData
}

func (p PlotCommand) displayPlotSelection(bot *discordgo.Session, interaction *discordgo.InteractionCreate, chartTracker *charts.ChartTracker, errors map[string][]discordgo.MessageComponent) {
	var files []*discordgo.File
	title := discordgo.TextDisplay{
		Content: "# Create a Chart\n Select the chart type, users to include, and provide a date range.",
	}

	components := append(errors["main"],
		title,
		util.GetSeparator(),
	)
	if chartTracker.CanGenerate() {
		chart, err := chartTracker.GenerateChart(bot)
		if err != nil {
			fmt.Println(err)
			components = append(components, discordgo.TextDisplay{
				Content: "Error happened while processing selection",
			})
		} else {
			files = append(files, chart)
			components = []discordgo.MessageComponent{
				discordgo.Section{
					Components: []discordgo.MessageComponent{
						title,
					},
					Accessory: discordgo.Thumbnail{
						Media: discordgo.UnfurledMediaItem{
							URL: fmt.Sprintf("attachment://%s", chart.Name),
						},
					},
				},
				util.GetSeparator(),
			}
		}
	}

	components = append(components, chartTracker.BuildComponents(errors)...)

	_, err := bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Components:  &components,
		Files:       files,
		Attachments: &[]*discordgo.MessageAttachment{},
	})
	if err != nil {
		fmt.Println(err)
		e := "Error happened while processing selection"
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &e,
		})
	}
}
