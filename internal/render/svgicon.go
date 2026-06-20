package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed icons/settings.svg
var settingsIconSVG string

//go:embed icons/exit.svg
var exitIconSVG string

type svgIconCache struct {
	mu    sync.Mutex
	items map[string]*image.RGBA
}

var iconCache svgIconCache

func (c *svgIconCache) get(svg string, size int) *image.RGBA {
	key := fmt.Sprintf("%s@%d", svg, size)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[string]*image.RGBA)
	}
	if img, ok := c.items[key]; ok {
		return img
	}
	img := rasterizeSVG(svg, size)
	c.items[key] = img
	return img
}

func colorizeSVG(tmpl string, c color.Color) string {
	r, g, b, _ := c.RGBA()
	hex := fmt.Sprintf("#%02x%02x%02x", byte(r>>8), byte(g>>8), byte(b>>8))
	return strings.ReplaceAll(tmpl, "__COLOR__", hex)
}

func rasterizeSVG(svg string, size int) *image.RGBA {
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err != nil || icon == nil {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.SetTarget(0, 0, float64(size), float64(size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)
	icon.Draw(raster, 1.0)
	return img
}

func blitIcon(dst *image.RGBA, x, y int, icon *image.RGBA) {
	if icon == nil {
		return
	}
	b := icon.Bounds()
	for dy := 0; dy < b.Dy(); dy++ {
		for dx := 0; dx < b.Dx(); dx++ {
			_, _, _, a := icon.At(dx, dy).RGBA()
			if a == 0 {
				continue
			}
			dst.Set(x+dx, y+dy, icon.At(dx, dy))
		}
	}
}

func drawSettingsSVGIcon(dst *image.RGBA, x, y, size int, c color.Color) {
	svg := colorizeSVG(settingsIconSVG, c)
	icon := iconCache.get(svg, size)
	blitIcon(dst, x, y, icon)
}

func drawExitSVGIcon(dst *image.RGBA, x, y, size int, c color.Color) {
	svg := colorizeSVG(exitIconSVG, c)
	icon := iconCache.get(svg, size)
	blitIcon(dst, x, y, icon)
}
