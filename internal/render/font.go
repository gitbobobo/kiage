package render

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var (
	fontOnce sync.Once
	fontErr  error
	baseFont *opentype.Font
	faceCache = map[int]font.Face{}
)

func loadBaseFont() (*opentype.Font, error) {
	fontOnce.Do(func() {
		for _, p := range fontSearchPaths() {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			baseFont, fontErr = opentype.Parse(data)
			if fontErr == nil {
				return
			}
		}
		if fontErr == nil {
			fontErr = os.ErrNotExist
		}
	})
	return baseFont, fontErr
}

func fontSearchPaths() []string {
	var paths []string
	if p := os.Getenv("KIAGE_FONT"); p != "" {
		paths = append(paths, p)
	}
	if root := os.Getenv("KIAGE_ROOT"); root != "" {
		paths = append(paths, filepath.Join(root, "fonts", "NotoSansSC-Regular.otf"))
		paths = append(paths, filepath.Join(root, "fonts", "NotoSansSC-Regular.ttf"))
	}
	if runtime.GOOS == "darwin" {
		paths = append(paths,
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
		)
	}
	// Kindle / extension deploy path
	paths = append(paths,
		"/mnt/us/extensions/kiage/fonts/NotoSansSC-Regular.otf",
		"/mnt/us/extensions/kiage/fonts/NotoSansSC-Regular.ttf",
		"extension/fonts/NotoSansSC-Regular.otf",
		"extension/fonts/NotoSansSC-Regular.ttf",
		"fonts/NotoSansSC-Regular.ttf",
	)
	return paths
}

func getFace(size int) font.Face {
	if size <= 0 {
		size = 14
	}
	if f, ok := faceCache[size]; ok {
		return f
	}
	ff, err := loadBaseFont()
	if err != nil {
		return nil
	}
	face, err := opentype.NewFace(ff, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	faceCache[size] = face
	return face
}
