package render

import (
	"image"
	"image/color"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const glyphMargin = 12

func drawText(img *image.RGBA, x, y int, text string, size int, bold bool) int {
	return drawTextColor(img, x, y, text, size, bold, color.Black)
}

func drawTextColor(img *image.RGBA, x, y int, text string, size int, bold bool, c color.Color) int {
	if size <= 0 {
		size = 14
	}
	face := getFace(size)
	if face == nil {
		return drawTextASCIIColor(img, x, y, text, c)
	}

	lineHeight := size + 8
	lines := splitLines(text)
	for i, line := range lines {
		ly := y + i*lineHeight
		drawStringFace(img, face, x, ly+size, line, bold, c)
	}
	return y + len(lines)*lineHeight
}

func drawStringFace(img *image.RGBA, face font.Face, x, baselineY int, text string, bold bool, c color.Color) {
	if text == "" {
		return
	}
	b := img.Bounds()
	if x < b.Min.X+glyphMargin || x >= b.Max.X-glyphMargin ||
		baselineY < b.Min.Y+glyphMargin || baselineY >= b.Max.Y+glyphMargin {
		return
	}
	point := fixed.P(x, baselineY)
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  point,
	}
	if bold {
		d.Dot = fixed.P(x+1, baselineY)
		d.DrawString(text)
		d.Dot = point
	}
	d.DrawString(text)
}

func drawTextASCII(img *image.RGBA, x, y int, text string) int {
	return drawTextASCIIColor(img, x, y, text, color.Black)
}

func drawTextASCIIColor(img *image.RGBA, x, y int, text string, c color.Color) int {
	text = asciiFallback(text)
	b := img.Bounds()
	if x >= b.Max.X {
		return y
	}
	point := fixed.P(x, y+13)
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(text)
	lines := splitLines(text)
	return y + len(lines)*16
}

func asciiFallback(text string) string {
	if text == "" {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		if r < unicode.MaxASCII && r != 0 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func textWidth(text string, size int) int {
	if size <= 0 {
		size = 14
	}
	face := getFace(size)
	if face == nil {
		return len(asciiFallback(text)) * 7
	}
	return font.MeasureString(face, text).Ceil()
}

func drawTextRight(img *image.RGBA, rightX, y int, text string, size int, bold bool) {
	drawText(img, rightX-textWidth(text, size), y, text, size, bold)
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i, ch := range text {
		if ch == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	lines = append(lines, text[start:])
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
