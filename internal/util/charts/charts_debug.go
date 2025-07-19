package charts

import (
	"fmt"
	"strings"
	"time"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

func (c *ChartTracker) getDebugData() (data []*ChartData, err error) {
	// Start tracking execution time
	startTime := time.Now()

	defer func() {
		// Calculate total execution time
		duration := time.Since(startTime)
		if util.ConfigFile.DEBUG {
			fmt.Printf("getData total execution time: %s\n", duration)
		}
	}()

	start, end, err := c.getDateRange()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	query, err := c.buildQuery(start, end)
	if err != nil {
		return nil, err
	}

	// Track execution time for the database query
	queryStartTime := time.Now()
	rs, err := database.QueryDuckDB(query, []interface{}{c.GuildID, start, end})
	if err != nil {
		return nil, err
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Database query execution time: %s\n", time.Since(queryStartTime))
	}

	var allData []*ChartData

	// Track execution time for scanning the data
	scanStartTime := time.Now()
	for rs.Next() {
		var xaxes, yaxes string
		var value float64

		if !c.GroupBy.MultiAxes {
			err = rs.Scan(&xaxes, &value)
		} else {
			err = rs.Scan(&yaxes, &xaxes, &value)
		}
		if err != nil {
			break
		}

		allData = append(allData, &ChartData{
			Xaxes:  xaxes,
			XLabel: xaxes,
			YLabel: yaxes,
			Yaxes:  yaxes,
			Value:  value,
		})
	}
	if util.ConfigFile.DEBUG {
		fmt.Printf("Data scanning execution time: %s\n", time.Since(scanStartTime))
	}

	// Process top 14 and "Other" category
	if !c.GroupBy.MultiAxes &&
		c.GroupBy.Metric != "date" && len(allData) > 14 {
		topData := allData[:14]
		otherValue := 0.0
		for _, d := range allData[14:] {
			otherValue += d.Value
		}
		topData = append(topData, &ChartData{
			Xaxes:  "other",
			XLabel: "Other",
			Value:  otherValue,
		})
		data = topData
	} else {
		data = allData
	}

	if c.Metric.Category == "reaction" {
		for _, d := range data {
			if grouped := strings.Split(c.GroupBy.ToString(), "_"); len(grouped) > 1 {
				if grouped[0] == "reaction" {
					if _, ok := database.CustomEmojiCache[d.Yaxes]; ok {
						d.Yaxes = fmt.Sprintf(":%s:", d.Yaxes)
					}
				}
				if grouped[1] == "reaction" {
					if _, ok := database.CustomEmojiCache[d.Xaxes]; ok {
						d.Xaxes = fmt.Sprintf(":%s:", d.Xaxes)
					}
				}
			}
		}
	}

	return
}
