//go:build go1.18

package captured

import (
	"errors"
	"image"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var (
	user32             = syscall.MustLoadDLL("user32.dll")
	procEnumWindows    = user32.MustFindProc("EnumWindows")
	procGetWindowTextW = user32.MustFindProc("GetWindowTextW")
	procGetWindowRect  = user32.MustFindProc("GetWindowRect")
)

type windows struct {
	base
}

type rectangle struct {
	Left, Top, Right, Bottom int32
}

func newCaptured() Interface {
	return &windows{}
}

func enumWindows(enumFunc uintptr, lparam uintptr) error {
	r0, _, err := syscall.SyscallN(procEnumWindows.Addr(), enumFunc, lparam)
	if r0 != 0 {
		return nil
	}
	if err != 0 {
		return error(err)
	}
	return syscall.EINVAL
}

func getWindowText(hWnd syscall.Handle, str *uint16, maxCount int32) (int32, error) {
	r0, _, err := syscall.SyscallN(procGetWindowTextW.Addr(), uintptr(hWnd), uintptr(unsafe.Pointer(str)), uintptr(maxCount))
	if r0 != 0 {
		return int32(r0), nil
	}
	if err != 0 {
		return 0, error(err)
	}
	return 0, syscall.EINVAL
}

func getWindowRect(hWnd syscall.Handle) (*rectangle, error) {
	rect := &rectangle{}
	r0, _, err := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(hWnd), uintptr(unsafe.Pointer(rect)))
	if r0 != 0 {
		return rect, nil
	}
	if err != 0 {
		return nil, error(err)
	}
	return nil, syscall.EINVAL
}

func (w windows) ListWindows() ([]*WindowInfo, error) {
	var result []*WindowInfo
	cb := syscall.NewCallback(func(hWnd syscall.Handle, p uintptr) uintptr {
		// Get window title
		title := make([]uint16, 256)
		_, err := getWindowText(hWnd, &title[0], int32(len(title)))
		if err != nil {
			return 1 // ignore the error, continue enumeration
		}

		// Get window size
		rect, err := getWindowRect(hWnd)

		windowInfo := &WindowInfo{}
		windowInfo.id = uintptr(hWnd)
		windowInfo.Title = syscall.UTF16ToString(title)
		windowInfo.Width = int(rect.Right - rect.Left)
		windowInfo.Height = int(rect.Bottom - rect.Top)

		result = append(result, windowInfo)
		return 1 // continue enumeration
	})
	if err := enumWindows(cb, 0); err != nil {
		return nil, err
	}
	return result, nil
}

func (w windows) CaptureWindow(window *WindowInfo, options Options) (*image.RGBA, error) {
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
