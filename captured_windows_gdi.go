//go:build go1.18

package captured

import (
	"errors"
	"image"
	"unsafe"

	"github.com/lxn/win"
)

// captureWindowGDI captures a window using the legacy GDI BitBlt method.
// This is used as a fallback on Windows versions before 1903.
func captureWindowGDI(window *WindowInfo, options Options) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, window.Width, window.Height))

	hWnd := win.HWND(window.id)
	hdc := win.GetDC(hWnd)
	if hdc == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer win.ReleaseDC(hWnd, hdc)

	memory_device := win.CreateCompatibleDC(hdc)
	if memory_device == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memory_device)

	bitmap := win.CreateCompatibleBitmap(hdc, int32(window.Width), int32(window.Height))
	if bitmap == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))

	var header win.BITMAPINFOHEADER
	header.BiSize = uint32(unsafe.Sizeof(header))
	header.BiPlanes = 1
	header.BiBitCount = 32
	header.BiWidth = int32(window.Width)
	header.BiHeight = int32(-window.Height)
	header.BiCompression = win.BI_RGB
	header.BiSizeImage = 0

	// GetDIBits balks at using Go memory on some systems. The MSDN example uses
	// GlobalAlloc, so we'll do that too. See:
	// https://docs.microsoft.com/en-gb/windows/desktop/gdi/capturing-an-image
	bitmapDataSize := uintptr(((int64(window.Width)*int64(header.BiBitCount) + 31) / 32) * 4 * int64(window.Height))
	hmem := win.GlobalAlloc(win.GMEM_MOVEABLE, bitmapDataSize)
	defer win.GlobalFree(hmem)
	memptr := win.GlobalLock(hmem)
	defer win.GlobalUnlock(hmem)

	old := win.SelectObject(memory_device, win.HGDIOBJ(bitmap))
	if old == 0 {
		return nil, errors.New("SelectObject failed")
	}
	defer win.SelectObject(memory_device, old)

	if !win.BitBlt(memory_device, 0, 0, int32(window.Width), int32(window.Height), hdc, int32(0), int32(0), win.SRCCOPY) {
		return nil, errors.New("BitBlt failed")
	}

	if win.GetDIBits(hdc, bitmap, 0, uint32(window.Height), (*uint8)(memptr), (*win.BITMAPINFO)(unsafe.Pointer(&header)), win.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	i := 0
	src := uintptr(memptr)
	for y := 0; y < window.Height; y++ {
		for x := 0; x < window.Width; x++ {
			v0 := *(*uint8)(unsafe.Pointer(src))
			v1 := *(*uint8)(unsafe.Pointer(src + 1))
			v2 := *(*uint8)(unsafe.Pointer(src + 2))

			// BGRA => RGBA, and set A to 255
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255

			i += 4
			src += 4
		}
	}

	return img, nil
}
