package charts

import (
	"errors"
	"time"

	"github.com/bwmarrin/discordgo"
)

// getDateRange returns the start and end timestamps based on the selected date range
func getDateRange(selection string) (time.Time, time.Time, error) {
	now := time.Now()
	switch selection {
	case "7d":
		return now.AddDate(0, 0, -7), now, nil
	case "30d":
		return now.AddDate(0, 0, -30), now, nil
	case "year":
		yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return yearStart, now, nil
	default:
		return time.Time{}, time.Time{}, errors.New("unsupported date range selection")
	}
}

func (c *ChartTracker) getSelectMenuDefaultValue(st discordgo.SelectMenuType) (response []discordgo.SelectMenuDefaultValue) {
	switch st {
	case discordgo.UserSelectMenu:
		for _, i := range c.Users {
			response = append(response, discordgo.SelectMenuDefaultValue{
				ID:   i,
				Type: discordgo.SelectMenuDefaultValueUser,
			})
		}
	case discordgo.ChannelSelectMenu:
		for _, i := range c.Channels {
			response = append(response, discordgo.SelectMenuDefaultValue{
				ID:   i,
				Type: discordgo.SelectMenuDefaultValueChannel,
			})
		}
	}
	return
}

func (c *ChartTracker) BuildComponents() *[]discordgo.MessageComponent {
	if c.ShowOptions {
		var components []discordgo.MessageComponent
		components = append(components,
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{

						MenuType:    discordgo.StringSelectMenu,
						CustomID:    "chart_type",
						Placeholder: "Select chart type",
						Options: []discordgo.SelectMenuOption{
							{
								Label:   "Pie",
								Value:   "pie",
								Default: c.ChartType == PieChart,
							},
							{
								Label:   "Graph",
								Value:   "graph",
								Default: c.ChartType == LineChart,
							},
							{
								Label:   "Histogram",
								Value:   "histogram",
								Default: c.ChartType == BarChart,
							},
							{
								Label:   "Sunburst",
								Value:   "sunburst",
								Default: c.ChartType == SunburstChart,
							},
							{
								Label:   "Heatmap",
								Value:   "heatmap",
								Default: c.ChartType == HeatmapChart,
							},
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "metric_type",
						Placeholder: "Choose a metric to chart...",
						Options: []discordgo.SelectMenuOption{
							{
								Label:       "Reaction Count",
								Value:       "reaction;count",
								Description: "How many times a reaction was used",
								Default:     c.Metric == MetricType{Category: "reaction", Metric: "count"},
							},
							{
								Label:       "Message Count",
								Value:       "message;count",
								Description: "How many messages are sent",
								Default:     c.Metric == MetricType{Category: "message", Metric: "count"},
							},
							{
								Label:       "Avg. Message Length",
								Value:       "message;avg_length",
								Description: "Average length of each message",
								Default:     c.Metric == MetricType{Category: "message", Metric: "avg_length"},
							},
							{
								Label:       "Message Frequency",
								Value:       "message;freq",
								Description: "Number of messages per day",
								Default:     c.Metric == MetricType{Category: "message", Metric: "freq"},
							},
							{
								Label:       "Bot interaction count",
								Value:       "interaction;count",
								Description: "How many times a bot has been interacted with",
								Default:     c.Metric == MetricType{Category: "interaction", Metric: "count"},
							},
							// {Label: "Mentions Received", Value: "mentions", Description: "Times the user was mentioned"},
							// {Label: "Reactions Received", Value: "reactions", Description: "Reactions per user (if available)"},
						},
					},
				},
			},
		)

		groupedBy := c.getGroupBy()

		if groupedBy != nil {
			components = append(components, *groupedBy)
		}
		components = append(components,
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "date_range_select",
						Placeholder: "Select a Date Range",
						Options: []discordgo.SelectMenuOption{
							{
								Label:   "Last 7 days",
								Value:   "7d",
								Default: c.DateRange == "7d",
							},
							{
								Label:   "Last 30 days",
								Value:   "30d",
								Default: c.DateRange == "30d",
							},
							{
								Label:   "This Year",
								Value:   "year",
								Default: c.DateRange == "year",
							},
							// {
							// 	Label:   "Custom Range",
							// 	Value:   "custom",
							// 	Default: c.DateRange == "custom",
							// },
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Submit",
						Style:    discordgo.PrimaryButton,
						CustomID: "submit_chart_form",
					},
					discordgo.Button{
						Label:    "Filter",
						Style:    discordgo.SecondaryButton,
						CustomID: "filter_chart_form",
					},
				},
			},
		)
		return &components
	} else {
		return &[]discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:      "user_select",
						MenuType:      discordgo.UserSelectMenu,
						Placeholder:   "Select users for the chart",
						MaxValues:     5, // or however many users you want to allow
						DefaultValues: c.getSelectMenuDefaultValue(discordgo.UserSelectMenu),
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:      "channel_select",
						MenuType:      discordgo.ChannelSelectMenu,
						Placeholder:   "Select channels for the chart",
						MaxValues:     5, // or however many users you want to allow
						DefaultValues: c.getSelectMenuDefaultValue(discordgo.ChannelSelectMenu),
						ChannelTypes: []discordgo.ChannelType{
							discordgo.ChannelTypeGuildText,
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Submit",
						Style:    discordgo.PrimaryButton,
						CustomID: "submit_chart_form",
					},
					discordgo.Button{
						Label:    "Options",
						Style:    discordgo.SecondaryButton,
						CustomID: "filter_chart_form",
					},
				},
			},
		}
	}
	// // 3. Start Date Input
	// discordgo.ActionsRow{
	// 	Components: []discordgo.MessageComponent{
	// 		discordgo.TextInput{
	// 			CustomID:    "start_date",
	// 			Label:       "Start Date (YYYY-MM-DD)",
	// 			Style:       discordgo.TextInputParagraph,
	// 			Placeholder: "e.g., 2024-01-01",
	// 			Required:     true,
	// 			MinLength:   10,
	// 			MaxLength:   10,
	// 		},
	// 	},
	// },
	// // 4. End Date Input
	// discordgo.ActionsRow{
	// 	Components: []discordgo.MessageComponent{
	// 		discordgo.TextInput{
	// 			CustomID:    "end_date",
	// 			Label:       "End Date (YYYY-MM-DD)",
	// 			Style:       discordgo.TextInputParagraph,
	// 			Placeholder: "e.g., 2024-12-31",
	// 			Required:    true,
	// 			MinLength:   10,
	// 			MaxLength:   10,
	// 		},
	// 	},
	// },
}

func (c *ChartTracker) getGroupBy() *discordgo.ActionsRow {

	var options []discordgo.SelectMenuOption

	switch c.ChartType {
	default:
		fallthrough
	case PieChart:
		fallthrough
	case BarChart:
		fallthrough
	case LineChart:
		if c.GroupBy == (MetricType{Category: "channel", Metric: "user", MultiAxes: true}) {
			c.GroupBy = MetricType{}
		}
		options = c.getSingleGroupBy()
	case SunburstChart:
		fallthrough
	case HeatmapChart:
		options = c.getMultiGroupBy()
		if !isOption(c.GroupBy, options) {
			c.GroupBy = MetricType{}
		}
	}
	if len(options) == 0 {
		return nil
	}

	return &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    "group_by",
				Placeholder: "Group chart data by...",
				Options:     options,
			},
		},
	}
}

func (c *ChartTracker) getSingleGroupBy() []discordgo.SelectMenuOption {
	switch c.Metric.Category {
	case "interaction":
		return []discordgo.SelectMenuOption{
			{
				Label:       "User",
				Value:       "interaction;user",
				Description: "Group results by initiator (author)",
				Default:     c.GroupBy == MetricType{Category: "interaction", Metric: "user"},
			},
			{
				Label:       "Bot",
				Value:       "interaction;bot",
				Description: "Group results by the bot",
				Default:     c.GroupBy == MetricType{Category: "interaction", Metric: "bot"},
			},
		}
	case "message":
		fallthrough
	case "reaction":
		return []discordgo.SelectMenuOption{
			{
				Label:       "User",
				Value:       "single;user",
				Description: "Group results by user (author)",
				Default:     c.GroupBy == MetricType{Category: "single", Metric: "user"},
			},
			{
				Label:       "Date",
				Value:       "single;date",
				Description: "Group results by individual day",
				Default:     c.GroupBy == MetricType{Category: "single", Metric: "date"},
			},
			{
				Label:       "Channel",
				Value:       "single;channel",
				Description: "Group results by channel",
				Default:     c.GroupBy == MetricType{Category: "single", Metric: "channel"},
			},
		}
	default:
		return []discordgo.SelectMenuOption{}
	}
}

func (c *ChartTracker) getMultiGroupBy() []discordgo.SelectMenuOption {
	switch c.Metric.Category {
	case "message":
		c.GroupBy = MetricType{Category: "channel", Metric: "user", MultiAxes: true}
		return []discordgo.SelectMenuOption{
			{
				Label:       "Channel & User",
				Value:       "channel;user;true",
				Description: "Group results by channel and user (author)",
				Default:     true,
			},
		}
	case "reaction":
		return []discordgo.SelectMenuOption{
			{
				Label:       "Reaction & User",
				Value:       "reaction;user;true",
				Description: "Group results by emoji and user (author)",
				Default:     c.GroupBy == MetricType{Category: "reaction", Metric: "user", MultiAxes: true},
			},
			{
				Label:       "Reaction & Channelr",
				Value:       "reaction;channel;true",
				Description: "Group results by emoji and channel",
				Default:     c.GroupBy == MetricType{Category: "reaction", Metric: "channel", MultiAxes: true},
			},
		}
	case "interaction":
		c.GroupBy = MetricType{Category: "interaction", Metric: "user", MultiAxes: true}
		return []discordgo.SelectMenuOption{
			{
				Label:       "Bot & User",
				Value:       "interaction;user;true",
				Description: "Group results by bot and user (author)",
				Default:     true,
			},
		}
	default:
		return []discordgo.SelectMenuOption{}
	}
}

func isOption(selected MetricType, options []discordgo.SelectMenuOption) bool {
	for _, option := range options {
		if option.Value == selected.ToString() {
			return true
		}
	}
	return false
}
