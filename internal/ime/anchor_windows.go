//go:build windows

package ime

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

const (
	cfsPoint   = 0x0002
	cfsExclude = 0x0080
)

var (
	imm32                    = windows.NewLazySystemDLL("imm32.dll")
	procImmGetContext        = imm32.NewProc("ImmGetContext")
	procImmReleaseContext    = imm32.NewProc("ImmReleaseContext")
	procImmSetCompositionWin = imm32.NewProc("ImmSetCompositionWindow")
	procImmSetCandidateWin   = imm32.NewProc("ImmSetCandidateWindow")
)

type winPoint struct {
	X int32
	Y int32
}

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type compositionForm struct {
	DwStyle      uint32
	PtCurrentPos winPoint
	RcArea       winRect
}

type candidateForm struct {
	DwIndex      uint32
	DwStyle      uint32
	PtCurrentPos winPoint
	RcArea       winRect
}

func setAnchor(window fyne.Window, object fyne.CanvasObject, local fyne.Position, size fyne.Size) bool {
	hwnd, ok := windowHWND(window)
	if !ok {
		return false
	}
	point, rect, ok := clientAnchor(object, local, size)
	if !ok {
		return false
	}

	himc, _, _ := procImmGetContext.Call(hwnd)
	if himc == 0 {
		return false
	}
	defer procImmReleaseContext.Call(hwnd, himc)

	comp := compositionForm{
		DwStyle:      cfsPoint,
		PtCurrentPos: point,
		RcArea:       rect,
	}
	cand := candidateForm{
		DwStyle:      cfsExclude,
		PtCurrentPos: point,
		RcArea:       rect,
	}
	compOK, _, _ := procImmSetCompositionWin.Call(himc, uintptr(unsafe.Pointer(&comp)))
	candOK, _, _ := procImmSetCandidateWin.Call(himc, uintptr(unsafe.Pointer(&cand)))
	return compOK != 0 || candOK != 0
}

func windowHWND(window fyne.Window) (uintptr, bool) {
	nativeWindow, ok := window.(driver.NativeWindow)
	if !ok {
		return 0, false
	}

	var hwnd uintptr
	nativeWindow.RunNative(func(context any) {
		winCtx, ok := context.(driver.WindowsWindowContext)
		if !ok {
			return
		}
		hwnd = winCtx.HWND
	})
	return hwnd, hwnd != 0
}

func clientAnchor(object fyne.CanvasObject, local fyne.Position, size fyne.Size) (winPoint, winRect, bool) {
	if object == nil {
		return winPoint{}, winRect{}, false
	}
	if size.Width < 1 {
		size.Width = 1
	}
	if size.Height < 1 {
		size.Height = 1
	}

	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return winPoint{}, winRect{}, false
	}
	canvas := app.Driver().CanvasForObject(object)
	if canvas == nil {
		return winPoint{}, winRect{}, false
	}

	abs := app.Driver().AbsolutePositionForObject(object).Add(local)
	x, y := canvas.PixelCoordinateForPosition(abs)
	right, bottom := canvas.PixelCoordinateForPosition(abs.AddXY(size.Width, size.Height))
	if right <= x {
		right = x + 1
	}
	if bottom <= y {
		bottom = y + 1
	}

	point := winPoint{X: int32(x), Y: int32(y)}
	rect := winRect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(right),
		Bottom: int32(bottom),
	}
	return point, rect, true
}
