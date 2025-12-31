//go:build go1.18

package captured

import (
	"image"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	d3d11                 = syscall.NewLazyDLL("d3d11.dll")
	procD3D11CreateDevice = d3d11.NewProc("D3D11CreateDevice")
)

// D3D11 constants
const (
	D3D11_SDK_VERSION = 7

	D3D_DRIVER_TYPE_UNKNOWN   = 0
	D3D_DRIVER_TYPE_HARDWARE  = 1
	D3D_DRIVER_TYPE_REFERENCE = 2
	D3D_DRIVER_TYPE_NULL      = 3
	D3D_DRIVER_TYPE_SOFTWARE  = 4
	D3D_DRIVER_TYPE_WARP      = 5

	D3D11_CREATE_DEVICE_BGRA_SUPPORT = 0x20

	D3D11_USAGE_DEFAULT   = 0
	D3D11_USAGE_IMMUTABLE = 1
	D3D11_USAGE_DYNAMIC   = 2
	D3D11_USAGE_STAGING   = 3

	D3D11_CPU_ACCESS_WRITE = 0x10000
	D3D11_CPU_ACCESS_READ  = 0x20000

	D3D11_MAP_READ              = 1
	D3D11_MAP_WRITE             = 2
	D3D11_MAP_READ_WRITE        = 3
	D3D11_MAP_WRITE_DISCARD     = 4
	D3D11_MAP_WRITE_NO_OVERWRITE = 5

	DXGI_FORMAT_B8G8R8A8_UNORM = 87
)

// D3D_FEATURE_LEVEL values
const (
	D3D_FEATURE_LEVEL_9_1  = 0x9100
	D3D_FEATURE_LEVEL_9_2  = 0x9200
	D3D_FEATURE_LEVEL_9_3  = 0x9300
	D3D_FEATURE_LEVEL_10_0 = 0xa000
	D3D_FEATURE_LEVEL_10_1 = 0xa100
	D3D_FEATURE_LEVEL_11_0 = 0xb000
	D3D_FEATURE_LEVEL_11_1 = 0xb100
)

// D3D11_TEXTURE2D_DESC describes a 2D texture
type D3D11_TEXTURE2D_DESC struct {
	Width          uint32
	Height         uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleDesc     DXGI_SAMPLE_DESC
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

// DXGI_SAMPLE_DESC describes multi-sampling parameters
type DXGI_SAMPLE_DESC struct {
	Count   uint32
	Quality uint32
}

// D3D11_MAPPED_SUBRESOURCE provides access to subresource data
type D3D11_MAPPED_SUBRESOURCE struct {
	pData      unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

// ID3D11Device interface
type ID3D11Device struct {
	ole.IUnknown
}

type ID3D11DeviceVtbl struct {
	ole.IUnknownVtbl
	CreateBuffer                         uintptr
	CreateTexture1D                      uintptr
	CreateTexture2D                      uintptr
	CreateTexture3D                      uintptr
	CreateShaderResourceView             uintptr
	CreateUnorderedAccessView            uintptr
	CreateRenderTargetView               uintptr
	CreateDepthStencilView               uintptr
	CreateInputLayout                    uintptr
	CreateVertexShader                   uintptr
	CreateGeometryShader                 uintptr
	CreateGeometryShaderWithStreamOutput uintptr
	CreatePixelShader                    uintptr
	CreateHullShader                     uintptr
	CreateDomainShader                   uintptr
	CreateComputeShader                  uintptr
	CreateClassLinkage                   uintptr
	CreateBlendState                     uintptr
	CreateDepthStencilState              uintptr
	CreateRasterizerState                uintptr
	CreateSamplerState                   uintptr
	CreateQuery                          uintptr
	CreatePredicate                      uintptr
	CreateCounter                        uintptr
	CreateDeferredContext                uintptr
	OpenSharedResource                   uintptr
	CheckFormatSupport                   uintptr
	CheckMultisampleQualityLevels        uintptr
	CheckCounterInfo                     uintptr
	CheckCounter                         uintptr
	CheckFeatureSupport                  uintptr
	GetPrivateData                       uintptr
	SetPrivateData                       uintptr
	SetPrivateDataInterface              uintptr
	GetFeatureLevel                      uintptr
	GetCreationFlags                     uintptr
	GetDeviceRemovedReason               uintptr
	GetImmediateContext                  uintptr
	SetExceptionMode                     uintptr
	GetExceptionMode                     uintptr
}

func (d *ID3D11Device) vtbl() *ID3D11DeviceVtbl {
	return (*ID3D11DeviceVtbl)(unsafe.Pointer(d.RawVTable))
}

func (d *ID3D11Device) CreateTexture2D(desc *D3D11_TEXTURE2D_DESC, initialData unsafe.Pointer) (*ID3D11Texture2D, error) {
	var texture *ID3D11Texture2D
	hr, _, _ := syscall.SyscallN(
		d.vtbl().CreateTexture2D,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(desc)),
		uintptr(initialData),
		uintptr(unsafe.Pointer(&texture)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return texture, nil
}

func (d *ID3D11Device) GetImmediateContext() *ID3D11DeviceContext {
	var context *ID3D11DeviceContext
	syscall.SyscallN(
		d.vtbl().GetImmediateContext,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(&context)),
	)
	return context
}

// ID3D11DeviceContext interface
type ID3D11DeviceContext struct {
	ole.IUnknown
}

type ID3D11DeviceContextVtbl struct {
	ole.IUnknownVtbl
	GetDevice                                 uintptr
	GetPrivateData                            uintptr
	SetPrivateData                            uintptr
	SetPrivateDataInterface                   uintptr
	VSSetConstantBuffers                      uintptr
	PSSetShaderResources                      uintptr
	PSSetShader                               uintptr
	PSSetSamplers                             uintptr
	VSSetShader                               uintptr
	DrawIndexed                               uintptr
	Draw                                      uintptr
	Map                                       uintptr
	Unmap                                     uintptr
	PSSetConstantBuffers                      uintptr
	IASetInputLayout                          uintptr
	IASetVertexBuffers                        uintptr
	IASetIndexBuffer                          uintptr
	DrawIndexedInstanced                      uintptr
	DrawInstanced                             uintptr
	GSSetConstantBuffers                      uintptr
	GSSetShader                               uintptr
	IASetPrimitiveTopology                    uintptr
	VSSetShaderResources                      uintptr
	VSSetSamplers                             uintptr
	Begin                                     uintptr
	End                                       uintptr
	GetData                                   uintptr
	SetPredication                            uintptr
	GSSetShaderResources                      uintptr
	GSSetSamplers                             uintptr
	OMSetRenderTargets                        uintptr
	OMSetRenderTargetsAndUnorderedAccessViews uintptr
	OMSetBlendState                           uintptr
	OMSetDepthStencilState                    uintptr
	SOSetTargets                              uintptr
	DrawAuto                                  uintptr
	DrawIndexedInstancedIndirect              uintptr
	DrawInstancedIndirect                     uintptr
	Dispatch                                  uintptr
	DispatchIndirect                          uintptr
	RSSetState                                uintptr
	RSSetViewports                            uintptr
	RSSetScissorRects                         uintptr
	CopySubresourceRegion                     uintptr
	CopyResource                              uintptr
	UpdateSubresource                         uintptr
	CopyStructureCount                        uintptr
	ClearRenderTargetView                     uintptr
	ClearUnorderedAccessViewUint              uintptr
	ClearUnorderedAccessViewFloat             uintptr
	ClearDepthStencilView                     uintptr
	GenerateMips                              uintptr
	SetResourceMinLOD                         uintptr
	GetResourceMinLOD                         uintptr
	ResolveSubresource                        uintptr
	ExecuteCommandList                        uintptr
	HSSetShaderResources                      uintptr
	HSSetShader                               uintptr
	HSSetSamplers                             uintptr
	HSSetConstantBuffers                      uintptr
	DSSetShaderResources                      uintptr
	DSSetShader                               uintptr
	DSSetSamplers                             uintptr
	DSSetConstantBuffers                      uintptr
	CSSetShaderResources                      uintptr
	CSSetUnorderedAccessViews                 uintptr
	CSSetShader                               uintptr
	CSSetSamplers                             uintptr
	CSSetConstantBuffers                      uintptr
	VSGetConstantBuffers                      uintptr
	PSGetShaderResources                      uintptr
	PSGetShader                               uintptr
	PSGetSamplers                             uintptr
	VSGetShader                               uintptr
	PSGetConstantBuffers                      uintptr
	IAGetInputLayout                          uintptr
	IAGetVertexBuffers                        uintptr
	IAGetIndexBuffer                          uintptr
	GSGetConstantBuffers                      uintptr
	GSGetShader                               uintptr
	IAGetPrimitiveTopology                    uintptr
	VSGetShaderResources                      uintptr
	VSGetSamplers                             uintptr
	GetPredication                            uintptr
	GSGetShaderResources                      uintptr
	GSGetSamplers                             uintptr
	OMGetRenderTargets                        uintptr
	OMGetRenderTargetsAndUnorderedAccessViews uintptr
	OMGetBlendState                           uintptr
	OMGetDepthStencilState                    uintptr
	SOGetTargets                              uintptr
	RSGetState                                uintptr
	RSGetViewports                            uintptr
	RSGetScissorRects                         uintptr
	HSGetShaderResources                      uintptr
	HSGetShader                               uintptr
	HSGetSamplers                             uintptr
	HSGetConstantBuffers                      uintptr
	DSGetShaderResources                      uintptr
	DSGetShader                               uintptr
	DSGetSamplers                             uintptr
	DSGetConstantBuffers                      uintptr
	CSGetShaderResources                      uintptr
	CSGetUnorderedAccessViews                 uintptr
	CSGetShader                               uintptr
	CSGetSamplers                             uintptr
	CSGetConstantBuffers                      uintptr
	ClearState                                uintptr
	Flush                                     uintptr
	GetType                                   uintptr
	GetContextFlags                           uintptr
	FinishCommandList                         uintptr
}

func (d *ID3D11DeviceContext) vtbl() *ID3D11DeviceContextVtbl {
	return (*ID3D11DeviceContextVtbl)(unsafe.Pointer(d.RawVTable))
}

func (d *ID3D11DeviceContext) CopyResource(dst, src *ID3D11Texture2D) {
	syscall.SyscallN(
		d.vtbl().CopyResource,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(unsafe.Pointer(src)),
	)
}

func (d *ID3D11DeviceContext) Map(resource *ID3D11Texture2D, subresource uint32, mapType uint32, mapFlags uint32) (*D3D11_MAPPED_SUBRESOURCE, error) {
	var mapped D3D11_MAPPED_SUBRESOURCE
	hr, _, _ := syscall.SyscallN(
		d.vtbl().Map,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subresource),
		uintptr(mapType),
		uintptr(mapFlags),
		uintptr(unsafe.Pointer(&mapped)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return &mapped, nil
}

func (d *ID3D11DeviceContext) Unmap(resource *ID3D11Texture2D, subresource uint32) {
	syscall.SyscallN(
		d.vtbl().Unmap,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(resource)),
		uintptr(subresource),
	)
}

// ID3D11Texture2D interface
type ID3D11Texture2D struct {
	ole.IUnknown
}

type ID3D11Texture2DVtbl struct {
	ole.IUnknownVtbl
	GetDevice              uintptr
	GetPrivateData         uintptr
	SetPrivateData         uintptr
	SetPrivateDataInterface uintptr
	GetType                uintptr
	SetEvictionPriority    uintptr
	GetEvictionPriority    uintptr
	GetDesc                uintptr
}

func (t *ID3D11Texture2D) vtbl() *ID3D11Texture2DVtbl {
	return (*ID3D11Texture2DVtbl)(unsafe.Pointer(t.RawVTable))
}

func (t *ID3D11Texture2D) GetDesc() D3D11_TEXTURE2D_DESC {
	var desc D3D11_TEXTURE2D_DESC
	syscall.SyscallN(
		t.vtbl().GetDesc,
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(&desc)),
	)
	return desc
}

// Cached D3D11 device
var (
	cachedDevice     *ID3D11Device
	cachedDeviceMu   sync.Mutex
	cachedDeviceOnce sync.Once
)

// getD3D11Device returns a cached D3D11 device, creating one if necessary
func getD3D11Device() (*ID3D11Device, error) {
	cachedDeviceMu.Lock()
	defer cachedDeviceMu.Unlock()

	if cachedDevice != nil {
		return cachedDevice, nil
	}

	var device *ID3D11Device
	var featureLevel uint32

	featureLevels := []uint32{
		D3D_FEATURE_LEVEL_11_1,
		D3D_FEATURE_LEVEL_11_0,
		D3D_FEATURE_LEVEL_10_1,
		D3D_FEATURE_LEVEL_10_0,
	}

	hr, _, _ := syscall.SyscallN(
		procD3D11CreateDevice.Addr(),
		0,                            // pAdapter
		D3D_DRIVER_TYPE_HARDWARE,     // DriverType
		0,                            // Software
		D3D11_CREATE_DEVICE_BGRA_SUPPORT, // Flags
		uintptr(unsafe.Pointer(&featureLevels[0])), // pFeatureLevels
		uintptr(len(featureLevels)),  // FeatureLevels
		D3D11_SDK_VERSION,            // SDKVersion
		uintptr(unsafe.Pointer(&device)), // ppDevice
		uintptr(unsafe.Pointer(&featureLevel)), // pFeatureLevel
		0, // ppImmediateContext (we'll get it from device)
	)

	if hr != 0 {
		return nil, ole.NewError(hr)
	}

	cachedDevice = device
	return cachedDevice, nil
}

// createStagingTexture creates a CPU-readable staging texture matching the source
func createStagingTexture(device *ID3D11Device, width, height uint32) (*ID3D11Texture2D, error) {
	desc := D3D11_TEXTURE2D_DESC{
		Width:          width,
		Height:         height,
		MipLevels:      1,
		ArraySize:      1,
		Format:         DXGI_FORMAT_B8G8R8A8_UNORM,
		SampleDesc:     DXGI_SAMPLE_DESC{Count: 1, Quality: 0},
		Usage:          D3D11_USAGE_STAGING,
		BindFlags:      0,
		CPUAccessFlags: D3D11_CPU_ACCESS_READ,
		MiscFlags:      0,
	}

	return device.CreateTexture2D(&desc, nil)
}

// copyTextureToImage copies a staging texture to an image.RGBA
func copyTextureToImage(context *ID3D11DeviceContext, texture *ID3D11Texture2D, width, height int) (*image.RGBA, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mapped, err := context.Map(texture, 0, D3D11_MAP_READ, 0)
	if err != nil {
		return nil, err
	}
	defer context.Unmap(texture, 0)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	rowPitch := int(mapped.RowPitch)
	src := uintptr(mapped.pData)

	for y := 0; y < height; y++ {
		rowStart := src + uintptr(y*rowPitch)
		for x := 0; x < width; x++ {
			pixelOffset := rowStart + uintptr(x*4)
			b := *(*uint8)(unsafe.Pointer(pixelOffset))
			g := *(*uint8)(unsafe.Pointer(pixelOffset + 1))
			r := *(*uint8)(unsafe.Pointer(pixelOffset + 2))
			a := *(*uint8)(unsafe.Pointer(pixelOffset + 3))

			i := (y*width + x) * 4
			// BGRA => RGBA
			img.Pix[i] = r
			img.Pix[i+1] = g
			img.Pix[i+2] = b
			img.Pix[i+3] = a
		}
	}

	return img, nil
}

// releaseD3D11Device releases the cached D3D11 device
func releaseD3D11Device() {
	cachedDeviceMu.Lock()
	defer cachedDeviceMu.Unlock()

	if cachedDevice != nil {
		cachedDevice.Release()
		cachedDevice = nil
	}
}
