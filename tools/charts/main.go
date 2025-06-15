package main

import (
	"github.com/stollenaar/statisticsbot/internal/util/charts"
)

func main() {
	chartTracker := charts.ChartTracker{
		GuildID:   "497544520695808000",
		ChartType: "pie",
		Metric:    "message_count",
		GroupBy:   "channel",
		DateRange: "30d",
	}

	chartTracker.GenerateDebugChart()
}
