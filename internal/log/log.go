package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxLogBytes = 512 * 1024

var (
	mu   sync.Mutex
	out  io.Writer = os.Stderr
	file *os.File
)

func Init(path string) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := trimIfLarge(path, maxLogBytes); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if file != nil {
		_ = file.Close()
	}
	file = f
	// start.sh 以 `>>LOG 2>&1` 将进程 stderr 重定向到同一日志文件，若再
	// MultiWriter 到 os.Stderr 会导致每行写两遍。仅当 stderr 是终端（dev 直跑）
	// 时才镜像到 stderr；被重定向到文件时只写文件，避免重复。
	if stderrIsTerminal() {
		out = io.MultiWriter(os.Stderr, f)
	} else {
		out = f
	}
	return nil
}

func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	out = os.Stderr
	return err
}

func Info(format string, args ...any)  { write("INFO", format, args...) }
func Warn(format string, args ...any)  { write("WARN", format, args...) }
func Error(format string, args ...any) { write("ERROR", format, args...) }

func write(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s\n", ts, level, msg)
	_, _ = out.Write([]byte(line))
}

func trimIfLarge(path string, keep int) error {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.Size() <= int64(keep) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) <= keep {
		return nil
	}
	data = data[len(data)-keep:]
	return os.WriteFile(path, data, 0o644)
}
