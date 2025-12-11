package charts

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	caser = cases.Title(language.AmericanEnglish)
)

func (c *ChartTracker) GenerateChart(client *bot.Client) (*discord.File, error) {
	data, err := c.getData(client)
	fmt.Println(data)
	t := time.Now()
	title := caser.String(strings.ReplaceAll(fmt.Sprintf("%s by %s", c.Metric.Title(), c.GroupBy.Title()), "_", " "))
	fileName := fmt.Sprintf("%d.png", t.UnixNano())
	if err != nil {
		return nil, err
	}
	var image []byte

	switch c.ChartType {
	case BarChart:
		barChart := c.generateBarChart(data, title)
		err = render.MakeChartSnapshot(barChart.RenderContent(), fileName)
	case PieChart:
		pieChart := c.generatePieChart(data, title)
		err = render.MakeChartSnapshot(pieChart.RenderContent(), fileName)
	case LineChart:
		lineChart := c.generateLineChart(data, title)
		err = render.MakeChartSnapshot(lineChart.RenderContent(), fileName)
	case HeatmapChart:
		heatmapChart := c.generateHeatMapChart(data, title)
		err = render.MakeChartSnapshot(heatmapChart.RenderContent(), fileName)
	case SunburstChart:
		sunburstChart := c.generateSunBurstChart(data, title)
		err = render.MakeChartSnapshot(sunburstChart.RenderContent(), fileName)
	}

	if err != nil {
		return nil, err
	}

	image, err = os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	os.Remove(fileName)
	imgReader := bytes.NewReader(image)
	return &discord.File{
		Name:        fileName,
		Reader:      imgReader,
	}, nil
}

func (c *ChartTracker) GenerateDebugChart() {
	data, err := c.getDebugData()

	title := caser.String(strings.ReplaceAll(fmt.Sprintf("%s by %s", c.Metric.Title(), c.GroupBy.Title()), "_", " "))
	f, _ := os.Create("chart.html")

	if err != nil {
		fmt.Println(err)
	}
	switch c.ChartType {
	case BarChart:
		barChart := c.generateBarChart(data, title)
		barChart.Render(f)
	case PieChart:
		pieChart := c.generatePieChart(data, title)
		pieChart.Render(f)
	case LineChart:
		lineChart := c.generateLineChart(data, title)
		lineChart.Render(f)
	case HeatmapChart:
		heatmapChart := c.generateHeatMapChart(data, title)
		heatmapChart.Render(f)
	case SunburstChart:
		sunburstChart := c.generateSunBurstChart(data, title)
		sunburstChart.Render(f)
	}
}

func (c *ChartTracker) generateBarChart(chartData []*ChartData, title string) *charts.Bar {
	// create a new bar instance
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type:      "category",
			SplitArea: &opts.SplitArea{Show: opts.Bool(true)},
			AxisLabel: &opts.AxisLabel{
				Show:     opts.Bool(true),
				Interval: "0",
				Rotate:   45,
				FontSize: 10,
			},
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	// Put data into instance
	bar.SetXAxis(toXaxes(chartData)).
		AddSeries(c.Metric.ToString(), genBarData(chartData))
	// Where the magic happens
	return bar
}

func (c *ChartTracker) generatePieChart(chartData []*ChartData, title string) *charts.Pie {
	pie := charts.NewPie()

	pie.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	pie.AddSeries(c.Metric.ToString(), genPieData(chartData))
	return pie
}

func (c *ChartTracker) generateLineChart(chartData []*ChartData, title string) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	line.SetXAxis(toXaxes(chartData)).
		AddSeries(c.Metric.ToString(), genLineData(chartData)).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				ShowSymbol: opts.Bool(true),
			}),
			charts.WithLabelOpts(opts.Label{
				Show: opts.Bool(true),
			}),
		)
	return line
}

func (c *ChartTracker) generateSunBurstChart(chartData []*ChartData, title string) *charts.Sunburst {
	sunburst := charts.NewSunburst()

	sunburst.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	sunburst.AddSeries(c.Metric.ToString(), genSunburst(chartData))
	return sunburst
}

func (c *ChartTracker) generateHeatMapChart(chartData []*ChartData, title string) *charts.HeatMap {
	heatmap := charts.NewHeatMap()
	heatMapData, xAxes, yAxes := genHeatMap(chartData)

	heatmap.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type:      "category",
			Data:      xAxes,
			SplitArea: &opts.SplitArea{Show: opts.Bool(true)},
			AxisLabel: &opts.AxisLabel{
				Show:     opts.Bool(true),
				Interval: "0",
				Rotate:   45,
				FontSize: 10,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Type:      "category",
			Data:      yAxes,
			SplitArea: &opts.SplitArea{Show: opts.Bool(true)},
			AxisLabel: &opts.AxisLabel{
				Show:     opts.Bool(true), // Ensure labels are always displayed
				Interval: "0",             // Force every label to appear,
			},
		}),
		charts.WithVisualMapOpts(opts.VisualMap{
			Show: opts.Bool(false),
			InRange: &opts.VisualMapInRange{
				Color: []string{"#50a3ba", "#eac736", "#d94e5d"},
			},
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	heatmap.
		SetXAxis(xAxes).
		AddSeries(
			c.Metric.ToString(),
			heatMapData,
		)

	return heatmap
}

func (c *ChartTracker) CanGenerate() bool {
	return ((c.DateRange != "" && c.DateRange != "custom") || (c.DateRange == "custom" && c.CustomDateRange.End != nil && c.CustomDateRange.Start != nil)) && c.Metric != MetricType{} && c.GroupBy != MetricType{} && c.ChartType != ""
}

func toXaxes(chartData []*ChartData) (rs []string) {
	for _, data := range chartData {
		rs = append(rs, data.XLabel)
	}
	return
}

func toYaxes(chartData []*ChartData) (rs []string) {
	for _, data := range chartData {
		rs = append(rs, data.YLabel)
	}
	return
}

func genBarData(chartData []*ChartData) (rs []opts.BarData) {
	for _, data := range chartData {
		rs = append(rs, opts.BarData{Value: data.Value})
	}
	return
}

func genPieData(chartData []*ChartData) (rs []opts.PieData) {
	for _, data := range chartData {
		rs = append(rs, opts.PieData{Name: data.XLabel, Value: data.Value})
	}
	return
}

func genLineData(chartData []*ChartData) (rs []opts.LineData) {
	for _, data := range chartData {
		rs = append(rs, opts.LineData{Value: data.Value})
	}
	return
}

func genHeatMap(chartData []*ChartData) (rs []opts.HeatMapData, xAxes []string, yAxes []string) {
	xAxesTotals, yAxesTotals := make(map[string]float64), make(map[string]float64)
	for _, data := range chartData {
		xAxesTotals[data.XLabel] += data.Value
		yAxesTotals[data.YLabel] += data.Value
	}
	xAxes, yAxes = topNKeys(xAxesTotals, 14), topNKeys(yAxesTotals, 10)
	var filteredChartData []*ChartData
	for _, data := range chartData {
		if slices.Contains(xAxes, data.XLabel) && slices.Contains(yAxes, data.YLabel) {
			filteredChartData = append(filteredChartData, data)
		}
	}

	// Find min & max values
	var minVal, maxVal float64 = filteredChartData[0].Value, filteredChartData[0].Value
	for _, data := range filteredChartData {
		if data.Value < minVal {
			minVal = data.Value
		}
		if data.Value > maxVal {
			maxVal = data.Value
		}
	}

	for _, data := range filteredChartData {
		normalizedValue := normalizeLog(data.Value, minVal, maxVal) * 100
		// rs = append(rs, opts.HeatMapData{
		// 	Value: [3]interface{}{data.Xaxes, data.Yaxes, normalizedValue * 100}, // Scale to 0-100
		// })
		rs = append(rs, opts.HeatMapData{Value: [3]interface{}{slices.Index(xAxes, data.XLabel), slices.Index(yAxes, data.YLabel), normalizedValue}})
	}
	return
}

func genSunburst(chartData []*ChartData) (rs []opts.SunBurstData) {
	yAxes := uniqueStrings(toYaxes(chartData))

	type sunburst struct {
		value    float64
		children []*opts.SunBurstData
	}

	yAxesData := make(map[string]sunburst)

	for _, data := range chartData {
		if slices.Contains(yAxes, data.YLabel) {
			yData := yAxesData[data.YLabel]
			yData.value += data.Value
			yData.children = append(yData.children, &opts.SunBurstData{
				Value: data.Value,
				Name:  data.XLabel,
			})
			yAxesData[data.YLabel] = yData
		}
	}

	for key, data := range yAxesData {
		rs = append(rs, opts.SunBurstData{
			Value:    data.value,
			Name:     key,
			Children: data.children,
		})
	}

	return
}

// Unique values in a slice
func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, v := range input {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func normalizeLog(value, minVal, maxVal float64) float64 {
	if value <= 0 {
		return 0 // Avoid log(0) issues
	}
	logMin := math.Log10(minVal + 1)
	logMax := math.Log10(maxVal + 1)
	logVal := math.Log10(value + 1)

	// Scale between 0 and 100
	return (logVal - logMin) / (logMax - logMin)
}

func topNKeys(m map[string]float64, n int) []string {
	type kv struct {
		Key   string
		Value float64
	}
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	top := []string{}
	for i := 0; i < n && i < len(sorted); i++ {
		top = append(top, sorted[i].Key)
	}
	return top
}
