package main

import (
	"github.com/stollenaar/statisticsbot/internal/util/charts"
)

func main() {
	chartTracker := charts.ChartTracker{
		GuildID:   "497544520695808000",
		ChartType: "sunburst",
		Metric:    "message_count",
		GroupBy:   "channel_user",
		DateRange: "30d",
	}

	chartTracker.GenerateDebugChart()
}
