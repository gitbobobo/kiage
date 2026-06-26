package app

import (
	"strconv"
	"strings"
)

func (a *App) classifyPowerdLine(line string) string {
	switch {
	case strings.Contains(line, "goingToScreenSaver"):
		return "suspend"
	case strings.Contains(line, "outOfScreenSaver"):
		return "resume"
	case strings.Contains(line, "readyToSuspend"):
		return "ready"
	case strings.Contains(line, "wakeupFromSuspend"), strings.Contains(line, "rtcWakeup"):
		return "rtc"
	default:
		return ""
	}
}

func parseReadyToSuspendDelay(line string) int {
	if !strings.Contains(line, "readyToSuspend") {
		return -1
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return 0
	}
	return n
}

func parseWakeupSuspendSec(line string) (int, bool) {
	if !strings.Contains(line, "wakeupFromSuspend") {
		return 0, false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// rtcWakeLooksScheduled 判断休眠时长是否接近已设的 RTC 闹钟（非「睡很久」或 ~90s 干扰）。
func rtcWakeLooksScheduled(sleptSec, alarmSec, slackSec int) bool {
	if alarmSec <= 0 {
		return false
	}
	if slackSec < 0 {
		slackSec = 0
	}
	if slackSec >= alarmSec {
		slackSec = alarmSec / 4
	}
	overshoot := slackSec * 2
	if overshoot < 600 {
		overshoot = 600
	}
	min := alarmSec - slackSec
	max := alarmSec + overshoot
	return sleptSec >= min && sleptSec <= max
}
