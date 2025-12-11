package charts

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

func (c *ChartTracker) getData(client *bot.Client) (data []*ChartData, err error) {
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

	usernames, channels := make(map[string]string), make(map[string]string) // Cache for user and channel IDs

	// Pre-fetch users if grouping by "user"
	if c.GroupBy.Metric == "user" || c.GroupBy.Metric == "bot" {

		members := slices.Collect(client.Caches.Members(snowflake.MustParse(c.GuildID))) // Fetch up to 1000 at a time

		for _, m := range members {
			usernames[m.User.ID.String()] = m.User.Username
		}
	}

	// Pre-fetch channels if grouping by "channel"
	if c.GroupBy.Metric == "channel" || c.GroupBy.Category == "channel" {
		guildChannels := slices.Collect(client.Caches.ChannelsForGuild(snowflake.MustParse(c.GuildID)))
		if err == nil {
			for _, ch := range guildChannels {
				channels[ch.ID().String()] = ch.Name()

			}
		} else {
			fmt.Println("Error fetching guild channels:", err)
		}
		threads := client.Caches.GuildThreadsInChannel(snowflake.MustParse(c.GuildID))
		for _, thread := range threads {
			channels[thread.ID().String()] = thread.Name()
		}
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
		var xLabel, yLabel string
		switch {
		case c.GroupBy.Metric == "bot" && !c.GroupBy.MultiAxes:
			fallthrough
		case c.GroupBy.Metric == "user" && !c.GroupBy.MultiAxes:
			if name, found := usernames[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
		case c.GroupBy.Metric == "channel" && !c.GroupBy.MultiAxes:
			if name, found := channels[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
		case c.GroupBy.Metric == "date":
			xLabel = xaxes
		case c.GroupBy.Category == "channel" && c.GroupBy.Metric == "user" && c.GroupBy.MultiAxes:
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
		case c.GroupBy.Category == "reaction" && c.GroupBy.Metric == "user" && c.GroupBy.MultiAxes:
			if name, found := usernames[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
			yLabel = yaxes
		case c.GroupBy.Category == "reaction" && c.GroupBy.Metric == "channel" && c.GroupBy.MultiAxes:
			if name, found := channels[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
			yLabel = yaxes
		case c.GroupBy.Category == "interaction" && c.GroupBy.Metric == "user" && c.GroupBy.MultiAxes:
			if name, found := usernames[xaxes]; found {
				xLabel = name
			} else {
				xLabel = xaxes
			}
			if name, found := usernames[yaxes]; found {
				yLabel = name
			} else {
				yLabel = yaxes
			}
		}
		allData = append(allData, &ChartData{
			Xaxes:  xaxes,
			XLabel: xLabel,
			YLabel: yLabel,
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
			Yaxes:  "other",
			YLabel: "Other",
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
