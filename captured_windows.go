//go:build go1.18

package captured

import (
	"errors"
	"image"
)

type windows struct {
	base
}

func newCaptured() Interface {
	return &windows{}
}

func (w windows) ListWindows() ([]*WindowInfo, error) {
	return nil, errors.New("not implemented")
}

func (w windows) CaptureWindow(window *WindowInfo, options Options) (*image.RGBA, error) {
	return nil, errors.New("not implemented")
}
