//go:build windows

package main

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
	"golang.org/x/sys/windows"

	"nmf/internal/config"
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
	procIsWindowArranged   = winUser32.NewProc("IsWindowArranged")
	procIsZoomed           = winUser32.NewProc("IsZoomed")
	procShowWindow         = winUser32.NewProc("ShowWindow")
	procSetWindowPos       = winUser32.NewProc("SetWindowPos")
	procMonitorFromWindow  = winUser32.NewProc("MonitorFromWindow")
	procMonitorFromPoint   = winUser32.NewProc("MonitorFromPoint")
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

func requestInitialWindowPosition(window fyne.Window, cfg config.WindowConfig) {
	if cfg.X == nil || cfg.Y == nil {
		return
	}
	desktopWindow, ok := window.(desktop.Window)
	if !ok {
		debugPrint("FileManager: Desktop window position request unavailable")
		return
	}

	desktopWindow.RequestPosition(*cfg.X, *cfg.Y)
	debugPrint("FileManager: requested initial window position x=%d y=%d", *cfg.X, *cfg.Y)
}

func applyInitialWindowPosition(window fyne.Window, cfg config.WindowConfig) {
	if cfg.X == nil || cfg.Y == nil {
		return
	}
	hwnd, ok := windowHWND(window)
	if !ok {
		debugPrint("FileManager: HWND unavailable for configured window position")
		return
	}
	rect, ok := getWindowRect(hwnd)
	if !ok {
		debugPrint("FileManager: Window rect unavailable for configured window position")
		return
	}
	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	workRect := monitorWorkRectForPoint(int32(*cfg.X), int32(*cfg.Y))
	x, y := selectWindowPositionInWorkRect(int32(*cfg.X), int32(*cfg.Y), width, height, windowSwitchRectFromWinRect(workRect))

	ret, _, err := procSetWindowPos.Call(
		hwnd,
		0,
		uintptr(x),
		uintptr(y),
		0,
		0,
		swpNoSize|swpNoZOrder,
	)
	if ret == 0 {
		debugPrint("FileManager: SetWindowPos for configured position failed: %v", err)
		return
	}
	debugPrint("FileManager: applied configured window position requested_x=%d requested_y=%d x=%d y=%d", *cfg.X, *cfg.Y, x, y)
}

func positionWindowNextTo(runtime *ApplicationRuntime, parent, child fyne.Window) {
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
	occupied := fileManagerWindowPlacementRects(runtime, parent, child)
	parentSwitchRect := windowSwitchRectFromWinRect(parentRect)
	workSwitchRect := windowSwitchRectFromWinRect(workRect)
	plan := planWindowPlacement(
		parentSwitchRect,
		childWidth,
		childHeight,
		workSwitchRect,
		occupied,
		false,
	)
	if plan.Side == windowPlacementFallback && sourceWindowMovementEnabled(runtime) && singleRegisteredParent(runtime, parent, child) && windowMovableForPlacement(parentHWND) {
		plan = planWindowPlacement(
			parentSwitchRect,
			childWidth,
			childHeight,
			workSwitchRect,
			occupied,
			true,
		)
	}

	if plan.MoveParent {
		if positionWindowPair(parentHWND, childHWND, parentRect.Top, plan) {
			debugPrint("FileManager: Positioned window pair parent_x=%d child_x=%d y=%d side=%s", plan.ParentX, plan.ChildX, plan.ChildY, plan.Side)
			return
		}
		plan = planWindowPlacement(
			parentSwitchRect,
			childWidth,
			childHeight,
			workSwitchRect,
			occupied,
			false,
		)
	}

	ret, _, err := procSetWindowPos.Call(
		childHWND,
		0,
		uintptr(plan.ChildX),
		uintptr(plan.ChildY),
		0,
		0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
	if ret == 0 {
		debugPrint("FileManager: SetWindowPos failed: %v", err)
		return
	}
	debugPrint("FileManager: Positioned new window x=%d y=%d side=%s", plan.ChildX, plan.ChildY, plan.Side)
}

func sourceWindowMovementEnabled(runtime *ApplicationRuntime) bool {
	return runtime != nil && runtime.config != nil && runtime.config.Window.MoveSourceOnNewWindow
}

func singleRegisteredParent(runtime *ApplicationRuntime, parent, child fyne.Window) bool {
	if runtime == nil || runtime.windows == nil {
		return false
	}
	parents := 0
	foundParent := false
	for _, manager := range runtime.windows.snapshot() {
		if manager == nil || manager.window == nil || manager.window == child {
			continue
		}
		parents++
		if manager.window == parent {
			foundParent = true
		}
	}
	return foundParent && parents == 1
}

func windowMovableForPlacement(hwnd uintptr) bool {
	if isWindowIconic(hwnd) {
		return false
	}
	zoomed, _, _ := procIsZoomed.Call(hwnd)
	if zoomed != 0 {
		return false
	}
	if err := procIsWindowArranged.Find(); err != nil {
		debugPrint("FileManager: snapped window state unavailable; keeping parent position")
		return false
	}
	arranged, _, _ := procIsWindowArranged.Call(hwnd)
	return arranged == 0
}

func positionWindowPair(parentHWND, childHWND uintptr, parentY int32, plan windowPlacementPlan) bool {
	hdwp, _, err := procBeginDeferWindowPos.Call(2)
	if hdwp == 0 {
		debugPrint("FileManager: BeginDeferWindowPos for window pair failed: %v", err)
		return false
	}
	flags := uintptr(swpNoSize | swpNoZOrder | swpNoActivate)
	hdwp, _, err = procDeferWindowPos.Call(
		hdwp,
		parentHWND,
		0,
		uintptr(plan.ParentX),
		uintptr(parentY),
		0,
		0,
		flags,
	)
	if hdwp == 0 {
		debugPrint("FileManager: DeferWindowPos for parent placement failed: %v", err)
		return false
	}
	hdwp, _, err = procDeferWindowPos.Call(
		hdwp,
		childHWND,
		0,
		uintptr(plan.ChildX),
		uintptr(plan.ChildY),
		0,
		0,
		flags,
	)
	if hdwp == 0 {
		debugPrint("FileManager: DeferWindowPos for child placement failed: %v", err)
		return false
	}
	ret, _, err := procEndDeferWindowPos.Call(hdwp)
	if ret == 0 {
		debugPrint("FileManager: EndDeferWindowPos for window pair failed: %v", err)
		return false
	}
	return true
}

func fileManagerWindowPlacementRects(runtime *ApplicationRuntime, parent, child fyne.Window) []windowSwitchRect {
	var managers []*FileManager
	if runtime != nil && runtime.windows != nil {
		managers = runtime.windows.snapshot()
	}
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
	return monitorWorkRectFromHandle(monitor)
}

func monitorWorkRectForPoint(x, y int32) winRect {
	point := winPoint{X: x, Y: y}
	monitor, _, _ := procMonitorFromPoint.Call(uintptr(*(*uint64)(unsafe.Pointer(&point))), monitorDefaultToNearest)
	return monitorWorkRectFromHandle(monitor)
}

func monitorWorkRectFromHandle(monitor uintptr) winRect {
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
