package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

type windowSwitchDirection int

const (
	windowSwitchLeft  windowSwitchDirection = -1
	windowSwitchRight windowSwitchDirection = 1
)

type windowSwitchRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type windowSwitchCandidate struct {
	manager *FileManager
	rect    windowSwitchRect
	hasRect bool
}

func registerFileManagerWindow(fm *FileManager) {
	if fm == nil || fm.window == nil {
		return
	}
	windowRegistry.Store(fm.window, fm)

	windowOrderMu.Lock()
	defer windowOrderMu.Unlock()
	windowOrder = append(windowOrder, fm)
}

func unregisterFileManagerWindow(fm *FileManager) {
	if fm == nil {
		return
	}
	if fm.window != nil {
		windowRegistry.Delete(fm.window)
	}

	windowOrderMu.Lock()
	defer windowOrderMu.Unlock()
	for i, candidate := range windowOrder {
		if candidate == fm {
			windowOrder = append(windowOrder[:i], windowOrder[i+1:]...)
			return
		}
	}
}

func recordReopenPath(path string) {
	if path == "" {
		return
	}

	windowOrderMu.Lock()
	defer windowOrderMu.Unlock()
	reopenPaths = append(reopenPaths, path)
}

func nextReopenPath() (string, bool) {
	windowOrderMu.Lock()
	defer windowOrderMu.Unlock()
	if len(reopenPaths) == 0 {
		return "", false
	}

	path := reopenPaths[0]
	reopenPaths = append(reopenPaths[:0], reopenPaths[1:]...)
	return path, true
}

func snapshotFileManagerWindows() []*FileManager {
	windowOrderMu.Lock()
	defer windowOrderMu.Unlock()

	snapshot := make([]*FileManager, 0, len(windowOrder))
	for _, fm := range windowOrder {
		if fm != nil && fm.window != nil {
			snapshot = append(snapshot, fm)
		}
	}
	return snapshot
}

func (fm *FileManager) FocusWindowLeft() {
	fm.focusNeighborWindow(windowSwitchLeft)
}

func (fm *FileManager) FocusWindowRight() {
	fm.focusNeighborWindow(windowSwitchRight)
}

func (fm *FileManager) focusNeighborWindow(direction windowSwitchDirection) {
	target := fm.neighborWindow(direction)
	if target == nil {
		debugPrint("FileManager: window switch target unavailable direction=%d", direction)
		return
	}

	if windowFocusUnsupported(target.window) {
		debugPrint("FileManager: window switch target selected direction=%d target=%s focus=unsupported-wayland", direction, target.currentPath)
		return
	}

	debugPrint("FileManager: window switch direction=%d target=%s", direction, target.currentPath)
	restoreWindowBeforeFocus(target.window)
	target.window.Show()
	target.window.RequestFocus()
	target.FocusFileList()
}

func (fm *FileManager) neighborWindow(direction windowSwitchDirection) *FileManager {
	windows := snapshotFileManagerWindows()
	if len(windows) < 2 {
		return nil
	}

	current := -1
	candidates := make([]windowSwitchCandidate, 0, len(windows))
	for _, candidate := range windows {
		if candidate == fm {
			current = len(candidates)
		}
		candidates = append(candidates, windowSwitchCandidate{manager: candidate})
	}
	if current < 0 {
		return nil
	}

	applyWindowSwitchRects(candidates)
	index, ok := selectWindowSwitchCandidate(candidates, current, direction)
	if !ok {
		return nil
	}
	return candidates[index].manager
}

func selectWindowSwitchCandidate(candidates []windowSwitchCandidate, current int, direction windowSwitchDirection) (int, bool) {
	if current < 0 || current >= len(candidates) || len(candidates) < 2 {
		return -1, false
	}
	for _, candidate := range candidates {
		if !candidate.hasRect {
			return selectWindowByOrder(candidates, current, direction)
		}
	}

	currentX, currentY := windowSwitchRectCenter(candidates[current].rect)
	bestIndex := -1
	var bestDistance int64
	var bestYDistance int64
	for i, candidate := range candidates {
		if i == current || !candidate.hasRect {
			continue
		}
		candidateX, candidateY := windowSwitchRectCenter(candidate.rect)
		deltaX := candidateX - currentX
		if direction == windowSwitchLeft && deltaX >= 0 {
			continue
		}
		if direction == windowSwitchRight && deltaX <= 0 {
			continue
		}

		distance := absInt64(deltaX)
		yDistance := absInt64(candidateY - currentY)
		if bestIndex < 0 || distance < bestDistance || (distance == bestDistance && yDistance < bestYDistance) {
			bestIndex = i
			bestDistance = distance
			bestYDistance = yDistance
		}
	}
	if bestIndex >= 0 {
		return bestIndex, true
	}
	return -1, false
}

func selectWindowByOrder(candidates []windowSwitchCandidate, current int, direction windowSwitchDirection) (int, bool) {
	if direction == windowSwitchLeft {
		if current <= 0 {
			return -1, false
		}
		return current - 1, true
	}
	if direction == windowSwitchRight {
		if current >= len(candidates)-1 {
			return -1, false
		}
		return current + 1, true
	}
	return -1, false
}

func windowSwitchRectCenter(rect windowSwitchRect) (int64, int64) {
	return int64(rect.Left+rect.Right) / 2, int64(rect.Top+rect.Bottom) / 2
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func fyneWindowSwitchRect(window fyne.Window) (windowSwitchRect, bool) {
	return platformWindowSwitchRect(window)
}

func windowFocusUnsupported(window fyne.Window) bool {
	nativeWindow, ok := window.(driver.NativeWindow)
	if !ok {
		return false
	}

	unsupported := false
	nativeWindow.RunNative(func(context any) {
		if _, ok := context.(driver.WaylandWindowContext); ok {
			unsupported = true
		}
	})
	return unsupported
}
