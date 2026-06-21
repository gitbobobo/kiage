package render

import "testing"

func TestKindleTopControlAction(t *testing.T) {
	t.Setenv("KIAGE_PORTRAIT", "1")
	size := Size{Width: 1264, Height: 1680}
	regions := TopControlsHitRegions(size, "Cursor", "token")

	cases := []struct {
		x, y   int
		action string
	}{
		{regions.MetricToggle.X + 10, 10, "metric_toggle"},
		{regions.Settings.X + 10, 10, "settings"},
		{regions.Exit.X + 10, 10, "exit"},
		{500, 150, ""},
	}
	for _, c := range cases {
		got := KindleTopControlAction(c.x, c.y, regions)
		if got != c.action {
			t.Fatalf("(%d,%d) got %q want %q", c.x, c.y, got, c.action)
		}
	}
}
