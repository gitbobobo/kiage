//go:build darwin

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

// ImportEdgeCursorToken reads WorkosCursorSessionToken from Microsoft Edge on macOS.
func ImportEdgeCursorToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cookieDB := filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "Default", "Cookies")
	if _, err := os.Stat(cookieDB); err != nil {
		return "", fmt.Errorf("edge cookies db not found: %w", err)
	}

	key, err := edgeDecryptionKey()
	if err != nil {
		return "", err
	}

	tmp := cookieDB + ".kiage-copy"
	data, err := os.ReadFile(cookieDB)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var enc []byte
	err = db.QueryRow(`
		SELECT encrypted_value FROM cookies
		WHERE host_key IN ('cursor.com', '.cursor.com') AND name = 'WorkosCursorSessionToken'
		ORDER BY length(encrypted_value) DESC LIMIT 1`).Scan(&enc)
	if err != nil {
		return "", fmt.Errorf("WorkosCursorSessionToken not found in Edge: %w", err)
	}

	plain, err := decryptChromeCookie(key, enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func edgeDecryptionKey() ([]byte, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", "Microsoft Edge Safe Storage",
		"-a", "Microsoft Edge",
		"-w").Output()
	if err != nil {
		return nil, fmt.Errorf("read edge safe storage key: %w", err)
	}
	password := strings.TrimSpace(string(out))
	return pbkdf2.Key([]byte(password), []byte("saltysalt"), 1003, 16, sha1.New), nil
}

func decryptChromeCookie(key, enc []byte) ([]byte, error) {
	if len(enc) < 3 {
		return nil, fmt.Errorf("cookie value too short")
	}
	// v10 = AES-128-GCM
	if string(enc[:3]) != "v10" {
		return nil, fmt.Errorf("unsupported cookie encryption prefix %q", string(enc[:3]))
	}
	enc = enc[3:]
	if len(enc) < 12 {
		return nil, fmt.Errorf("invalid gcm payload")
	}
	nonce := enc[:12]
	ciphertext := enc[12:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
