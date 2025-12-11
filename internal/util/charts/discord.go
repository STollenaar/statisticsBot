package charts

import (
	"errors"
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/util"
)

// getDateRange returns the start and end timestamps based on the selected date range
func (c *ChartTracker) getDateRange() (time.Time, time.Time, error) {
	now := time.Now()
	switch c.DateRange {
	case "7d":
		return now.AddDate(0, 0, -7), now, nil
	case "30d":
		return now.AddDate(0, 0, -30), now, nil
	case "year":
		yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return yearStart, now, nil
	case "custom":
		return *c.CustomDateRange.Start, *c.CustomDateRange.End, nil
	default:
		return time.Time{}, time.Time{}, errors.New("unsupported date range selection")
	}
}

func (c *ChartTracker) getSelectMenuDefaultValue(st discord.SelectMenuDefaultValueType) (response []discord.SelectMenuDefaultValue) {
	switch st {
	case discord.SelectMenuDefaultValueTypeUser:
		for _, i := range c.Users {
			response = append(response, discord.SelectMenuDefaultValue{
				ID:   snowflake.MustParse(i),
				Type: discord.SelectMenuDefaultValueTypeUser,
			})
		}
	case discord.SelectMenuDefaultValueTypeChannel:
		for _, i := range c.Channels {
			response = append(response, discord.SelectMenuDefaultValue{
				ID:   snowflake.MustParse(i),
				Type: discord.SelectMenuDefaultValueTypeChannel,
			})
		}
	}
	return
}

func (c *ChartTracker) BuildComponents(errors map[string][]discord.LayoutComponent) []discord.LayoutComponent {
	var components []discord.LayoutComponent
	components = append(components,
		discord.TextDisplayComponent{
			Content: "Required Settings",
		},
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.StringSelectMenuComponent{
					CustomID:    "chart_type",
					Placeholder: "Select chart type",
					Options: []discord.StringSelectMenuOption{
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
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.StringSelectMenuComponent{
					CustomID:    "metric_type",
					Placeholder: "Choose a metric to chart...",
					Options: []discord.StringSelectMenuOption{
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

	components = append(components, c.getDate()...)
	if c.DateRange == "custom" {
		components = append(components, errors["custom_date"]...)
		components = append(components, c.getCustomDate()...)
	}

	components = append(components, c.getOptionalSettings()...)

	components = append(components, discord.ActionRowComponent{
		Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Label:    "Submit",
				Style:    discord.ButtonStylePrimary,
				CustomID: "submit_chart_form",
			},
		},
	})

	return components
}

func (c *ChartTracker) getOptionalSettings() []discord.LayoutComponent {
	return []discord.LayoutComponent{
		discord.TextDisplayComponent{
			Content: "Optional Settings",
		},
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.UserSelectMenuComponent{
					CustomID:      "user_select",
					Placeholder:   "Select users for the chart",
					MaxValues:     5, // or however many users you want to allow
					DefaultValues: c.getSelectMenuDefaultValue(discord.SelectMenuDefaultValueTypeUser),
				},
			},
		},
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.ChannelSelectMenuComponent{
					CustomID:      "channel_select",
					Placeholder:   "Select channels for the chart",
					MaxValues:     5, // or however many users you want to allow
					DefaultValues: c.getSelectMenuDefaultValue(discord.SelectMenuDefaultValueTypeChannel),
					ChannelTypes: []discord.ChannelType{
						discord.ChannelTypeGuildText,
					},
				},
			},
		},
		util.GetSeparator(),
	}
}

func (c *ChartTracker) getDate() []discord.LayoutComponent {
	return []discord.LayoutComponent{
		discord.ActionRowComponent{
			Components: []discord.InteractiveComponent{
				discord.StringSelectMenuComponent{
					CustomID:    "date_range_select",
					Placeholder: "Select a Date Range",
					Options: []discord.StringSelectMenuOption{
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
						{
							Label:   "Custom Range",
							Value:   "custom",
							Default: c.DateRange == "custom",
						},
					},
				},
			},
		},
		util.GetSeparator(),
	}
}

func (c *ChartTracker) getCustomDate() []discord.LayoutComponent {
	startDate, endDate := "Not Set", "Not Set"
	if c.CustomDateRange.Start != nil {
		startDate = c.CustomDateRange.Start.Format("2006-01-02")
	}
	if c.CustomDateRange.End != nil {
		endDate = c.CustomDateRange.End.Format("2006-01-02")
	}

	return []discord.LayoutComponent{
		discord.SectionComponent{
			Components: []discord.SectionSubComponent{
				discord.TextDisplayComponent{
					Content: fmt.Sprintf("### Start Date: %s", startDate),
				},
				discord.TextDisplayComponent{
					Content: fmt.Sprintf("### End Date: %s", endDate),
				},
			},
			Accessory: discord.ButtonComponent{
				Label:    "Set Date Range",
				CustomID: "custom_date_range",
				Style:    discord.ButtonStylePrimary,
			},
		},
		util.GetSeparator(),
	}
}

func (c *ChartTracker) getGroupBy() *discord.ActionRowComponent {

	var options []discord.StringSelectMenuOption

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

	return &discord.ActionRowComponent{
		Components: []discord.InteractiveComponent{
			discord.StringSelectMenuComponent{
				CustomID:    "group_by",
				Placeholder: "Group chart data by...",
				Options:     options,
			},
		},
	}
}

func (c *ChartTracker) getSingleGroupBy() []discord.StringSelectMenuOption {
	switch c.Metric.Category {
	case "interaction":
		return []discord.StringSelectMenuOption{
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
		return []discord.StringSelectMenuOption{
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
		return []discord.StringSelectMenuOption{}
	}
}

func (c *ChartTracker) getMultiGroupBy() 	[]discord.StringSelectMenuOption {
	switch c.Metric.Category {
	case "message":
		c.GroupBy = MetricType{Category: "channel", Metric: "user", MultiAxes: true}
		return []discord.StringSelectMenuOption{
			{
				Label:       "Channel & User",
				Value:       "channel;user;true",
				Description: "Group results by channel and user (author)",
				Default:     true,
			},
		}
	case "reaction":
		return []discord.StringSelectMenuOption{
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
		return []discord.StringSelectMenuOption{
			{
				Label:       "Bot & User",
				Value:       "interaction;user;true",
				Description: "Group results by bot and user (author)",
				Default:     true,
			},
		}
	default:
		return []discord.StringSelectMenuOption{}
	}
}

func isOption(selected MetricType, options []discord.StringSelectMenuOption) bool {
	for _, option := range options {
		if option.Value == selected.ToString() {
			return true
		}
	}
	return false
}
