package charts

import (
	"errors"
	"fmt"
	"strings"
)

type (
	ChartType   string
	GroupByType string
)

const (
	BarChart      ChartType = "bar"
	PieChart      ChartType = "pie"
	LineChart     ChartType = "line"
	SunburstChart ChartType = "sunburst"
	HeatmapChart  ChartType = "heatmap"
	InvalidChart  ChartType = "invalid"
)

type MetricType struct {
	Category string
	Metric   string
}

// ChartData Basic count group for the max command
type ChartData struct {
	Xaxes  string  `json:"xAxes"`
	Yaxes  string  `json:"yAxes"`
	XLabel string  `json:"xLabel"`
	YLabel string  `json:"yLabel"`
	Value  float64 `json:"value"`
}

type ChartTracker struct {
	GuildID       string      `json:"guildID"`
	InteractionID string      `json:"interactionID"`
	UserID        string      `json:"userID"`
	ChartType     ChartType   `json:"chart"`
	Metric        MetricType  `json:"metrics"`
	Users         []string    `json:"users"`
	Channels      []string    `json:"channels"`
	DateRange     string      `json:"date"`
	GroupBy       GroupByType `json:"groupBy"`
	ShowOptions   bool        `json:"showOptions"`
}

func (c *ChartTracker) Marshal() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", c.InteractionID, c.UserID, c.ChartType, strings.Join(c.Users, "-"), c.DateRange)
}

func (c *ChartTracker) Unmarshal(data []byte) error {
	d := strings.Split(string(data), "|")
	if len(d) != 5 {
		return errors.New("unknown data format")
	}
	c.InteractionID = d[0]
	c.UserID = d[1]
	c.ChartType = GetChartType(d[2])
	c.Users = strings.Split(d[3], "-")
	c.DateRange = d[4]

	return nil
}

func GetChartType(in string) ChartType {
	switch in {
	case "pie":
		return PieChart
	case "graph":
		return LineChart
	case "histogram":
		return BarChart
	case "sunburst":
		return SunburstChart
	case "heatmap":
		return HeatmapChart
	default:
		return InvalidChart
	}
}

func (g *GroupByType) ToString() string {
	return string(*g)
}

func (m *MetricType) ToString() string {
	return fmt.Sprintf("%s_%s", m.Category, m.Metric)
}

func GetMetricType(in string) MetricType {
	cat, metric := strings.Split(in, "_")[0], strings.Join(strings.Split(in, "_")[1:], "_")
	return MetricType{Category: cat, Metric: metric}
}

func GetGroupByType(in string) GroupByType {
	return GroupByType(in)
}
