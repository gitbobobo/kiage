package glm

import "testing"

func TestPlanLabel(t *testing.T) {
	cases := map[string]string{
		"lite":     "Lite",
		"standard": "Pro",
		"pro":      "Pro",
		"max":      "Max",
		"MAX":      "Max",
	}
	for in, want := range cases {
		if got := planLabel(in); got != want {
			t.Fatalf("planLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
