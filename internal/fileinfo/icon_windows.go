//go:build windows

package fileinfo

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
)

// Minimal Windows icon extraction using SHGetFileInfo and GDI to render HICON into a 32-bit DIB.

var (
	modShell32         = syscall.NewLazyDLL("shell32.dll")
	procSHGetFileInfoW = modShell32.NewProc("SHGetFileInfoW")

	modUser32       = syscall.NewLazyDLL("user32.dll")
	procDestroyIcon = modUser32.NewProc("DestroyIcon")
	procDrawIconEx  = modUser32.NewProc("DrawIconEx")

	modGdi32               = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modGdi32.NewProc("DeleteDC")
	procCreateDIBSection   = modGdi32.NewProc("CreateDIBSection")
	procSelectObject       = modGdi32.NewProc("SelectObject")
	procDeleteObject       = modGdi32.NewProc("DeleteObject")
)

const (
	SHGFI_ICON              = 0x000000100
	SHGFI_USEFILEATTRIBUTES = 0x000000010
	SHGFI_LARGEICON         = 0x000000000
	SHGFI_SMALLICON         = 0x000000001

	FILE_ATTRIBUTE_NORMAL = 0x00000080

	DI_NORMAL      = 0x0003
	DIB_RGB_COLORS = 0
	BI_RGB         = 0
)

type shfileinfoW struct {
	hIcon         syscall.Handle
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [260]uint16
	szTypeName    [80]uint16
}

type bitmapinfoheader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapinfo struct {
	bmiHeader bitmapinfoheader
	// We don't need color table for 32-bit BI_RGB
}

func platformFetchExtIcon(ext string, size int) (fyne.Resource, error) {
	hicon, err := getHICONForExt(ext, size)
	if err != nil || hicon == 0 {
		return nil, err
	}
	defer destroyIcon(hicon)
	img, err := renderHICONToImage(hicon, iconSizeFor(size))
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return fyne.NewStaticResource("ext:"+ext, buf.Bytes()), nil
}

func platformFetchFileIcon(path string, size int) (fyne.Resource, error) {
	hicon, err := getHICONForPath(path, size)
	if err != nil || hicon == 0 {
		return nil, err
	}
	defer destroyIcon(hicon)
	img, err := renderHICONToImage(hicon, iconSizeFor(size))
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return fyne.NewStaticResource("file:"+path, buf.Bytes()), nil
}

// preferFileIcon returns true for types where a file-specific icon is beneficial.
// EXE/COM/BAT/CMD and LNK typically embed or resolve to unique icons.
func preferFileIcon(path, ext string) bool {
	switch strings.ToLower(ext) {
	case ".exe", ".lnk", ".ico":
		// .exe: embedded app icon; .lnk: should resolve/display target’s icon; .ico: file itself is an icon
		return true
	default:
		return false
	}
}

func iconSizeFor(size int) int {
	if size <= 16 {
		return 16
	}
	if size <= 24 {
		return 24
	}
	if size <= 32 {
		return 32
	}
	return 32
}

func getHICONForExt(ext string, size int) (syscall.Handle, error) {
	if ext == "" {
		return 0, fmt.Errorf("empty extension")
	}
	var sfi shfileinfoW
	flags := uint32(SHGFI_ICON | SHGFI_USEFILEATTRIBUTES)
	if iconSizeFor(size) <= 16 {
		flags |= SHGFI_SMALLICON
	} else {
		flags |= SHGFI_LARGEICON
	}
	pExt, _ := syscall.UTF16PtrFromString(ext)
	r1, _, e1 := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pExt)),
		uintptr(FILE_ATTRIBUTE_NORMAL),
		uintptr(unsafe.Pointer(&sfi)),
		uintptr(uint32(unsafe.Sizeof(sfi))),
		uintptr(flags),
	)
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return 0, e1
		}
		return 0, fmt.Errorf("SHGetFileInfoW failed for ext %s", ext)
	}
	return sfi.hIcon, nil
}

func getHICONForPath(path string, size int) (syscall.Handle, error) {
	var sfi shfileinfoW
	flags := uint32(SHGFI_ICON)
	if iconSizeFor(size) <= 16 {
		flags |= SHGFI_SMALLICON
	} else {
		flags |= SHGFI_LARGEICON
	}
	pPath, _ := syscall.UTF16PtrFromString(path)
	r1, _, e1 := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pPath)),
		0,
		uintptr(unsafe.Pointer(&sfi)),
		uintptr(uint32(unsafe.Sizeof(sfi))),
		uintptr(flags),
	)
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return 0, e1
		}
		return 0, fmt.Errorf("SHGetFileInfoW failed for path")
	}
	return sfi.hIcon, nil
}

func renderHICONToImage(hicon syscall.Handle, size int) (image.Image, error) {
	// Create memory DC
	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdc)

	// Prepare 32-bit top-down DIB section
	var bmi bitmapinfo
	bmi.bmiHeader.biSize = uint32(unsafe.Sizeof(bmi.bmiHeader))
	bmi.bmiHeader.biWidth = int32(size)
	bmi.bmiHeader.biHeight = -int32(size) // top-down
	bmi.bmiHeader.biPlanes = 1
	bmi.bmiHeader.biBitCount = 32
	bmi.bmiHeader.biCompression = BI_RGB

	var bits unsafe.Pointer
	hbmp, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbmp == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hbmp)

	// Select bitmap into DC
	oldObj, _, _ := procSelectObject.Call(hdc, hbmp)
	defer func() {
		if oldObj != 0 {
			procSelectObject.Call(hdc, oldObj)
		}
	}()

	// Draw the icon
	ok, _, _ := procDrawIconEx.Call(
		hdc,
		0, 0,
		uintptr(hicon),
		uintptr(size), uintptr(size),
		0,
		0,
		DI_NORMAL,
	)
	if ok == 0 {
		return nil, fmt.Errorf("DrawIconEx failed")
	}

	// Read pixels (BGRA) and convert to RGBA
	stride := size * 4
	buf := CGoBytes(bits, size*stride)
	// Convert to image.RGBA
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// BGRA -> RGBA
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := y*stride + x*4
			b := buf[i+0]
			g := buf[i+1]
			r := buf[i+2]
			a := buf[i+3]
			oi := img.PixOffset(x, y)
			img.Pix[oi+0] = r
			img.Pix[oi+1] = g
			img.Pix[oi+2] = b
			img.Pix[oi+3] = a
		}
	}
	return img, nil
}

func destroyIcon(h syscall.Handle) {
	if h != 0 {
		procDestroyIcon.Call(uintptr(h))
	}
}

// CGoBytes converts C memory to a Go byte slice without using cgo by copying.
func CGoBytes(p unsafe.Pointer, n int) []byte {
	if p == nil || n <= 0 {
		return nil
	}
	var b = make([]byte, n)
	src := (*[1 << 30]byte)(p)[:n:n]
	copy(b, src)
	return b
}
