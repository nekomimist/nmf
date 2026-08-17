package main

type windowZOrderEntry struct {
	hwnd      uintptr
	visible   bool
	iconified bool
}

func selectFileManagerWindowZOrder(eligible map[uintptr]struct{}, zOrder []windowZOrderEntry) ([]uintptr, bool) {
	remaining := make(map[uintptr]struct{}, len(eligible))
	for hwnd := range eligible {
		remaining[hwnd] = struct{}{}
	}

	order := make([]uintptr, 0, len(remaining))
	alreadyFront := true
	encounteredOtherVisible := false
	for _, entry := range zOrder {
		if _, ok := remaining[entry.hwnd]; ok {
			order = append(order, entry.hwnd)
			delete(remaining, entry.hwnd)
			if encounteredOtherVisible {
				alreadyFront = false
			}
			continue
		}
		if entry.visible && !entry.iconified {
			encounteredOtherVisible = true
		}
	}
	if len(remaining) > 0 {
		alreadyFront = false
	}
	return order, alreadyFront
}
