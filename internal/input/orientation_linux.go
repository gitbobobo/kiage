//go:build linux

package input

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/display"
	"github.com/godbobo/kiage/internal/log"
)

const (
	evMsc   = 0x04
	mscGyro = 0x07

	// Oasis 2/3（bma2x2 等）ABS_PRESSURE 方向值
	oasis23PortraitUp   = 15
	oasis23PortraitDown = 16
	oasis23LandscapeL   = 17
	oasis23LandscapeR   = 18

	// Oasis 1 ABS_PRESSURE 方向值
	oasis1PortraitUp       = 19
	oasis1PortraitDown     = 20
	oasis1Landscape        = 21
	oasis1LandscapeRotated = 22
	oasis1PortraitLeft     = 15
	oasis1PortraitRotatedL = 16
	oasis1PortraitRight    = 17
	oasis1PortraitRotatedR = 18
)

type gyroGeneration int

const (
	gyroGenUnknown gyroGeneration = iota
	gyroGenOasis1
	gyroGenOasis23
)

// OrientationListener 监听 Oasis 加速度计旋转事件。
type OrientationListener struct {
	dev string
	f   *os.File
	gen gyroGeneration
}

func OpenOrientationListener() (*OrientationListener, error) {
	touchDev := touchDevicePath()
	accel := findAccelDevice(touchDev)
	l := &OrientationListener{}
	if accel.path == "" {
		log.Warn("orientation accel not found, rotation events disabled")
		return l, nil
	}
	f, err := os.Open(accel.path)
	if err != nil {
		return nil, fmt.Errorf("open accel %s: %w", accel.path, err)
	}
	gen := detectGyroGeneration(accel.path)
	l.dev = accel.path
	l.f = f
	l.gen = gen
	log.Info("orientation device %s name=%q gen=%d", accel.path, accel.name, gen)
	return l, nil
}

func detectGyroGeneration(dev string) gyroGeneration {
	namePath := "/sys/class/input/" + filepathBase(dev) + "/device/name"
	data, err := os.ReadFile(namePath)
	if err == nil {
		n := strings.ToLower(string(data))
		if strings.Contains(n, "bma") || strings.Contains(n, "kx132") || strings.Contains(n, "accel") {
			return gyroGenOasis23
		}
	}
	if data, err := os.ReadFile("/etc/prettyversion"); err == nil {
		s := strings.ToLower(string(data))
		if strings.Contains(s, "oasis") && (strings.Contains(s, "2") || strings.Contains(s, "3")) {
			return gyroGenOasis23
		}
		if strings.Contains(s, "oasis") {
			return gyroGenOasis1
		}
	}
	return gyroGenOasis23
}

func filepathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func (l *OrientationListener) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	return l.f.Close()
}

// LipcPortraitRota 读取 LIPC 竖屏方向（全屏模式下须短暂恢复 awesome）。
func LipcPortraitRota() (int, bool) {
	return lipcPortraitRotaReliable()
}

func lipcPortraitRotaReliable() (int, bool) {
	return rotaFromLIPCCode(queryAccelerometerLIPC())
}

func rotaFromLIPCCode(code string) (int, bool) {
	switch code {
	case "U", "V":
		return 0, true
	case "D":
		return 2, true
	default:
		return 0, false
	}
}

func (l *OrientationListener) Run(ctx context.Context, onRota func(rota int)) {
	if l == nil || l.f == nil || onRota == nil {
		return
	}
	go l.runLIPCPoll(ctx, onRota)

	var (
		lastRota = -1
		lastAt   time.Time
		buf      = make([]byte, inputEventSize)
		debounce = 150 * time.Millisecond
	)
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
		rota, ok := l.parseEvent(buf)
		if !ok {
			continue
		}
		now := time.Now()
		if rota == lastRota && now.Sub(lastAt) < debounce {
			continue
		}
		lastRota = rota
		lastAt = now
		log.Info("orientation event rota=%d dev=%s", rota, l.dev)
		onRota(rota)
	}
}

func (l *OrientationListener) runLIPCPoll(ctx context.Context, onRota func(rota int)) {
	var lastRota = -1
	tick := time.NewTicker(800 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			rota, ok := lipcPortraitRota()
			if !ok || rota == lastRota {
				continue
			}
			lastRota = rota
			log.Info("orientation lipc rota=%d", rota)
			onRota(rota)
		}
	}
}

func lipcPortraitRota() (int, bool) {
	code := queryAccelerometerLIPCQuiet()
	if code == "" {
		code = queryAccelerometerLIPC()
	}
	switch code {
	case "U", "V":
		return 0, true
	case "D":
		return 2, true
	default:
		return 0, false
	}
}

func queryAccelerometerLIPCQuiet() string {
	out, err := exec.Command("lipc-get-prop", "com.lab126.winmgr", "accelerometer").Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "[]")
}

func (l *OrientationListener) parseEvent(buf []byte) (rota int, ok bool) {
	typ := binary.LittleEndian.Uint16(buf[8:10])
	code := binary.LittleEndian.Uint16(buf[10:12])
	val := int32(binary.LittleEndian.Uint32(buf[12:16]))

	switch typ {
	case evAbs:
		if code != absPressure {
			return 0, false
		}
		return mapAbsGyroValue(l.gen, int(val))
	case evMsc:
		if code != mscGyro {
			return 0, false
		}
		return mapMscGyroValue(int(val))
	default:
		return 0, false
	}
}

func mapAbsGyroValue(gen gyroGeneration, val int) (int, bool) {
	if gen == gyroGenOasis1 {
		switch val {
		case oasis1PortraitUp, oasis1PortraitLeft, oasis1PortraitRight:
			return 0, true
		case oasis1PortraitDown, oasis1PortraitRotatedL, oasis1PortraitRotatedR:
			return 2, true
		case oasis1Landscape, oasis1LandscapeRotated:
			return 0, false
		}
		return 0, false
	}
	switch val {
	case oasis23PortraitUp:
		return 0, true
	case oasis23PortraitDown:
		return 2, true
	case oasis23LandscapeL, oasis23LandscapeR:
		return 0, false
	}
	return 0, false
}

func mapMscGyroValue(val int) (int, bool) {
	switch val {
	case 0: // UPRIGHT
		return 0, true
	case 2: // UPSIDE_DOWN
		return 2, true
	default:
		return 0, false
	}
}

// QueryInitialRota 启动时探测竖屏旋转角（0 正立，2 倒立）。
func QueryInitialRota(fbinkBin string) int {
	if v := os.Getenv("KIAGE_ORIENTATION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && (n == 0 || n == 2) {
			return n
		}
	}
	if code := queryAccelerometerLIPC(); code != "" {
		switch code {
		case "U", "V":
			return 0
		case "D":
			return 2
		case "R", "L":
			log.Warn("orientation initial lipc=%q landscape ignored", code)
		}
	}
	if fbinkBin != "" {
		if vp, err := display.QueryViewport(fbinkBin); err == nil {
			if r := vp.CurrentRota; r == 0 || r == 2 {
				return r
			}
		}
	}
	return 0
}

func queryAccelerometerLIPC() string {
	resume := os.Getenv("KIAGE_AWESOME_STOPPED") == "yes"
	if resume {
		_ = exec.Command("killall", "-CONT", "awesome").Run()
		defer func() { _ = exec.Command("killall", "-STOP", "awesome").Run() }()
	}
	out, err := exec.Command("lipc-get-prop", "com.lab126.winmgr", "accelerometer").Output()
	if err != nil {
		log.Warn("orientation lipc accelerometer: %v", err)
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "[]")
}
