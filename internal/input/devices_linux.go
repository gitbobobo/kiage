//go:build linux

package input

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type inputDeviceBlock struct {
	name     string
	handlers string
	evLine   string
}

type accelDevice struct {
	path string
	name string
}

var eventFromHandlers = regexp.MustCompile(`event[0-9]+`)

// parseInputDevices 解析 /proc/bus/input/devices 为设备块列表。
func parseInputDevices(data string) []inputDeviceBlock {
	var blocks []inputDeviceBlock
	for _, chunk := range strings.Split(data, "\n\n") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		var b inputDeviceBlock
		for _, line := range strings.Split(chunk, "\n") {
			switch {
			case strings.HasPrefix(line, "N: Name="):
				b.name = strings.Trim(strings.TrimPrefix(line, "N: Name="), `"`)
			case strings.HasPrefix(line, "H: Handlers="):
				b.handlers = strings.TrimPrefix(line, "H: Handlers=")
			case strings.HasPrefix(line, "B: EV="):
				b.evLine = line
			}
		}
		if b.handlers != "" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func blockHasAbsEV(b inputDeviceBlock) bool {
	s := strings.TrimPrefix(b.evLine, "B: EV=")
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if s == "" {
		return false
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return false
	}
	const evAbs = 0x08
	return v&evAbs != 0
}

func isNamedAccelDevice(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "bma") ||
		strings.Contains(n, "kx132") ||
		strings.Contains(n, "accel")
}

func isInterruptAccelDevice(name string) bool {
	return strings.Contains(strings.ToLower(name), "interrupt")
}

// accelDeviceRank 越高越优先；interrupt 伪设备不参与。
func accelDeviceRank(name string) int {
	n := strings.ToLower(name)
	if isInterruptAccelDevice(name) {
		return -1
	}
	if strings.Contains(n, "bma2x2") || strings.Contains(n, "kx132") {
		return 2
	}
	if isNamedAccelDevice(name) {
		return 1
	}
	return 0
}

func betterAccelDevice(name string, num int, bestName string, bestNum, bestRank int) bool {
	rank := accelDeviceRank(name)
	if rank > bestRank {
		return true
	}
	if rank < bestRank {
		return false
	}
	return num > bestNum
}

func isExcludedInputName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "touch") ||
		strings.Contains(n, "cytma") ||
		strings.Contains(n, "cyttsp") ||
		strings.Contains(n, "goodix")
}

func eventPathFromHandlers(handlers string) string {
	m := eventFromHandlers.FindString(handlers)
	if m == "" {
		return ""
	}
	return filepath.Join("/dev/input", m)
}

func inputEventNum(path string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(filepath.Base(path), "event"))
	return n
}

func readInputDeviceName(path string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/input", filepath.Base(path), "device", "name"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readInputDevicesFile() (string, error) {
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func findAccelDevicePath(exclude ...string) string {
	return findAccelDevice(exclude...).path
}

func findAccelDevice(exclude ...string) accelDevice {
	if p := os.Getenv("KIAGE_ACCEL_DEV"); p != "" {
		return accelDevice{path: p, name: readInputDeviceName(p)}
	}

	excluded := make(map[string]struct{}, len(exclude))
	for _, p := range exclude {
		if p != "" {
			excluded[p] = struct{}{}
		}
	}

	if d := bestOrientationInputFromSysfs(excluded); d.path != "" {
		return d
	}
	return bestOrientationInputFromProc(excluded)
}

// orientationInputRank 选监听设备：Oasis 上 bma_interrupt 才有旋转事件。
func orientationInputRank(name string) int {
	n := strings.ToLower(name)
	if isInterruptAccelDevice(name) {
		return 3
	}
	if strings.Contains(n, "bma2x2") || strings.Contains(n, "kx132") {
		return 2
	}
	if isNamedAccelDevice(name) {
		return 1
	}
	return 0
}

func betterOrientationInput(name string, num int, bestName string, bestNum, bestRank int) bool {
	rank := orientationInputRank(name)
	if rank > bestRank {
		return true
	}
	if rank < bestRank {
		return false
	}
	return num > bestNum
}

func bestOrientationInputFromSysfs(excluded map[string]struct{}) accelDevice {
	entries, err := os.ReadDir("/dev/input")
	if err != nil {
		return accelDevice{}
	}
	var best accelDevice
	bestNum := -1
	bestRank := -1
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		path := filepath.Join("/dev/input", e.Name())
		if _, skip := excluded[path]; skip {
			continue
		}
		name := readInputDeviceName(path)
		if name == "" || !isNamedAccelDevice(name) {
			continue
		}
		if isTouchscreen(path) {
			continue
		}
		num := inputEventNum(path)
		if betterOrientationInput(name, num, best.name, bestNum, bestRank) {
			bestNum = num
			bestRank = orientationInputRank(name)
			best = accelDevice{path: path, name: name}
		}
	}
	return best
}

func bestOrientationInputFromProc(excluded map[string]struct{}) accelDevice {
	data, err := readInputDevicesFile()
	if err != nil {
		return accelDevice{}
	}
	var bestNamed accelDevice
	bestNamedNum := -1
	bestNamedRank := -1
	var fallback accelDevice
	for _, b := range parseInputDevices(data) {
		if !blockHasAbsEV(b) {
			continue
		}
		path := eventPathFromHandlers(b.handlers)
		if path == "" {
			continue
		}
		if _, skip := excluded[path]; skip {
			continue
		}
		if isTouchscreen(path) || isExcludedInputName(b.name) {
			continue
		}
		if isNamedAccelDevice(b.name) {
			num := inputEventNum(path)
			if betterOrientationInput(b.name, num, bestNamed.name, bestNamedNum, bestNamedRank) {
				bestNamedNum = num
				bestNamedRank = orientationInputRank(b.name)
				bestNamed = accelDevice{path: path, name: b.name}
			}
			continue
		}
		if fallback.path == "" {
			fallback = accelDevice{path: path, name: b.name}
		}
	}
	if bestNamed.path != "" {
		return bestNamed
	}
	return fallback
}

func bestNamedAccelFromSysfs(excluded map[string]struct{}) accelDevice {
	entries, err := os.ReadDir("/dev/input")
	if err != nil {
		return accelDevice{}
	}
	var best accelDevice
	bestNum := -1
	bestRank := -1
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		path := filepath.Join("/dev/input", e.Name())
		if _, skip := excluded[path]; skip {
			continue
		}
		name := readInputDeviceName(path)
		if name == "" || !isNamedAccelDevice(name) || isInterruptAccelDevice(name) {
			continue
		}
		if isTouchscreen(path) {
			continue
		}
		num := inputEventNum(path)
		if betterAccelDevice(name, num, best.name, bestNum, bestRank) {
			bestNum = num
			bestRank = accelDeviceRank(name)
			best = accelDevice{path: path, name: name}
		}
	}
	return best
}

func bestAccelFromProc(excluded map[string]struct{}) accelDevice {
	data, err := readInputDevicesFile()
	if err != nil {
		return accelDevice{}
	}
	var bestNamed accelDevice
	bestNamedNum := -1
	bestNamedRank := -1
	var fallback accelDevice
	for _, b := range parseInputDevices(data) {
		if !blockHasAbsEV(b) {
			continue
		}
		path := eventPathFromHandlers(b.handlers)
		if path == "" {
			continue
		}
		if _, skip := excluded[path]; skip {
			continue
		}
		if isTouchscreen(path) || isExcludedInputName(b.name) {
			continue
		}
		if isNamedAccelDevice(b.name) {
			if isInterruptAccelDevice(b.name) {
				continue
			}
			num := inputEventNum(path)
			if betterAccelDevice(b.name, num, bestNamed.name, bestNamedNum, bestNamedRank) {
				bestNamedNum = num
				bestNamedRank = accelDeviceRank(b.name)
				bestNamed = accelDevice{path: path, name: b.name}
			}
			continue
		}
		if fallback.path == "" {
			fallback = accelDevice{path: path, name: b.name}
		}
	}
	if bestNamed.path != "" {
		return bestNamed
	}
	return fallback
}
