//go:build !linux

package input

import "context"

type OrientationListener struct{}

func OpenOrientationListener() (*OrientationListener, error) {
	return nil, nil
}

func (l *OrientationListener) Close() error { return nil }

func (l *OrientationListener) Run(ctx context.Context, onRota func(rota int)) {}

func QueryInitialRota(fbinkBin string) int {
	return 0
}

func LipcPortraitRota() (int, bool) {
	return 0, false
}
