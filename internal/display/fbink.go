package display

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RefreshMode 控制 e-ink 刷屏方式（对齐 KOReader 波形策略）。
type RefreshMode int

const (
	RefreshFirst RefreshMode = iota // 启动首次：清屏 + GC16 闪烁全刷
	RefreshPartial                  // 日常局部：GL16 无闪烁（等同 KOReader partial/ui）
	RefreshFull                     // 周期性：GC16 全刷除残影（等同 KOReader full）
	RefreshInteractive              // 用户交互：DU 快速刷新
)

// DefaultFullRefreshEvery 与 KOReader 默认 full_refresh_count 一致。
const DefaultFullRefreshEvery = 6

type FBInk struct {
	Bin    string
	Width  int
	Height int
}

func New(bin string) *FBInk {
	if bin == "" {
		bin = ResolveFBInkBin()
	}
	return &FBInk{Bin: bin}
}

func (f *FBInk) SetViewport(vp Viewport) {
	f.Width = vp.Width
	f.Height = vp.Height
}

func (f *FBInk) ShowPNG(path string, mode RefreshMode) error {
	if _, err := exec.LookPath(f.Bin); err != nil {
		if _, statErr := os.Stat(f.Bin); statErr != nil {
			return fmt.Errorf("fbink not found at %s: %w", f.Bin, err)
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	w, h := f.Width, f.Height
	if w <= 0 {
		w = -1
	}
	if h <= 0 {
		h = -1
	}
	imgSpec := fmt.Sprintf("file=%s,w=%d,h=%d,x=0,y=0,dither", abs, w, h)
	args := []string{"-q", "-g", imgSpec}
	switch mode {
	case RefreshFirst:
		args = append(args, "-c", "-f", "-W", "GC16")
	case RefreshFull:
		args = append(args, "-f", "-W", "GC16")
	case RefreshInteractive:
		args = append(args, "-f", "-W", "DU")
	default:
		// KOReader 在 Kindle 上 partial/ui 使用 GL16（无闪烁）
		args = append(args, "-W", "GL16")
	}
	cmd := exec.Command(f.Bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%s %v: %s", f.Bin, err, msg)
		}
		return fmt.Errorf("%s %v", f.Bin, err)
	}
	return nil
}

func WriteTempPNG(dir string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "frame.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// PickRefreshMode 按 KOReader 策略选择波形：首次 GC16 清屏，之后 GL16 局部，每 N 次升全刷。
func PickRefreshMode(firstDone bool, partialCount int, every int) (RefreshMode, int) {
	if !firstDone {
		return RefreshFirst, 0
	}
	if every < 1 {
		every = DefaultFullRefreshEvery
	}
	partialCount++
	if partialCount >= every {
		return RefreshFull, 0
	}
	return RefreshPartial, partialCount
}
