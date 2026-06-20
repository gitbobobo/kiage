package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/godbobo/kiage/internal/aggregate"
)

type ViewState struct {
	ProviderName   string
	ChartMetric    string // "token" or "cost"
	Orientation    string // "landscape" or "portrait"
	SyncStatus     string
	SyncMessage    string
	SettingsActive bool
	SettingsURL    string
}

type Size struct {
	Width  int
	Height int
}

func DefaultSize(orientation string) Size {
	if orientation == "portrait" {
		return Size{Width: 1072, Height: 1448}
	}
	return Size{Width: 1448, Height: 1072}
}

func RenderPNG(dash aggregate.Dashboard, line []aggregate.LinePoint, heat aggregate.HeatmapStats, view ViewState, size Size) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
			err = fmt.Errorf("render panic: %v", r)
		}
	}()

	img := image.NewRGBA(image.Rect(0, 0, size.Width, size.Height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	w := size.Width - PadX*2
	topFixed, _ := regionHeights(view.Orientation)

	drawTopSection(img, dash, view, PadX, 16, w, topFixed)

	weeks := heat.Weeks
	if weeks <= 0 {
		weeks = HeatmapWeeksForWidth(w)
	}
	cellSize := HeatmapCellSizeForWidth(w, weeks)
	heatH := HeatmapBlockHeight(cellSize)

	contentTop := 16 + topFixed + 4
	heatY := size.Height - footerH - heatH
	contentBottom := heatY - heatGapTop

	chartH := contentBottom - contentTop
	if chartH >= 80 {
		_ = drawLineChart(img, PadX, contentTop, w, chartH, line, view.ChartMetric)
	}

	drawHeatmap(img, PadX, heatY, w, heatH, heat)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawBar(img *image.RGBA, x, y, w int, label string, pct float64) int {
	drawText(img, x, y, label+" "+formatPct(pct), 14, false)
	barY := y + 20
	barH := 12
	fillW := int(float64(w) * clamp(pct/100, 0, 1))
	drawRect(img, x, barY, w, barH, color.Gray{Y: 220})
	if fillW > 0 {
		drawRect(img, x, barY, fillW, barH, color.Gray{Y: 40})
	}
	return barY + barH
}

func drawBox(img *image.RGBA, x, y, w, h int, text string) {
	drawRect(img, x, y, w, h, color.Gray{Y: 228})
	drawText(img, x+10, y+8, text, 15, true)
}

func drawRect(img *image.RGBA, x, y, w, h int, c color.Color) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			img.Set(x+dx, y+dy, c)
		}
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
