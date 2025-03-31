package charts

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/database"
)

type ChartType string

const (
	BarChart     ChartType = "bar"
	PieChart     ChartType = "pie"
	LineChart    ChartType = "line"
	InvalidChart ChartType = "invalid"
)

// ChartData Basic count group for the max command
type ChartData struct {
	Xaxes  string  `json:"xAxes"`
	XLabel string  `json:"xLabel"`
	Value  float64 `json:"value"`
}

type ChartTracker struct {
	GuildID       string    `json:"guildID"`
	InteractionID string    `json:"interactionID"`
	UserID        string    `json:"userID"`
	ChartType     ChartType `json:"chart"`
	Metric        string    `json:"metrics"`
	Users         []string  `json:"users"`
	Channels      []string  `json:"channels"`
	DateRange     string    `json:"date"`
	GroupBy       string    `json:"groupBy"`
	ShowOptions   bool      `json:"showOptions"`
}

func (c *ChartTracker) GetChart() {
	switch c.ChartType {
	case BarChart:
	case PieChart:
	case LineChart:
	}
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
	default:
		return InvalidChart
	}
}

func (c *ChartTracker) getData(bot *discordgo.Session) (data []ChartData, err error) {
	query := `
	WITH latest_messages AS (
		SELECT *
		FROM (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC) AS rn
			FROM messages
		) t
		WHERE rn = 1
	)
	SELECT %s as xaxes, %s AS value
	FROM latest_messages
	WHERE guild_id = ?
	AND date BETWEEN ? AND ?
`

	queryCont := `
	%s
	GROUP BY %s
	ORDER BY value DESC;
`

	start, end, err := getDateRange(c.DateRange)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Determine aggregation
	var aggExpr string
	switch c.Metric {
	case "message_count":
		aggExpr = "COUNT(*)"
	case "avg_length":
		aggExpr = "AVG(LENGTH(content))"
	case "message_freq":
		aggExpr = fmt.Sprintf(
			"COUNT(*) * 1.0 / DATEDIFF('day', DATE '%s', DATE '%s')",
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)
	}

	// Determine grouping
	var groupField string
	switch c.GroupBy {
	case "user":
		groupField = "author_id"
	case "date":
		groupField = "strftime('%Y-%m-%d', date)"
	case "month":
		groupField = "strftime('%%Y-%%m', date)"
	case "channel":
		groupField = "channel_id"
	default:
		groupField = "author_id" // fallback
	}

	query = fmt.Sprintf(query, groupField, aggExpr)

	var filters []string
	if len(c.Users) > 0 {
		filters = append(filters, fmt.Sprintf(`author_id in (%s)`, strings.Join(c.Users, ", ")))
	}
	if len(c.Channels) > 0 {
		filters = append(filters, fmt.Sprintf(`channel_id in (%s)`, strings.Join(c.Channels, ", ")))
	}

	whereClause := ""
	if len(filters) > 0 {
		whereClause = fmt.Sprintf("AND %s", strings.Join(filters, " AND "))
	}

	query += fmt.Sprintf(queryCont, whereClause, groupField)

	rs, err := database.QueryDuckDB(query, []interface{}{c.GuildID, start, end})
	if err != nil {
		return nil, err
	}

	for rs.Next() {
		var xaxes string
		var value float64

		err = rs.Scan(&xaxes, &value)
		if err != nil {
			break
		}
		var label string
		switch c.GroupBy {
		case "user":
			mbr, _ := bot.GuildMember(c.GuildID, xaxes)
			label = mbr.User.Username
		case "channel":
			ch, _ := bot.Channel(xaxes)
			label = ch.Name
		case "date":
			label = xaxes
		}
		data = append(data, ChartData{
			Xaxes:  xaxes,
			XLabel: label,
			Value:  value,
		})
	}
	return
}
