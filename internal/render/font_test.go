package render

import (
	"os"
	"testing"
)

func TestLoadCJKFont(t *testing.T) {
	t.Setenv("KIAGE_ROOT", findKiageRoot(t))
	f, err := loadBaseFont()
	if err != nil {
		t.Fatalf("load font: %v", err)
	}
	if f == nil {
		t.Fatal("nil font")
	}
	face := getFace(16)
	if face == nil {
		t.Fatal("nil face")
	}
}

func findKiageRoot(t *testing.T) string {
	for _, p := range []string{"extension", "../extension", "../../extension"} {
		if _, err := os.Stat(p + "/fonts/NotoSansSC-Regular.otf"); err == nil {
			abs, _ := os.Getwd()
			return abs + "/" + p
		}
	}
	t.Skip("extension/fonts not found")
	return ""
}
