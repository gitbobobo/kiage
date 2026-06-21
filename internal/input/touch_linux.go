//go:build linux

package input

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/godbobo/kiage/internal/log"
	"golang.org/x/sys/unix"
)

const (
	evSyn       = 0x00
	evKey       = 0x01
	evAbs       = 0x03
	synReport   = 0x00
	absX        = 0x00
	absY        = 0x01
	absPressure = 0x18
	absMtTrackingID = 0x39
	absMtPositionX  = 0x35
	absMtPositionY  = 0x36
	btnTouch    = 0x14a

	tapMaxDuration = 500 * time.Millisecond
	tapFlickerMin  = 20 * time.Millisecond
)

const inputEventSize = 16

type TouchListener struct {
	dev    string
	f      *os.File
	bounds TouchBounds
}

func OpenTouchListener() (*TouchListener, error) {
	dev := touchDevicePath()
	if dev == "" {
		return nil, fmt.Errorf("touch device not found")
	}
	f, err := os.Open(dev)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dev, err)
	}
	if err := grabTouchDevice(f); err != nil {
		log.Warn("touch grab failed (continuing): %v", err)
	}
	bounds := readTouchBounds(f, dev)
	log.Info("touch device %s maxX=%d maxY=%d", dev, bounds.MaxX, bounds.MaxY)
	return &TouchListener{dev: dev, f: f, bounds: bounds}, nil
}

type inputAbsinfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

func readTouchBounds(f *os.File, dev string) TouchBounds {
	maxX := absMaximum(f, absX)
	if maxX <= 0 {
		maxX = absMaximum(f, absMtPositionX)
	}
	maxY := absMaximum(f, absY)
	if maxY <= 0 {
		maxY = absMaximum(f, absMtPositionY)
	}
	if maxX <= 0 || maxY <= 0 {
		sx, sy := absMaxFromSysfs(dev)
		if maxX <= 0 {
			maxX = sx
		}
		if maxY <= 0 {
			maxY = sy
		}
	}
	if maxX <= 0 {
		maxX = 1071
	}
	if maxY <= 0 {
		maxY = 1447
	}
	return TouchBounds{MaxX: maxX, MaxY: maxY}
}

func absMaximum(f *os.File, code int) int {
	v, _ := absInfo(f, code)
	return int(v.Maximum)
}

func absValue(f *os.File, code int) (int, bool) {
	info, ok := absInfo(f, code)
	if !ok {
		return 0, false
	}
	return int(info.Value), true
}

func absInfo(f *os.File, code int) (inputAbsinfo, bool) {
	var info inputAbsinfo
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		f.Fd(),
		eviocgabs(code),
		uintptr(unsafe.Pointer(&info)),
	)
	if errno != 0 {
		return inputAbsinfo{}, false
	}
	return info, true
}

func absMaxFromSysfs(dev string) (maxX, maxY int) {
	event := filepath.Base(dev)
	base := filepath.Join("/sys/class/input", event, "device", "abs")
	pairs := [][2]string{
		{"abs_x", "abs_y"},
		{"ABS_X", "ABS_Y"},
		{"abs_0", "abs_1"},
		{"ABS_MT_POSITION_X", "ABS_MT_POSITION_Y"},
	}
	for _, p := range pairs {
		if maxX <= 0 {
			maxX = readSysfsInt(filepath.Join(base, p[0], "max"))
		}
		if maxY <= 0 {
			maxY = readSysfsInt(filepath.Join(base, p[1], "max"))
		}
		if maxX > 0 && maxY > 0 {
			break
		}
	}
	return maxX, maxY
}

func readSysfsInt(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return v
}

func grabTouchDevice(f *os.File) error {
	one := int32(1)
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		f.Fd(),
		eviocgrab(),
		uintptr(unsafe.Pointer(&one)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func releaseInputGrab(f *os.File) {
	if f == nil {
		return
	}
	zero := int32(0)
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		f.Fd(),
		eviocgrab(),
		uintptr(unsafe.Pointer(&zero)),
	)
	if errno != 0 {
		log.Warn("input ungrab failed: %v", errno)
	}
}

func touchDevicePath() string {
	if p := os.Getenv("KIAGE_TOUCH_DEV"); p != "" {
		return p
	}
	candidates := []string{
		"/dev/input/by-path/platform-imx-i2c.1-event",
		"/dev/input/by-path/platform-30a30000.i2c-event",
		"/dev/input/by-path/platform-ts-event",
		"/dev/input/touchscreen",
		"/dev/input/touchscreen0",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			return c
		}
	}
	entries, err := os.ReadDir("/dev/input")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		path := filepath.Join("/dev/input", e.Name())
		if isTouchscreen(path) {
			return path
		}
	}
	return ""
}

func isTouchscreen(path string) bool {
	namePath := filepath.Join("/sys/class/input", filepath.Base(path), "device", "name")
	data, err := os.ReadFile(namePath)
	if err != nil {
		return false
	}
	name := strings.ToLower(string(data))
	return strings.Contains(name, "touch") ||
		strings.Contains(name, "ts") ||
		strings.Contains(name, "cytma") ||
		strings.Contains(name, "goodix")
}

func (l *TouchListener) Close() error {
	if l.f == nil {
		return nil
	}
	releaseInputGrab(l.f)
	return l.f.Close()
}

func (l *TouchListener) Run(ctx context.Context, screenFn func() ScreenMapping, h Handler) {
	if l == nil || l.f == nil || h == nil || screenFn == nil {
		return
	}
	bounds := l.bounds
	qv, trackQuirk := h.(QuirkVersionHandler)

	var (
		active     bool
		fired      bool
		seenX      bool
		seenY      bool
		start      time.Time
		tapVersion uint64
		x, y       int
		buf        = make([]byte, inputEventSize)
	)

	beginTouch := func() {
		if active {
			return
		}
		active = true
		fired = false
		start = time.Now()
		if trackQuirk {
			tapVersion = qv.TouchQuirkVersion()
		}
	}

	fireTap := func() {
		if !active || fired {
			return
		}
		fired = true
		defer func() {
			active = false
			seenX = false
			seenY = false
		}()

		if trackQuirk && qv.TouchQuirkVersion() != tapVersion {
			log.Info("touch tap ignored quirk changed during tap")
			return
		}

		elapsed := time.Since(start)
		if elapsed < tapFlickerMin {
			log.Info("touch tap ignored flicker ms=%d", elapsed.Milliseconds())
			return
		}
		if elapsed >= tapMaxDuration {
			log.Info("touch tap ignored long ms=%d", elapsed.Milliseconds())
			return
		}
		if !seenX || !seenY {
			log.Info("touch tap ignored incomplete coords seenX=%v seenY=%v", seenX, seenY)
			return
		}
		screen := screenFn()
		px, py := MapTouch(x, y, bounds, screen)
		log.Info("touch raw=(%d,%d) mapped=(%d,%d) quirk swap=%v mx=%v my=%v",
			x, y, px, py, screen.Quirk.SwapAxes, screen.Quirk.MirrorX, screen.Quirk.MirrorY)
		h.OnTap(px, py)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if _, err := io.ReadFull(l.f, buf); err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		typ := binary.LittleEndian.Uint16(buf[8:10])
		code := binary.LittleEndian.Uint16(buf[10:12])
		val := int32(binary.LittleEndian.Uint32(buf[12:16]))

		switch typ {
		case evAbs:
			switch code {
			case absX, absMtPositionX:
				x = int(val)
				seenX = true
			case absY, absMtPositionY:
				y = int(val)
				seenY = true
			case absMtTrackingID:
				if val >= 0 {
					beginTouch()
				} else if active {
					fireTap()
				}
			case absPressure:
				if val > 0 {
					beginTouch()
				}
			}
		case evKey:
			if code != btnTouch {
				continue
			}
			if val == 1 {
				beginTouch()
			} else if val == 0 && active {
				fireTap()
			}
		}
	}
}
