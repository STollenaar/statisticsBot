package charts

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	caser = cases.Title(language.AmericanEnglish)
)

func (c *ChartTracker) GenerateChart(bot *discordgo.Session) (*discordgo.File, error) {
	data, err := c.getData(bot)
	fmt.Println(data)
	t := time.Now()
	title := caser.String(strings.ReplaceAll(fmt.Sprintf("%s by %s", c.Metric, c.GroupBy), "_", " "))
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
	return &discordgo.File{
		Name:        fileName,
		ContentType: "image/png",
		Reader:      imgReader,
	}, nil
}

func (c *ChartTracker) generateBarChart(chartData []ChartData, title string) *charts.Bar {
	// create a new bar instance
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Right: "80%"}),
	)

	// Put data into instance
	bar.SetXAxis(toXaxes(chartData)).
		AddSeries(c.Metric, genBarData(chartData))
	// Where the magic happens
	return bar
}

func (c *ChartTracker) generatePieChart(ChartData []ChartData, title string) *charts.Pie {
	pie := charts.NewPie()

	pie.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Right: "80%"}),
	)

	pie.AddSeries(c.Metric, genPieData(ChartData))
	return pie
}

func (c *ChartTracker)generateLineChart(chartData []ChartData, title string) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Right: "80%"}),
	)

	line.SetXAxis(toXaxes(chartData)).
		AddSeries(c.Metric, genLineData(chartData)).
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

func toXaxes(chartData []ChartData) (rs []string) {
	for _, data := range chartData {
		rs = append(rs, data.XLabel)
	}
	return
}

func genBarData(chartData []ChartData) (rs []opts.BarData) {
	for _, data := range chartData {
		rs = append(rs, opts.BarData{Value: data.Value})
	}
	return
}

func genPieData(chartData []ChartData) (rs []opts.PieData) {
	for _, data := range chartData {
		rs = append(rs, opts.PieData{Name: data.XLabel, Value: data.Value})
	}
	return
}

func genLineData(chartData []ChartData) (rs []opts.LineData) {
	for _, data := range chartData {
		rs = append(rs, opts.LineData{Value: data.Value})
	}
	return
}
