package charts

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

func (c *ChartTracker) getData(bot *discordgo.Session) (data []*ChartData, err error) {
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
	if c.GroupBy.Metric == "channel" || c.GroupBy.Category == "channel" {
		guildChannels, err := bot.GuildChannels(c.GuildID)
		if err == nil {
			for _, ch := range guildChannels {
				channels[ch.ID] = ch.Name

			}
		} else {
			fmt.Println("Error fetching guild channels:", err)
		}
		threads, err := bot.GuildThreadsActive(c.GuildID)
		if err != nil {
			fmt.Printf("Error fetching threads for %s: %e\n", c.GuildID, err)
		}
		for _, thread := range threads.Threads {
			channels[thread.ID] = thread.Name
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
