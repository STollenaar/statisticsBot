package plotcommand

import (
	"fmt"
	"log"
	"strings"

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
		ShowOptions:   true,
	}
	cache[chartTracker.InteractionID] = chartTracker

	err := bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:    discordgo.MessageFlagsEphemeral,
			CustomID: "plot_modal",
			Title:    "Plotting modal form",
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "Create a Chart",
					Description: "Select the chart type, users to include, and provide a date range.",
					Color:       0x00bfff, // light blue
				},
			},
			Components: *chartTracker.BuildComponents(),
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func (p PlotCommand) embedHandler(bot *discordgo.Session, interaction *discordgo.InteractionCreate) {
	cID := strings.Split(interaction.Interaction.MessageComponentData().CustomID, ";")
	chartTracker := cache[interaction.Interaction.Message.Interaction.ID]

	bot.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content: "Loading...",
		},
	})

	if cID[0] != "submit_chart_form" {
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
			chartTracker.GroupBy = charts.GetGroupByType(interaction.Interaction.MessageComponentData().Values[0])
		case "filter_chart_form":
			chartTracker.ShowOptions = !chartTracker.ShowOptions
		}

		_, err := bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{
				{
					Title:       "Create a Chart",
					Description: "Select the chart type, users to include, and provide a date range.",
					Color:       0x00bfff, // light blue
				},
			},
			Components: chartTracker.BuildComponents(),
		})
		if err != nil {
			fmt.Println(err)
			e := "Error happened while processing selection"
			bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
				Content: &e,
			})
		}
	} else if !chartTracker.CanGenerate() {
		e := "Not all required options have been selected"
		bot.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
			Content: &e,
		})
	} else {
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
