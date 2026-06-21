package app

import (
	"testing"

	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/render"
)

func TestNextScreenCycle(t *testing.T) {
	screen, id := nextScreen(render.ScreenSummary, "")
	if screen != render.ScreenProvider || id != provider.CursorID {
		t.Fatalf("summary -> cursor: got %s %s", screen, id)
	}
	screen, id = nextScreen(render.ScreenProvider, provider.CursorID)
	if screen != render.ScreenProvider || id != provider.GLMID {
		t.Fatalf("cursor -> glm: got %s %s", screen, id)
	}
	screen, id = nextScreen(render.ScreenProvider, provider.GLMID)
	if screen != render.ScreenSummary || id != "" {
		t.Fatalf("glm -> summary: got %s %s", screen, id)
	}
}
