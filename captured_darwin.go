//go:build go1.18

package captured

import (
	/*
		#cgo LDFLAGS: -framework CoreGraphics
		#include <CoreGraphics/CoreGraphics.h>

		CGContextRef CGBitmapContextCreateSafe(void *data, size_t width, size_t height, size_t bitsPerComponent, size_t bytesPerRow, CGColorSpaceRef space, uint32_t bitmapInfo) {
			return CGBitmapContextCreate(data, width, height, bitsPerComponent, bytesPerRow, space, bitmapInfo);
		}
	*/
	"C"
	"errors"
	"image"
	"unsafe"
)

const titleHeight = 29

type darwin struct {
	base
}

func newCaptured() Interface {
	C.CGRequestScreenCaptureAccess()
	return &darwin{}
}

func (d darwin) ListWindows() ([]*WindowInfo, error) {
	windowInfoArray := CFArrayToArray(C.CGWindowListCopyWindowInfo(C.kCGWindowListOptionAll|C.kCGWindowListExcludeDesktopElements, C.kCGNullWindowID))

	var result []*WindowInfo
	for _, windowInfoRef := range windowInfoArray {
		windowInfo, err := Convert(windowInfoRef)
		if err != nil {
			return nil, errors.New("cannot convert windowInfoRef")
		}
		entry := &WindowInfo{}
		for k, v := range windowInfo.(map[interface{}]interface{}) {
			switch k.(string) {
			case "kCGWindowBounds":
				rect := v.(map[interface{}]interface{})
				entry.Width = int(rect["Width"].(float64))
				entry.Height = int(rect["Height"].(float64))
			case "kCGWindowName":
				entry.Title = v.(string)
			case "kCGWindowNumber":
				entry.id = uintptr(v.(int64))
			}
		}
		result = append(result, entry)
		Release(windowInfoRef)
	}

	return result, nil
}

func convertARGBtoRGBA(img *image.RGBA) {
	for r := 0; r < img.Rect.Max.Y; r++ {
		for c := 0; c < img.Rect.Max.X; c++ {
			offset := r*img.Stride + c*4
			img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2], img.Pix[offset+3] = img.Pix[offset+1], img.Pix[offset+2], img.Pix[offset+3], img.Pix[offset]
		}
	}
}

func (d darwin) CaptureWindow(window *WindowInfo, options Options) (*image.RGBA, error) {
	colorSpace := C.CGColorSpaceCreateWithName(C.kCGColorSpaceSRGB)
	if colorSpace == C.CGColorSpaceRef(0) {
		return nil, errors.New("cannot create color space")
	}
	defer C.CGColorSpaceRelease(colorSpace)

	capture := C.CGWindowListCreateImage(C.CGRectNull, C.kCGWindowListOptionIncludingWindow, C.uint32_t(window.id), C.kCGWindowImageBoundsIgnoreFraming)
	if capture == C.CGImageRef(0) {
		return nil, errors.New("cannot capture window")
	}
	defer C.CGImageRelease(capture)

	img := image.NewRGBA(image.Rect(0, 0, window.Width, window.Height))

	bitmapContext := C.CGBitmapContextCreateSafe(
		unsafe.Pointer(&img.Pix[0]),
		C.size_t(window.Width),
		C.size_t(window.Height),
		8,
		C.size_t(img.Stride),
		colorSpace,
		C.kCGImageAlphaNoneSkipFirst)
	if bitmapContext == C.CGContextRef(0) {
		return nil, errors.New("cannot create bitmap context")
	}
	defer C.CGContextRelease(bitmapContext)

	switch options {
	case FullWindow:
		C.CGContextDrawImage(bitmapContext, C.CGRectMake(C.CGFloat(0), C.CGFloat(0), C.CGFloat(window.Width), C.CGFloat(window.Height)), capture)
	case CropTitle:
		C.CGContextDrawImage(bitmapContext, C.CGRectMake(C.CGFloat(0), C.CGFloat(titleHeight), C.CGFloat(window.Width), C.CGFloat(window.Height)), capture)
	}

	convertARGBtoRGBA(img)

	return img, nil
}
