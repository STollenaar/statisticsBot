package main

import (
	"github.com/stollenaar/statisticsbot/internal/util/charts"
)

func main() {
	chartTracker := charts.ChartTracker{
		GuildID:   "497544520695808000",
		ChartType: "heatmap",
		Metric:    "reaction_count",
		GroupBy:   "reaction_user",
		DateRange: "30d",
	}

	chartTracker.GenerateDebugChart()
}
