//go:build windows

package main

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

const (
	monitorDefaultToNearest = 2
	swRestore               = 9

	swpNoSize   = 0x0001
	swpNoZOrder = 0x0004
)

var (
	winUser32              = windows.NewLazySystemDLL("user32.dll")
	procGetWindowPlacement = winUser32.NewProc("GetWindowPlacement")
	procGetWindowRect      = winUser32.NewProc("GetWindowRect")
	procIsIconic           = winUser32.NewProc("IsIconic")
	procShowWindow         = winUser32.NewProc("ShowWindow")
	procSetWindowPos       = winUser32.NewProc("SetWindowPos")
	procMonitorFromWindow  = winUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW    = winUser32.NewProc("GetMonitorInfoW")
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type winPoint struct {
	X int32
	Y int32
}

type winMonitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

type winWindowPlacement struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    winPoint
	PtMaxPosition    winPoint
	RcNormalPosition winRect
}

func positionWindowNextTo(parent, child fyne.Window) {
	parentHWND, ok := windowHWND(parent)
	if !ok {
		debugPrint("FileManager: Parent HWND unavailable for window placement")
		return
	}
	childHWND, ok := windowHWND(child)
	if !ok {
		debugPrint("FileManager: Child HWND unavailable for window placement")
		return
	}

	parentRect, ok := getWindowRect(parentHWND)
	if !ok {
		debugPrint("FileManager: Parent window rect unavailable for window placement")
		return
	}
	childRect, ok := getWindowRect(childHWND)
	if !ok {
		debugPrint("FileManager: Child window rect unavailable for window placement")
		return
	}

	workRect := monitorWorkRect(parentHWND)
	childWidth := childRect.Right - childRect.Left
	childHeight := childRect.Bottom - childRect.Top
	occupied := fileManagerWindowPlacementRects(parent, child)
	x, y, side := selectWindowPlacement(
		windowSwitchRectFromWinRect(parentRect),
		childWidth,
		childHeight,
		windowSwitchRectFromWinRect(workRect),
		occupied,
	)

	ret, _, err := procSetWindowPos.Call(
		childHWND,
		0,
		uintptr(x),
		uintptr(y),
		0,
		0,
		swpNoSize|swpNoZOrder,
	)
	if ret == 0 {
		debugPrint("FileManager: SetWindowPos failed: %v", err)
		return
	}
	debugPrint("FileManager: Positioned new window x=%d y=%d side=%s", x, y, side)
}

func fileManagerWindowPlacementRects(parent, child fyne.Window) []windowSwitchRect {
	managers := snapshotFileManagerWindows()
	rects := make([]windowSwitchRect, 0, len(managers))
	for _, manager := range managers {
		if manager == nil || manager.window == nil || manager.window == parent || manager.window == child {
			continue
		}
		rect, ok := platformWindowSwitchRect(manager.window)
		if !ok {
			continue
		}
		rects = append(rects, rect)
	}
	return rects
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

func getWindowRect(hwnd uintptr) (winRect, bool) {
	var rect winRect
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	return rect, ret != 0
}

func getWindowPlacement(hwnd uintptr) (winWindowPlacement, bool) {
	placement := winWindowPlacement{Length: uint32(unsafe.Sizeof(winWindowPlacement{}))}
	ret, _, _ := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&placement)))
	return placement, ret != 0
}

func restoreWindowBeforeFocus(window fyne.Window) {
	hwnd, ok := windowHWND(window)
	if !ok {
		return
	}

	if !isWindowIconic(hwnd) {
		return
	}

	ret, _, err := procShowWindow.Call(hwnd, swRestore)
	if ret == 0 {
		debugPrint("FileManager: ShowWindow restore returned false: %v", err)
		return
	}
	debugPrint("FileManager: restored iconified window before focus")
}

func isWindowIconic(hwnd uintptr) bool {
	ret, _, _ := procIsIconic.Call(hwnd)
	return ret != 0
}

func monitorWorkRect(hwnd uintptr) winRect {
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if monitor == 0 {
		return winRect{
			Left:   -32000,
			Top:    -32000,
			Right:  32000,
			Bottom: 32000,
		}
	}

	info := winMonitorInfo{CbSize: uint32(unsafe.Sizeof(winMonitorInfo{}))}
	ret, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return winRect{
			Left:   -32000,
			Top:    -32000,
			Right:  32000,
			Bottom: 32000,
		}
	}
	return info.RcWork
}
