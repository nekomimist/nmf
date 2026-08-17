//go:build windows

package main

const (
	gwHWNDNext       = 2
	swpNoMove        = 0x0002
	swpNoActivate    = 0x0010
	swpNoOwnerZOrder = 0x0200
)

var (
	procGetForegroundWindow = winUser32.NewProc("GetForegroundWindow")
	procGetWindow           = winUser32.NewProc("GetWindow")
	procIsWindowVisible     = winUser32.NewProc("IsWindowVisible")
	procBeginDeferWindowPos = winUser32.NewProc("BeginDeferWindowPos")
	procDeferWindowPos      = winUser32.NewProc("DeferWindowPos")
	procEndDeferWindowPos   = winUser32.NewProc("EndDeferWindowPos")
)

func installWindowActivationHandler(runtime *ApplicationRuntime) {
	if runtime == nil || runtime.app == nil || runtime.config == nil || !runtime.config.Window.BringAllToFront {
		return
	}
	runtime.app.Lifecycle().SetOnEnteredForeground(func() {
		bringFileManagerWindowsToFront(runtime)
	})
}

func bringFileManagerWindowsToFront(runtime *ApplicationRuntime) {
	if runtime == nil || runtime.config == nil || !runtime.config.Window.BringAllToFront || runtime.windows == nil {
		return
	}
	managers := runtime.windows.snapshot()
	if len(managers) < 2 {
		return
	}

	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground == 0 {
		return
	}

	eligible := make(map[uintptr]struct{}, len(managers))
	for _, manager := range managers {
		if manager == nil || manager.window == nil {
			continue
		}
		hwnd, ok := windowHWND(manager.window)
		if !ok || !windowVisible(hwnd) || isWindowIconic(hwnd) {
			continue
		}
		eligible[hwnd] = struct{}{}
	}
	if _, ok := eligible[foreground]; !ok {
		return
	}
	delete(eligible, foreground)
	if len(eligible) == 0 {
		return
	}

	order, alreadyFront := fileManagerWindowZOrder(foreground, eligible)
	if len(order) == 0 || alreadyFront {
		return
	}
	if !deferFileManagerWindowZOrder(foreground, order) {
		return
	}
	debugPrint("FileManager: raised window group active=%#x windows=%d", foreground, len(order)+1)
}

func fileManagerWindowZOrder(foreground uintptr, eligible map[uintptr]struct{}) ([]uintptr, bool) {
	entries := make([]windowZOrderEntry, 0, len(eligible))
	remaining := len(eligible)
	found := make(map[uintptr]struct{}, len(eligible))
	seen := map[uintptr]struct{}{foreground: {}}
	current := foreground
	for remaining > 0 {
		next, _, _ := procGetWindow.Call(current, gwHWNDNext)
		if next == 0 {
			break
		}
		if _, duplicate := seen[next]; duplicate {
			break
		}
		seen[next] = struct{}{}
		current = next
		entries = append(entries, windowZOrderEntry{
			hwnd:      next,
			visible:   windowVisible(next),
			iconified: isWindowIconic(next),
		})
		if _, ok := eligible[next]; ok {
			if _, duplicate := found[next]; !duplicate {
				found[next] = struct{}{}
				remaining--
			}
		}
	}
	return selectFileManagerWindowZOrder(eligible, entries)
}

func deferFileManagerWindowZOrder(foreground uintptr, order []uintptr) bool {
	hdwp, _, err := procBeginDeferWindowPos.Call(uintptr(len(order)))
	if hdwp == 0 {
		debugPrint("FileManager: BeginDeferWindowPos for window group failed: %v", err)
		return false
	}

	insertAfter := foreground
	flags := uintptr(swpNoActivate | swpNoMove | swpNoSize | swpNoOwnerZOrder)
	for _, hwnd := range order {
		next, _, callErr := procDeferWindowPos.Call(
			hdwp,
			hwnd,
			insertAfter,
			0,
			0,
			0,
			0,
			flags,
		)
		if next == 0 {
			debugPrint("FileManager: DeferWindowPos for window group failed: %v", callErr)
			return false
		}
		hdwp = next
		insertAfter = hwnd
	}

	ret, _, err := procEndDeferWindowPos.Call(hdwp)
	if ret == 0 {
		debugPrint("FileManager: EndDeferWindowPos for window group failed: %v", err)
		return false
	}
	return true
}

func windowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}
