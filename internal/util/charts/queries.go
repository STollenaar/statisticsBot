package charts

import (
	"fmt"
	"strings"
	"time"
)

const (
	MessageQuery = `
	WITH latest_messages AS (
		SELECT *
		FROM (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC) AS rn
			FROM %s
		) t
		WHERE rn = 1
	)
	SELECT %s, %s AS value
	FROM latest_messages
	WHERE guild_id = ?
	AND date BETWEEN ? AND ?
`

	ReactionQuery = `
	SELECT %s, %s AS value
	FROM reactions
	WHERE guild_id = ?
	AND date BETWEEN ? AND ?
`

	QueryCont = `
	%s
	GROUP BY %s
	ORDER BY %s;
`
)

func (c *ChartTracker) buildQuery(start, end time.Time) (query string, err error) {

	// Determine aggregation
	var aggExpr string
	switch c.Metric.Metric {
	case "count":
		aggExpr = "COUNT(*)"
	case "avg_length":
		aggExpr = "AVG(LENGTH(content))"
	case "freq":
		aggExpr = fmt.Sprintf(
			"COUNT(*) * 1.0 / DATEDIFF('day', DATE '%s', DATE '%s')",
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)
	}

	// Determine grouping
	var selectExpr, groupField string
	switch c.GroupBy {
	case MetricType{Category: "interaction", Metric: "user"}:
		selectExpr, groupField = "interaction_author_id AS xaxes", "interaction_author_id"
	case MetricType{Category: "interaction", Metric: "bot"}:
		selectExpr, groupField = "author_id AS xaxes", "author_id"
	case MetricType{Category: "single", Metric: "user"}:
		selectExpr, groupField = "author_id AS xaxes", "author_id"
	case MetricType{Category: "single", Metric: "date"}:
		selectExpr, groupField = "strftime('%Y-%m-%d', date) AS xaxes", "strftime('%Y-%m-%d', date)"
	case MetricType{Category: "single", Metric: "month"}:
		selectExpr, groupField = "strftime('%%Y-%%m', date) AS xaxes", "strftime('%%Y-%%m', date)"
	case MetricType{Category: "single", Metric: "channel"}:
		selectExpr, groupField = "channel_id AS xaxes", "channel_id"
	case MetricType{Category: "channel", Metric: "user", MultiAxes: true}:
		selectExpr, groupField = "channel_id AS yaxes, author_id AS xaxes", "channel_id, author_id"
	case MetricType{Category: "reaction", Metric: "user", MultiAxes: true}:
		selectExpr, groupField = "reaction AS yaxes, author_id AS xaxes", "reaction, author_id"
	case MetricType{Category: "reaction", Metric: "channel", MultiAxes: true}:
		selectExpr, groupField = "reaction AS yaxes, channel_id AS xaxes", "reaction, channel_id"
	case MetricType{Category: "interaction", Metric: "user", MultiAxes: true}:
		selectExpr, groupField = "author_id AS yaxes, interaction_author_id AS xaxes", "author_id, interaction_author_id"
	default:
		selectExpr, groupField = "author_id", "author_id" // fallback
	}

	orderByField := "value DESC"
	if c.GroupBy.Metric == "date" {
		orderByField = fmt.Sprintf("%s ASC", groupField)
	}

	switch c.Metric.Category {
	case "reaction":
		query = fmt.Sprintf(ReactionQuery, selectExpr, aggExpr)
	case "interaction":
		query = fmt.Sprintf(MessageQuery, "bot_messages", selectExpr, aggExpr)
	case "message":
		fallthrough
	default:
		query = fmt.Sprintf(MessageQuery, "messages", selectExpr, aggExpr)
	}

	var filters []string
	if len(c.Users) > 0 {
		filters = append(filters, fmt.Sprintf(`author_id in (%s)`, strings.Join(c.Users, ", ")))
	}
	if len(c.Channels) > 0 {
		filters = append(filters, fmt.Sprintf(`channel_id in (%s)`, strings.Join(c.Channels, ", ")))
	}

	whereClause := ""
	if c.Metric.Category == "interaction" {
		filters = append(filters, "interaction_author_id IS NOT NULL")
	}
	if len(filters) > 0 {
		whereClause = fmt.Sprintf("AND %s", strings.Join(filters, " AND "))
	}

	query += fmt.Sprintf(QueryCont, whereClause, groupField, orderByField)
	return
}
