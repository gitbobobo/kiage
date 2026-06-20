package paths

import (
	"os"
	"path/filepath"
)

type Roots struct {
	Root   string
	Etc    string
	Cache  string
	Config string
	DB     string
	Import string
	Log    string
	Lock   string
}

func Resolve() Roots {
	root := os.Getenv("KIAGE_ROOT")
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	r := Roots{
		Root:   root,
		Etc:    filepath.Join(root, "etc"),
		Cache:  filepath.Join(root, "cache"),
		Config: filepath.Join(root, "etc", "config.json"),
		DB:     filepath.Join(root, "cache", "kiage.db"),
		Import: filepath.Join(root, "etc", "import", "token"),
		Log:    filepath.Join(root, "cache", "kiage.log"),
		Lock:   filepath.Join(root, "cache", ".sync.lock"),
	}
	return r
}
