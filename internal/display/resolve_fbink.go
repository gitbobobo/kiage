package display

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/godbobo/kiage/internal/log"
)

const fbinkNoImageMarker = "Image support is disabled in this FBInk build"

// FBInkCandidates 返回按优先级排序的 fbink 路径（含环境变量与扩展内捆绑副本）。
func FBInkCandidates() []string {
	root := os.Getenv("KIAGE_ROOT")
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}

	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	if p := os.Getenv("KIAGE_FBINK"); p != "" {
		add(p)
	}
	// 带图片支持的完整版优先（hotfix / kmc）；KOReader 附带的 fbink 常无 Image 支持
	add("/mnt/us/libkh/bin/fbink")
	add("/var/local/kmc/bin/fbink")
	if root != "" {
		add(filepath.Clean(filepath.Join(root, "..", "..", "libkh", "bin", "fbink")))
	}
	add("/mnt/usr/usbnet/bin/fbink")
	add("/usr/bin/fbink")
	add("/usr/local/bin/fbink")
	matches, _ := filepath.Glob("/mnt/us/fbink/*/bin/fbink")
	for _, m := range matches {
		add(m)
	}
	if root != "" {
		add(filepath.Join(root, "bin", "fbink"))
	}
	add("/mnt/us/koreader/fbink")

	if p, err := exec.LookPath("fbink"); err == nil {
		add(p)
	}

	return out
}

func FBInkSupportsImages(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return !bytes.Contains(data, []byte(fbinkNoImageMarker))
}

func ResolveFBInkBin() string {
	for _, p := range FBInkCandidates() {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if FBInkSupportsImages(p) {
			return p
		}
		log.Info("fbink skip (no image support): %s", p)
	}
	for _, p := range FBInkCandidates() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if root := os.Getenv("KIAGE_ROOT"); root != "" {
		return filepath.Join(root, "bin", "fbink")
	}
	return "./bin/fbink"
}

func LogFBInkProbe() {
	candidates := FBInkCandidates()
	resolved := ResolveFBInkBin()
	log.Info("fbink probe resolved=%q image_support=%v", resolved, FBInkSupportsImages(resolved))
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			log.Info("fbink candidate miss: %s (%v)", p, err)
			continue
		}
		log.Info("fbink candidate ok: %s image_support=%v", p, FBInkSupportsImages(p))
	}
}
