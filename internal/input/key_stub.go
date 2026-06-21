//go:build !linux

package input

import "context"

type KeyListener struct{}

func OpenKeyListener() (*KeyListener, error) {
	return nil, nil
}

func (l *KeyListener) Close() error { return nil }

func (l *KeyListener) Run(ctx context.Context, h KeyHandler) {}
