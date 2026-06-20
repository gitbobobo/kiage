package display

import (
	"fmt"
	"os/exec"
)

// Eips 在屏幕显示简短提示（Kindle 固件自带）。
func Eips(row int, msg string) {
	if row < 1 {
		row = 1
	}
	_ = exec.Command("eips", fmt.Sprintf("%d", row), "1", msg).Run()
}
