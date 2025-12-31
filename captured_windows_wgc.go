//go:build go1.18

package captured

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/lxn/win"
)

var (
	winrtInitOnce sync.Once
	winrtInitErr  error
)

// WinRT and DXGI DLLs
var (
	combase                              = syscall.NewLazyDLL("combase.dll")
	procRoInitialize                     = combase.NewProc("RoInitialize")
	procRoGetActivationFactory           = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateStringReference     = combase.NewProc("WindowsCreateStringReference")

	dxgiDll                              = syscall.NewLazyDLL("dxgi.dll")
	procCreateDXGIFactory1               = dxgiDll.NewProc("CreateDXGIFactory1")

	d3d11Dll                             = syscall.NewLazyDLL("d3d11.dll")
	procCreateDirect3D11DeviceFromDXGIDevice = d3d11Dll.NewProc("CreateDirect3D11DeviceFromDXGIDevice")
)

// GUIDs
var (
	IID_IGraphicsCaptureItemInterop        = ole.NewGUID("{3628E81B-3CAC-4C60-B7F4-23CE0E0C3356}")
	IID_IGraphicsCaptureItem               = ole.NewGUID("{79c3f95b-31f7-4ec2-a464-632ef5d30760}")
	IID_IDirect3D11CaptureFramePoolStatics = ole.NewGUID("{7784056A-67AA-4D53-AE54-1088D5A8CA21}")
	IID_IDirect3DDxgiInterfaceAccess       = ole.NewGUID("{A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1}")
	IID_ID3D11Texture2D                    = ole.NewGUID("{6f15aaf2-d208-4e89-9ab4-489535d34f9c}")
	IID_IGraphicsCaptureSession3           = ole.NewGUID("{f2cdd966-22ae-5ea1-9596-3a289344c3be}")
	IID_IDXGIDevice                        = ole.NewGUID("{54ec77fa-1377-44e6-8c32-88fd5f44c84c}")
	IID_IInspectable                       = ole.NewGUID("{AF86E2E0-B12D-4C6A-9C5A-D7AA65101E90}")
)

// RuntimeClass names
const (
	RuntimeClass_GraphicsCaptureItem       = "Windows.Graphics.Capture.GraphicsCaptureItem"
	RuntimeClass_Direct3D11CaptureFramePool = "Windows.Graphics.Capture.Direct3D11CaptureFramePool"
)

// DirectX pixel format
const DirectXPixelFormat_B8G8R8A8UIntNormalized int32 = 87

// HSTRING handling
type HSTRING uintptr

type HSTRING_HEADER struct {
	Reserved [24]byte
}

func createHString(s string) (HSTRING, *HSTRING_HEADER, error) {
	u16, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, nil, err
	}
	var header HSTRING_HEADER
	var hstring HSTRING
	hr, _, _ := syscall.SyscallN(
		procWindowsCreateStringReference.Addr(),
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)-1),
		uintptr(unsafe.Pointer(&header)),
		uintptr(unsafe.Pointer(&hstring)),
	)
	if hr != 0 {
		return 0, nil, ole.NewError(hr)
	}
	return hstring, &header, nil
}

// SizeInt32 for WinRT
type SizeInt32 struct {
	Width  int32
	Height int32
}

// initWinRT initializes Windows Runtime
func initWinRT() error {
	winrtInitOnce.Do(func() {
		hr, _, _ := syscall.SyscallN(procRoInitialize.Addr(), 1) // RO_INIT_MULTITHREADED
		if hr != 0 && hr != 0x80010106 { // S_OK or RPC_E_CHANGED_MODE (already initialized)
			winrtInitErr = ole.NewError(hr)
		}
	})
	return winrtInitErr
}

// captureWindowWGC captures a window using Windows.Graphics.Capture API
func captureWindowWGC(window *WindowInfo, options Options) (*image.RGBA, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize WinRT
	if err := initWinRT(); err != nil {
		return nil, err
	}

	// Get D3D11 device
	d3dDevice, err := getD3D11Device()
	if err != nil {
		return nil, err
	}

	// Create WinRT IDirect3DDevice from D3D11 device
	direct3DDevice, err := createWinRTDevice(d3dDevice)
	if err != nil {
		return nil, err
	}
	defer direct3DDevice.Release()

	// Create GraphicsCaptureItem from HWND
	captureItem, err := createCaptureItemForWindow(win.HWND(window.id))
	if err != nil {
		return nil, err
	}
	defer captureItem.Release()

	// Get item size
	size, err := captureItem.Size()
	if err != nil {
		return nil, err
	}

	// Get frame pool statics
	framePoolStatics, err := getFramePoolStatics()
	if err != nil {
		return nil, err
	}
	defer framePoolStatics.Release()

	// Create frame pool
	framePool, err := framePoolStatics.Create(direct3DDevice, DirectXPixelFormat_B8G8R8A8UIntNormalized, 1, size)
	if err != nil {
		return nil, err
	}
	defer framePool.Close()

	// Create capture session
	session, err := framePool.CreateCaptureSession(captureItem)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// Try to disable border (Windows 11+)
	if session3, err := session.QuerySession3(); err == nil {
		session3.SetIsBorderRequired(false)
		session3.Release()
	}

	// Start capture
	if err := session.StartCapture(); err != nil {
		return nil, err
	}

	// Wait for frame with timeout
	var frame *IDirect3D11CaptureFrame
	deadline := time.Now().Add(time.Second * 2)
	for time.Now().Before(deadline) {
		frame, err = framePool.TryGetNextFrame()
		if err == nil && frame != nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	if frame == nil {
		return nil, errors.New("failed to capture frame: timeout")
	}
	defer frame.Close()

	// Get content size
	contentSize, err := frame.ContentSize()
	if err != nil {
		return nil, err
	}

	// Get surface from frame
	surface, err := frame.Surface()
	if err != nil {
		return nil, err
	}
	defer surface.Release()

	// Get ID3D11Texture2D from surface
	texture, err := getTextureFromSurface(surface)
	if err != nil {
		return nil, err
	}
	defer texture.Release()

	// Create staging texture and copy
	stagingTexture, err := createStagingTexture(d3dDevice, uint32(contentSize.Width), uint32(contentSize.Height))
	if err != nil {
		return nil, err
	}
	defer stagingTexture.Release()

	context := d3dDevice.GetImmediateContext()
	defer context.Release()
	context.CopyResource(stagingTexture, texture)

	// Extract pixels to image
	img, err := copyTextureToImage(context, stagingTexture, int(contentSize.Width), int(contentSize.Height))
	if err != nil {
		return nil, err
	}

	return img, nil
}

// createWinRTDevice creates a WinRT IDirect3DDevice from a D3D11 device
func createWinRTDevice(d3dDevice *ID3D11Device) (*IDirect3DDevice, error) {
	// Query for IDXGIDevice
	var dxgiDevice *ole.IUnknown
	hr, _, _ := syscall.SyscallN(
		d3dDevice.VTable().QueryInterface,
		uintptr(unsafe.Pointer(d3dDevice)),
		uintptr(unsafe.Pointer(IID_IDXGIDevice)),
		uintptr(unsafe.Pointer(&dxgiDevice)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	defer dxgiDevice.Release()

	// Create WinRT device from DXGI device
	var inspectable *ole.IInspectable
	hr, _, _ = syscall.SyscallN(
		procCreateDirect3D11DeviceFromDXGIDevice.Addr(),
		uintptr(unsafe.Pointer(dxgiDevice)),
		uintptr(unsafe.Pointer(&inspectable)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}

	return (*IDirect3DDevice)(unsafe.Pointer(inspectable)), nil
}

// createCaptureItemForWindow creates a GraphicsCaptureItem from an HWND
func createCaptureItemForWindow(hwnd win.HWND) (*IGraphicsCaptureItem, error) {
	// Get activation factory for GraphicsCaptureItem
	hstring, _, err := createHString(RuntimeClass_GraphicsCaptureItem)
	if err != nil {
		return nil, err
	}

	var factory *ole.IUnknown
	hr, _, _ := syscall.SyscallN(
		procRoGetActivationFactory.Addr(),
		uintptr(hstring),
		uintptr(unsafe.Pointer(IID_IGraphicsCaptureItemInterop)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	defer factory.Release()

	interop := (*IGraphicsCaptureItemInterop)(unsafe.Pointer(factory))

	// Create capture item from window
	var item *ole.IInspectable
	err = interop.CreateForWindow(hwnd, IID_IGraphicsCaptureItem, &item)
	if err != nil {
		return nil, err
	}

	return (*IGraphicsCaptureItem)(unsafe.Pointer(item)), nil
}

// getFramePoolStatics gets the frame pool statics interface
func getFramePoolStatics() (*IDirect3D11CaptureFramePoolStatics, error) {
	hstring, _, err := createHString(RuntimeClass_Direct3D11CaptureFramePool)
	if err != nil {
		return nil, err
	}

	var factory *ole.IUnknown
	hr, _, _ := syscall.SyscallN(
		procRoGetActivationFactory.Addr(),
		uintptr(hstring),
		uintptr(unsafe.Pointer(IID_IDirect3D11CaptureFramePoolStatics)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}

	return (*IDirect3D11CaptureFramePoolStatics)(unsafe.Pointer(factory)), nil
}

// getTextureFromSurface gets ID3D11Texture2D from IDirect3DSurface
func getTextureFromSurface(surface *IDirect3DSurface) (*ID3D11Texture2D, error) {
	// Query for IDirect3DDxgiInterfaceAccess
	var dxgiAccess *IDirect3DDxgiInterfaceAccess
	hr, _, _ := syscall.SyscallN(
		surface.VTable().QueryInterface,
		uintptr(unsafe.Pointer(surface)),
		uintptr(unsafe.Pointer(IID_IDirect3DDxgiInterfaceAccess)),
		uintptr(unsafe.Pointer(&dxgiAccess)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	defer dxgiAccess.Release()

	// Get the texture
	return dxgiAccess.GetInterface()
}

// ============================================================================
// Interface definitions
// ============================================================================

// IDirect3DDevice represents a WinRT Direct3D device
type IDirect3DDevice struct {
	ole.IInspectable
}

// IGraphicsCaptureItem represents a capture target
type IGraphicsCaptureItem struct {
	ole.IInspectable
}

type IGraphicsCaptureItemVtbl struct {
	QueryInterface         uintptr
	AddRef                 uintptr
	Release                uintptr
	GetIids                uintptr
	GetRuntimeClassName    uintptr
	GetTrustLevel          uintptr
	GetDisplayName         uintptr
	GetSize                uintptr
	AddClosed              uintptr
	RemoveClosed           uintptr
}

func (item *IGraphicsCaptureItem) vtbl() *IGraphicsCaptureItemVtbl {
	return (*IGraphicsCaptureItemVtbl)(unsafe.Pointer(item.RawVTable))
}

func (item *IGraphicsCaptureItem) Size() (*SizeInt32, error) {
	var size SizeInt32
	hr, _, _ := syscall.SyscallN(
		item.vtbl().GetSize,
		uintptr(unsafe.Pointer(item)),
		uintptr(unsafe.Pointer(&size)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return &size, nil
}

// IGraphicsCaptureItemInterop for creating items from HWND
type IGraphicsCaptureItemInterop struct {
	ole.IUnknown
}

type IGraphicsCaptureItemInteropVtbl struct {
	ole.IUnknownVtbl
	CreateForWindow  uintptr
	CreateForMonitor uintptr
}

func (interop *IGraphicsCaptureItemInterop) vtbl() *IGraphicsCaptureItemInteropVtbl {
	return (*IGraphicsCaptureItemInteropVtbl)(unsafe.Pointer(interop.RawVTable))
}

func (interop *IGraphicsCaptureItemInterop) CreateForWindow(hwnd win.HWND, riid *ole.GUID, result **ole.IInspectable) error {
	hr, _, _ := syscall.SyscallN(
		interop.vtbl().CreateForWindow,
		uintptr(unsafe.Pointer(interop)),
		uintptr(hwnd),
		uintptr(unsafe.Pointer(riid)),
		uintptr(unsafe.Pointer(result)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

// IDirect3D11CaptureFramePoolStatics for creating frame pools
type IDirect3D11CaptureFramePoolStatics struct {
	ole.IInspectable
}

type IDirect3D11CaptureFramePoolStaticsVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	GetIids             uintptr
	GetRuntimeClassName uintptr
	GetTrustLevel       uintptr
	Create              uintptr
}

func (s *IDirect3D11CaptureFramePoolStatics) vtbl() *IDirect3D11CaptureFramePoolStaticsVtbl {
	return (*IDirect3D11CaptureFramePoolStaticsVtbl)(unsafe.Pointer(s.RawVTable))
}

func (s *IDirect3D11CaptureFramePoolStatics) Create(device *IDirect3DDevice, format int32, buffers int32, size *SizeInt32) (*IDirect3D11CaptureFramePool, error) {
	var pool *IDirect3D11CaptureFramePool
	// SizeInt32 is passed as two separate int32 values on the stack
	hr, _, _ := syscall.SyscallN(
		s.vtbl().Create,
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(device)),
		uintptr(format),
		uintptr(buffers),
		uintptr(size.Width)|(uintptr(size.Height)<<32), // Pack as single 64-bit value
		uintptr(unsafe.Pointer(&pool)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return pool, nil
}

// IDirect3D11CaptureFramePool manages capture frames
type IDirect3D11CaptureFramePool struct {
	ole.IInspectable
}

type IDirect3D11CaptureFramePoolVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	GetIids             uintptr
	GetRuntimeClassName uintptr
	GetTrustLevel       uintptr
	Recreate            uintptr
	TryGetNextFrame     uintptr
	AddFrameArrived     uintptr
	RemoveFrameArrived  uintptr
	CreateCaptureSession uintptr
	GetDispatcherQueue  uintptr
}

func (p *IDirect3D11CaptureFramePool) vtbl() *IDirect3D11CaptureFramePoolVtbl {
	return (*IDirect3D11CaptureFramePoolVtbl)(unsafe.Pointer(p.RawVTable))
}

func (p *IDirect3D11CaptureFramePool) TryGetNextFrame() (*IDirect3D11CaptureFrame, error) {
	var frame *IDirect3D11CaptureFrame
	hr, _, _ := syscall.SyscallN(
		p.vtbl().TryGetNextFrame,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&frame)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return frame, nil
}

func (p *IDirect3D11CaptureFramePool) CreateCaptureSession(item *IGraphicsCaptureItem) (*IGraphicsCaptureSession, error) {
	var session *IGraphicsCaptureSession
	hr, _, _ := syscall.SyscallN(
		p.vtbl().CreateCaptureSession,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(item)),
		uintptr(unsafe.Pointer(&session)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return session, nil
}

func (p *IDirect3D11CaptureFramePool) Close() error {
	// IClosable::Close is at offset 6 in the vtable (after IInspectable methods)
	closeVtbl := (*struct {
		QueryInterface      uintptr
		AddRef              uintptr
		Release             uintptr
		GetIids             uintptr
		GetRuntimeClassName uintptr
		GetTrustLevel       uintptr
		Close               uintptr
	})(unsafe.Pointer(p.RawVTable))

	hr, _, _ := syscall.SyscallN(closeVtbl.Close, uintptr(unsafe.Pointer(p)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

// IDirect3D11CaptureFrame represents a captured frame
type IDirect3D11CaptureFrame struct {
	ole.IInspectable
}

type IDirect3D11CaptureFrameVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	GetIids             uintptr
	GetRuntimeClassName uintptr
	GetTrustLevel       uintptr
	GetSurface          uintptr
	GetSystemRelativeTime uintptr
	GetContentSize      uintptr
}

func (f *IDirect3D11CaptureFrame) vtbl() *IDirect3D11CaptureFrameVtbl {
	return (*IDirect3D11CaptureFrameVtbl)(unsafe.Pointer(f.RawVTable))
}

func (f *IDirect3D11CaptureFrame) Surface() (*IDirect3DSurface, error) {
	var surface *IDirect3DSurface
	hr, _, _ := syscall.SyscallN(
		f.vtbl().GetSurface,
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(&surface)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return surface, nil
}

func (f *IDirect3D11CaptureFrame) ContentSize() (*SizeInt32, error) {
	var size SizeInt32
	hr, _, _ := syscall.SyscallN(
		f.vtbl().GetContentSize,
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(&size)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return &size, nil
}

func (f *IDirect3D11CaptureFrame) Close() error {
	closeVtbl := (*struct {
		QueryInterface      uintptr
		AddRef              uintptr
		Release             uintptr
		GetIids             uintptr
		GetRuntimeClassName uintptr
		GetTrustLevel       uintptr
		Close               uintptr
	})(unsafe.Pointer(f.RawVTable))

	hr, _, _ := syscall.SyscallN(closeVtbl.Close, uintptr(unsafe.Pointer(f)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

// IGraphicsCaptureSession manages a capture session
type IGraphicsCaptureSession struct {
	ole.IInspectable
}

type IGraphicsCaptureSessionVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	GetIids             uintptr
	GetRuntimeClassName uintptr
	GetTrustLevel       uintptr
	StartCapture        uintptr
}

func (s *IGraphicsCaptureSession) vtbl() *IGraphicsCaptureSessionVtbl {
	return (*IGraphicsCaptureSessionVtbl)(unsafe.Pointer(s.RawVTable))
}

func (s *IGraphicsCaptureSession) StartCapture() error {
	hr, _, _ := syscall.SyscallN(
		s.vtbl().StartCapture,
		uintptr(unsafe.Pointer(s)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (s *IGraphicsCaptureSession) Close() error {
	closeVtbl := (*struct {
		QueryInterface      uintptr
		AddRef              uintptr
		Release             uintptr
		GetIids             uintptr
		GetRuntimeClassName uintptr
		GetTrustLevel       uintptr
		Close               uintptr
	})(unsafe.Pointer(s.RawVTable))

	hr, _, _ := syscall.SyscallN(closeVtbl.Close, uintptr(unsafe.Pointer(s)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (s *IGraphicsCaptureSession) QuerySession3() (*IGraphicsCaptureSession3, error) {
	var session3 *IGraphicsCaptureSession3
	hr, _, _ := syscall.SyscallN(
		s.VTable().QueryInterface,
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(IID_IGraphicsCaptureSession3)),
		uintptr(unsafe.Pointer(&session3)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return session3, nil
}

// IGraphicsCaptureSession3 for Windows 11+ border control
type IGraphicsCaptureSession3 struct {
	ole.IInspectable
}

type IGraphicsCaptureSession3Vtbl struct {
	QueryInterface        uintptr
	AddRef                uintptr
	Release               uintptr
	GetIids               uintptr
	GetRuntimeClassName   uintptr
	GetTrustLevel         uintptr
	GetIsBorderRequired   uintptr
	SetIsBorderRequired   uintptr
}

func (s *IGraphicsCaptureSession3) vtbl() *IGraphicsCaptureSession3Vtbl {
	return (*IGraphicsCaptureSession3Vtbl)(unsafe.Pointer(s.RawVTable))
}

func (s *IGraphicsCaptureSession3) SetIsBorderRequired(value bool) error {
	var val uintptr
	if value {
		val = 1
	}
	hr, _, _ := syscall.SyscallN(
		s.vtbl().SetIsBorderRequired,
		uintptr(unsafe.Pointer(s)),
		val,
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

// IDirect3DSurface represents a Direct3D surface
type IDirect3DSurface struct {
	ole.IInspectable
}

// IDirect3DDxgiInterfaceAccess for getting DXGI interface from WinRT
type IDirect3DDxgiInterfaceAccess struct {
	ole.IUnknown
}

type IDirect3DDxgiInterfaceAccessVtbl struct {
	ole.IUnknownVtbl
	GetInterface uintptr
}

func (d *IDirect3DDxgiInterfaceAccess) vtbl() *IDirect3DDxgiInterfaceAccessVtbl {
	return (*IDirect3DDxgiInterfaceAccessVtbl)(unsafe.Pointer(d.RawVTable))
}

func (d *IDirect3DDxgiInterfaceAccess) GetInterface() (*ID3D11Texture2D, error) {
	var texture *ID3D11Texture2D
	hr, _, _ := syscall.SyscallN(
		d.vtbl().GetInterface,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(IID_ID3D11Texture2D)),
		uintptr(unsafe.Pointer(&texture)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return texture, nil
}
