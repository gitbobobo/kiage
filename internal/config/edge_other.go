//go:build !darwin

package config

import "errors"

func ImportEdgeCursorToken() (string, error) {
	return "", errors.New("edge cookie import only supported on macOS")
}
