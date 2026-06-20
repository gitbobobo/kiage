package render

import "testing"

func TestKindleTopBarAction(t *testing.T) {
	t.Setenv("KIAGE_PORTRAIT", "1")
	size := Size{Width: 1264, Height: 1680}
	regions := TopControlsHitRegions(size, "Cursor", "token")

	// 22:49 日志：y=0~25 均在同一条视觉按钮行，旧热区 y>=12 导致 miss
	cases := []struct {
		x, y   int
		action string
	}{
		{1081, 0, "metric_toggle"},
		{1081, 24, "metric_toggle"},
		{987, 7, "metric_toggle"},
		{961, 15, "metric_toggle"},
		{1234, 17, "exit"},
		{500, 20, ""},
		{500, 150, ""},
	}
	for _, c := range cases {
		got := KindleTopBarAction(size, c.x, c.y, regions)
		if got != c.action {
			t.Fatalf("(%d,%d) got %q want %q", c.x, c.y, got, c.action)
		}
	}
}
