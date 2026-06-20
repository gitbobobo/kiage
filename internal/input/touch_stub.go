//go:build !linux

package input

import "context"

// TouchListener 非 Linux 平台无触摸输入。
type TouchListener struct{}

func OpenTouchListener() (*TouchListener, error) {
	return nil, nil
}

func (l *TouchListener) Close() error { return nil }

func (l *TouchListener) Run(ctx context.Context, screen ScreenMapping, h Handler) {}
