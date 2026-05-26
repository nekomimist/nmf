package main

const windowPlacementNearThreshold int32 = 32

type windowPlacementSide string

const (
	windowPlacementRight    windowPlacementSide = "right"
	windowPlacementLeft     windowPlacementSide = "left"
	windowPlacementFallback windowPlacementSide = "fallback"
)

func selectWindowPlacement(parentRect windowSwitchRect, childWidth, childHeight int32, workRect windowSwitchRect, occupied []windowSwitchRect) (int32, int32, windowPlacementSide) {
	y := clampInt32(parentRect.Top, workRect.Top, workRect.Bottom-childHeight)

	rightX := parentRect.Right
	if rightX+childWidth <= workRect.Right && !windowPlacementOccupied(rightX, y, occupied) {
		return rightX, y, windowPlacementRight
	}

	leftX := parentRect.Left - childWidth
	if leftX >= workRect.Left && !windowPlacementOccupied(leftX, y, occupied) {
		return leftX, y, windowPlacementLeft
	}

	fallbackX := clampInt32(parentRect.Left+windowPlacementNearThreshold, workRect.Left, workRect.Right-childWidth)
	return fallbackX, y, windowPlacementFallback
}

func windowPlacementOccupied(x, y int32, occupied []windowSwitchRect) bool {
	for _, rect := range occupied {
		if absInt32(x-rect.Left) <= windowPlacementNearThreshold && absInt32(y-rect.Top) <= windowPlacementNearThreshold {
			return true
		}
	}
	return false
}

func absInt32(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

func clampInt32(value, min, max int32) int32 {
	if max < min {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
