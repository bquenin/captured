package captured

import (
	"fmt"
	"image"
	"strings"
	"unicode"
)

const (
	FullWindow = 0
	CropTitle  = 1 << iota
)

type WindowInfo struct {
	id     uintptr
	Title  string
	Width  int
	Height int
}

type Options int

type Interface interface {
	ListWindows() ([]*WindowInfo, error)
	CaptureWindow(window *WindowInfo, options Options) (*image.RGBA, error)
	CaptureWindowByTitle(contains string, options Options) (*image.RGBA, error)
}

type base struct {
}

func (b *base) CaptureWindowByTitle(contains string, options Options) (*image.RGBA, error) {
	windowList, _ := Captured.ListWindows()
	for _, window := range windowList {
		windowTitle := strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, window.Title)
		if !strings.Contains(strings.ToLower(windowTitle), strings.ToLower(contains)) {
			continue
		}
		img, err := Captured.CaptureWindow(window, options)
		if err != nil {
			return nil, err
		}
		return img, nil
	}
	return nil, fmt.Errorf(`no window title containing "%s" found`, contains)
}

var Captured = newCaptured()
