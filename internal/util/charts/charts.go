package charts

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

type ChartType string

const (
	BarChart      ChartType = "bar"
	PieChart      ChartType = "pie"
	LineChart     ChartType = "line"
	SunburstChart ChartType = "sunburst"
	HeatmapChart  ChartType = "heatmap"
	InvalidChart  ChartType = "invalid"
)

// ChartData Basic count group for the max command
type ChartData struct {
	Xaxes  string  `json:"xAxes"`
	Yaxes  string  `json:"yAxes"`
	XLabel string  `json:"xLabel"`
	YLabel string  `json:"yLabel"`
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

func (c *ChartTracker) getData(bot *discordgo.Session) (data []ChartData, err error) {
	// Start tracking execution time
	startTime := time.Now()

	defer func() {
		// Calculate total execution time
		duration := time.Since(startTime)
		if util.ConfigFile.DEBUG {
			fmt.Printf("getData total execution time: %s\n", duration)
		}
	}()

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
	SELECT %s, %s as value
	FROM latest_messages
	WHERE guild_id = ?
	AND date BETWEEN ? AND ?
`

	queryCont := `
	%s
	GROUP BY %s
	ORDER BY %s;
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
	var selectExpr, groupField string
	switch c.GroupBy {
	case "user":
		selectExpr, groupField = "author_id AS xaxes", "author_id"
	case "date":
		selectExpr, groupField = "strftime('%Y-%m-%d', date) AS xaxes", "strftime('%Y-%m-%d', date)"
	case "month":
		selectExpr, groupField = "strftime('%%Y-%%m', date) AS xaxes", "strftime('%%Y-%%m', date)"
	case "channel":
		selectExpr, groupField = "channel_id AS xaxes", "channel_id"
	case "channel_user":
		selectExpr, groupField = "channel_id AS yaxes, author_id as xaxes", "channel_id, author_id"
	default:
		selectExpr, groupField = "author_id", "author_id" // fallback
	}

	orderByField := "value DESC"
	if c.GroupBy == "date" {
		orderByField = fmt.Sprintf("%s ASC", groupField)
	}

	query = fmt.Sprintf(query, selectExpr, aggExpr)

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

	query += fmt.Sprintf(queryCont, whereClause, groupField, orderByField)

	// Track execution time for the database query
	queryStartTime := time.Now()
	rs, err := database.QueryDuckDB(query, []interface{}{c.GuildID, start, end})
	if err != nil {
		return nil, err
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Database query execution time: %s\n", time.Since(queryStartTime))
	}

	usernames, channels := make(map[string]string), make(map[string]string) // Cache for user and channel IDs

	// Pre-fetch users if grouping by "user"
	if c.GroupBy == "user" || c.GroupBy == "channel_user" {
		lastID := "" // Discord API requires the last ID for pagination

		for {
			members, err := bot.GuildMembers(c.GuildID, lastID, 1000) // Fetch up to 1000 at a time
			if err != nil {
				fmt.Println("Error fetching guild members:", err)
				break
			}

			if len(members) == 0 {
				break // No more members to fetch
			}

			for _, m := range members {
				usernames[m.User.ID] = m.User.Username
				lastID = m.User.ID // Set last ID for next batch
			}
		}
	}

	// Pre-fetch channels if grouping by "channel"
	if c.GroupBy == "channel" || c.GroupBy == "channel_user" {
		guildChannels, err := bot.GuildChannels(c.GuildID)
		if err == nil {
			for _, ch := range guildChannels {
				channels[ch.ID] = ch.Name
			}
		} else {
			fmt.Println("Error fetching guild channels:", err)
		}
	}

	var allData []ChartData

	// Track execution time for scanning the data
	scanStartTime := time.Now()
	for rs.Next() {
		var xaxes, yaxes string
		var value float64

		if c.GroupBy != "channel_user" {
			err = rs.Scan(&xaxes, &value)
		} else {
			err = rs.Scan(&yaxes, &xaxes, &value)
		}
		if err != nil {
			break
		}
		var xLabel, yLabel string
		switch c.GroupBy {
		case "user":
			if name, found := usernames[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
		case "channel":
			if name, found := channels[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
		case "channel_user":
			if name, found := usernames[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
			if name, found := channels[yaxes]; found {
				yLabel = name
			} else {
				yLabel = yaxes
			}
		case "date":
			xLabel = xaxes
		}
		allData = append(allData, ChartData{
			Xaxes:  xaxes,
			XLabel: xLabel,
			YLabel: yLabel,
			Value:  value,
		})
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Data scanning execution time: %s\n", time.Since(scanStartTime))
	}

	// Process top 14 and "Other" category
	if (c.GroupBy != "channel_user" && c.ChartType == SunburstChart ) || c.GroupBy != "date" && len(allData) > 14 {
		topData := allData[:14]
		otherValue := 0.0
		for _, d := range allData[14:] {
			otherValue += d.Value
		}
		topData = append(topData, ChartData{
			Xaxes:  "other",
			XLabel: "Other",
			Yaxes:  "other",
			YLabel: "Other",
			Value:  otherValue,
		})
		data = topData
	} else {
		data = allData
	}

	return
}

func (c *ChartTracker) getDebugData() (data []ChartData, err error) {
	// Start tracking execution time
	startTime := time.Now()

	defer func() {
		// Calculate total execution time
		duration := time.Since(startTime)
		if util.ConfigFile.DEBUG {
			fmt.Printf("getData total execution time: %s\n", duration)
		}
	}()

	
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
	SELECT %s, %s as value
	FROM latest_messages
	WHERE guild_id = ?
	AND date BETWEEN ? AND ?
`

	queryCont := `
	%s
	GROUP BY %s
	ORDER BY %s;
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
	var selectExpr, groupField string
	switch c.GroupBy {
	case "user":
		selectExpr, groupField = "author_id AS xaxes", "author_id"
	case "date":
		selectExpr, groupField = "strftime('%Y-%m-%d', date) AS xaxes", "strftime('%Y-%m-%d', date)"
	case "month":
		selectExpr, groupField = "strftime('%%Y-%%m', date) AS xaxes", "strftime('%%Y-%%m', date)"
	case "channel":
		selectExpr, groupField = "channel_id AS xaxes", "channel_id"
	case "channel_user":
		selectExpr, groupField = "channel_id AS yaxes, author_id as xaxes", "channel_id, author_id"
	default:
		selectExpr, groupField = "author_id", "author_id" // fallback
	}

	orderByField := "value DESC"
	if c.GroupBy == "date" {
		orderByField = fmt.Sprintf("%s ASC", groupField)
	}

	query = fmt.Sprintf(query, selectExpr, aggExpr)

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

	query += fmt.Sprintf(queryCont, whereClause, groupField, orderByField)

	// Track execution time for the database query
	queryStartTime := time.Now()
	rs, err := database.QueryDuckDB(query, []interface{}{c.GuildID, start, end})
	if err != nil {
		return nil, err
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Database query execution time: %s\n", time.Since(queryStartTime))
	}

	var allData []ChartData

	// Track execution time for scanning the data
	scanStartTime := time.Now()
	for rs.Next() {
		var xaxes, yaxes string
		var value float64

		if c.GroupBy != "channel_user" {
			err = rs.Scan(&xaxes, &value)
		} else {
			err = rs.Scan(&yaxes, &xaxes, &value)
		}
		if err != nil {
			break
		}
		
		allData = append(allData, ChartData{
			Xaxes:  xaxes,
			XLabel: xaxes,
			YLabel: yaxes,
			Value:  value,
		})
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Data scanning execution time: %s\n", time.Since(scanStartTime))
	}

	// Process top 14 and "Other" category
	if c.GroupBy != "channel_user" && c.GroupBy != "date" && len(allData) > 14 {
		topData := allData[:14]
		otherValue := 0.0
		for _, d := range allData[14:] {
			otherValue += d.Value
		}
		topData = append(topData, ChartData{
			Xaxes:  "other",
			XLabel: "Other",
			Value:  otherValue,
		})
		data = topData
	} else {
		data = allData
	}

	return
}
