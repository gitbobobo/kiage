package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ImportEdgeViaPython runs scripts/import-edge-cookie.py (requires browser-cookie3).
func ImportEdgeViaPython() (string, error) {
	roots := os.Getenv("KIAGE_ROOT")
	if roots == "" {
		wd, _ := os.Getwd()
		roots = wd
	}
	script := filepath.Join(roots, "scripts", "import-edge-cookie.py")
	if _, err := os.Stat(script); err != nil {
		// repo dev layout
		script = filepath.Join(roots, "..", "scripts", "import-edge-cookie.py")
	}
	py := os.Getenv("KIAGE_PYTHON")
	if py == "" {
		py = "python3"
	}
	cmd := exec.Command(py, script)
	cmd.Env = append(os.Environ(), "KIAGE_ROOT="+roots)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("python import failed: %v: %s", err, out)
	}
	cfg, err := Load(filepath.Join(roots, "etc", "config.json"))
	if err != nil {
		return "", err
	}
	if cfg.Cursor.SessionToken == "" {
		return "", fmt.Errorf("token still empty after python import")
	}
	return cfg.Cursor.SessionToken, nil
}
