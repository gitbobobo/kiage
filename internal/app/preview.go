package app

import (
	_ "embed"
	"net/http"
)

//go:embed preview.html
var previewHTML []byte

func servePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(previewHTML)
}
