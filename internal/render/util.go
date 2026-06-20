package render

import "fmt"

func formatPct(p float64) string {
	return fmt.Sprintf("%.0f%%", p)
}
