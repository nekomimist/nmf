package main

const navigationBackStackLimit = 256

type directoryNavigationKind uint8

const (
	directoryNavigationNormal directoryNavigationKind = iota
	directoryNavigationBack
)

type directoryNavigation struct {
	kind   directoryNavigationKind
	target string
}

// HistoryBack returns to the most recent directory successfully left by a
// normal navigation. Back navigation itself never pushes the current path,
// so repeated activation continues toward older directories instead of
// alternating between two paths.
func (fm *FileManager) HistoryBack() {
	target, ok := fm.peekNavigationBack()
	if !ok {
		debugPrint("FileManager: HistoryBack stack empty")
		return
	}

	fm.loadDirectoryWithNavigation(target, true, directoryNavigation{
		kind:   directoryNavigationBack,
		target: target,
	})
}

func (fm *FileManager) peekNavigationBack() (string, bool) {
	if fm == nil || len(fm.navigationBackStack) == 0 {
		return "", false
	}
	return fm.navigationBackStack[len(fm.navigationBackStack)-1], true
}

// acceptDirectoryNavigation updates the per-window back stack only after a
// target listing has become usable. The persisted frecency history remains a
// separate concern and is still updated by recordNavigationHistory.
func (fm *FileManager) acceptDirectoryNavigation(previousPath, openedPath string, navigation directoryNavigation) {
	if fm == nil {
		return
	}

	if navigation.kind == directoryNavigationBack {
		fm.discardNavigationBackTarget(navigation.target)
		return
	}

	previousPath = canonicalNavigationHistoryPath(previousPath)
	openedPath = canonicalNavigationHistoryPath(openedPath)
	if previousPath == "" || openedPath == "" || previousPath == openedPath {
		return
	}

	fm.navigationBackStack = append(fm.navigationBackStack, previousPath)
	if overflow := len(fm.navigationBackStack) - navigationBackStackLimit; overflow > 0 {
		copy(fm.navigationBackStack, fm.navigationBackStack[overflow:])
		newLength := len(fm.navigationBackStack) - overflow
		clear(fm.navigationBackStack[newLength:])
		fm.navigationBackStack = fm.navigationBackStack[:newLength]
	}
	debugPrint("FileManager: HistoryBack push path=%s depth=%d", previousPath, len(fm.navigationBackStack))
}

// rejectDirectoryNavigation drops a back target that could not be opened, so
// one stale or inaccessible path cannot permanently trap repeated Back calls.
// Canceled and superseded loads never reach this helper.
func (fm *FileManager) rejectDirectoryNavigation(navigation directoryNavigation) {
	if fm == nil || navigation.kind != directoryNavigationBack {
		return
	}
	fm.discardNavigationBackTarget(navigation.target)
}

func (fm *FileManager) discardNavigationBackTarget(target string) bool {
	if fm == nil || len(fm.navigationBackStack) == 0 {
		return false
	}

	target = canonicalNavigationHistoryPath(target)
	last := len(fm.navigationBackStack) - 1
	if fm.navigationBackStack[last] != target {
		debugPrint("FileManager: HistoryBack target changed target=%s top=%s", target, fm.navigationBackStack[last])
		return false
	}

	fm.navigationBackStack[last] = ""
	fm.navigationBackStack = fm.navigationBackStack[:last]
	debugPrint("FileManager: HistoryBack pop path=%s depth=%d", target, len(fm.navigationBackStack))
	return true
}
