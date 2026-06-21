//go:build linux

package input

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/log"
)

type KeyListener struct {
	f *os.File
}

func OpenKeyListener() (*KeyListener, error) {
	dev := gpioKeyDevicePath()
	if dev == "" {
		return nil, fmt.Errorf("gpio key device not found")
	}
	f, err := os.Open(dev)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dev, err)
	}
	if err := grabTouchDevice(f); err != nil {
		log.Warn("key grab failed (continuing): %v", err)
	}
	log.Info("key device %s name=%q", dev, readInputDeviceName(dev))
	return &KeyListener{f: f}, nil
}

func (l *KeyListener) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	releaseInputGrab(l.f)
	return l.f.Close()
}

func (l *KeyListener) Run(ctx context.Context, h KeyHandler) {
	if l == nil || l.f == nil || h == nil {
		return
	}
	det := newClickDetector()
	defer det.Stop()

	var (
		buf       = make([]byte, inputEventSize)
		pressedAt = make(map[uint16]int)
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
		typ := binary.LittleEndian.Uint16(buf[8:10])
		code := binary.LittleEndian.Uint16(buf[10:12])
		val := int32(binary.LittleEndian.Uint32(buf[12:16]))
		if typ != evKey {
			continue
		}
		c := int(code)
		switch val {
		case 1:
			pressedAt[code] = h.PortraitRota()
		case 0:
			rota, ok := pressedAt[code]
			if !ok {
				continue
			}
			delete(pressedAt, code)

			var dir clickDir
			switch {
			case ScreenUpKey(c, rota):
				dir = clickUp
			case ScreenDownKey(c, rota):
				dir = clickDown
			default:
				continue
			}

			keyCode, keyRota := c, rota
			det.onRelease(dir, func(action ScreenKeyAction) {
				log.Info("key %s code=%d rota=%d", screenKeyActionName(action), keyCode, keyRota)
				h.OnScreenKey(action)
			})
		case 2:
		}
	}
}

func screenKeyActionName(a ScreenKeyAction) string {
	switch a {
	case ScreenUpSingle:
		return "up-single"
	case ScreenUpDouble:
		return "up-double"
	case ScreenDownSingle:
		return "down-single"
	case ScreenDownDouble:
		return "down-double"
	}
	return "?"
}

func gpioKeyDevicePath() string {
	if p := os.Getenv("KIAGE_KEY_DEV"); p != "" {
		return p
	}
	candidates := []string{
		"/dev/input/by-path/platform-gpiokey.0-event",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			return c
		}
	}
	data, err := readInputDevicesFile()
	if err != nil {
		return ""
	}
	for _, b := range parseInputDevices(data) {
		n := strings.ToLower(b.name)
		if !strings.Contains(n, "gpiokey") && !strings.Contains(n, "gpio-keys") && !strings.Contains(n, "fsr_keypad") {
			continue
		}
		if path := eventPathFromHandlers(b.handlers); path != "" {
			return path
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
		n := strings.ToLower(readInputDeviceName(path))
		if strings.Contains(n, "gpiokey") || strings.Contains(n, "gpio-keys") || strings.Contains(n, "fsr_keypad") {
			return path
		}
	}
	return ""
}
