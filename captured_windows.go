//go:build go1.18

package captured

import (
	"image"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32             = syscall.MustLoadDLL("user32.dll")
	procEnumWindows    = user32.MustFindProc("EnumWindows")
	procGetWindowTextW = user32.MustFindProc("GetWindowTextW")
	procGetWindowRect  = user32.MustFindProc("GetWindowRect")
)

// useWGC indicates whether to use Windows.Graphics.Capture API
// This is true on Windows 10 version 1903 (build 18362) and later
var useWGC = detectWGCSupport()

// detectWGCSupport checks if the Windows.Graphics.Capture API is available
func detectWGCSupport() bool {
	ver := windows.RtlGetVersion()
	// Windows 10 version 1903 (build 18362) introduced Windows.Graphics.Capture
	return ver.MajorVersion >= 10 && ver.BuildNumber >= 18362
}

type windowsImpl struct {
	base
}

type rectangle struct {
	Left, Top, Right, Bottom int32
}

func newCaptured() Interface {
	return &windowsImpl{}
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

func (w windowsImpl) ListWindows() ([]*WindowInfo, error) {
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

func (w windowsImpl) CaptureWindow(window *WindowInfo, options Options) (*image.RGBA, error) {
	if useWGC {
		return captureWindowWGC(window, options)
	}
	return captureWindowGDI(window, options)
}
