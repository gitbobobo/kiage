package render

import (
	"image"
	"image/color"
)

const (
	settingsBtnSize = 26
	settingsIconSz  = 16
	controlsGap     = 8
)

func metricToggleWidth(metric string) int {
	fontSize := MetricToggleFontSize()
	padX := MetricTogglePadX()
	gap := 2
	if KindleUI() {
		gap = 4
	}
	opts := []string{"Token", "Cost"}
	total := 0
	for i, label := range opts {
		total += textWidth(label, fontSize) + padX*2
		if i > 0 {
			total += gap
		}
	}
	_ = metric
	return total
}

func drawTopRightControls(img *image.RGBA, rightX, y int, view ViewState) {
	btnSz := SettingsBtnSize()
	gap := ControlsGap()
	drawExitButton(img, rightX, y)
	settingsRight := rightX - btnSz - gap
	drawSettingsButton(img, settingsRight, y, view.SettingsActive)
	metricRight := settingsRight - btnSz - gap
	drawMetricToggle(img, metricRight, y, view.ChartMetric, view.SupportsCost)
	if view.SettingsActive && view.SettingsURL != "" {
		drawSettingsBubble(img, settingsRight, y+btnSz+6, view.SettingsURL)
	}
}

func drawExitButton(img *image.RGBA, rightX, y int) {
	btnSz := SettingsBtnSize()
	iconSz := SettingsIconSize()
	x := rightX - btnSz
	drawRect(img, x, y, btnSz, btnSz, color.Gray{Y: 235})
	pad := (btnSz - iconSz) / 2
	drawExitSVGIcon(img, x+pad, y+pad, iconSz, color.Black)
}

func drawSettingsButton(img *image.RGBA, rightX, y int, active bool) {
	btnSz := SettingsBtnSize()
	iconSz := SettingsIconSize()
	x := rightX - btnSz
	bg := color.Gray{Y: 235}
	iconColor := color.Black
	if active {
		bg = color.Gray{Y: 60}
		iconColor = color.White
	}
	drawRect(img, x, y, btnSz, btnSz, bg)
	pad := (btnSz - iconSz) / 2
	drawSettingsSVGIcon(img, x+pad, y+pad, iconSz, iconColor)
}

func drawSettingsBubble(img *image.RGBA, rightX, y int, url string) {
	const fontSize = 11
	const pad = 10
	text := url
	textW := textWidth(text, fontSize)
	boxW := textW + pad*2
	if boxW < 120 {
		boxW = 120
	}
	boxH := fontSize + pad*2 + 4
	boxX := rightX - boxW
	drawRect(img, boxX, y, boxW, boxH, color.White)
	drawRectOutline(img, boxX, y, boxW, boxH, color.Gray{Y: 140})
	drawText(img, boxX+pad, y+pad, text, fontSize, false)
}
