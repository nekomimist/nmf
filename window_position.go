package main

const windowPlacementNearThreshold int32 = 32

type windowPlacementSide string

const (
	windowPlacementRight    windowPlacementSide = "right"
	windowPlacementLeft     windowPlacementSide = "left"
	windowPlacementFallback windowPlacementSide = "fallback"
)

type windowPlacementPlan struct {
	ParentX    int32
	ChildX     int32
	ChildY     int32
	Side       windowPlacementSide
	MoveParent bool
}

func selectWindowPlacement(parentRect windowSwitchRect, childWidth, childHeight int32, workRect windowSwitchRect, occupied []windowSwitchRect) (int32, int32, windowPlacementSide) {
	plan := planWindowPlacement(parentRect, childWidth, childHeight, workRect, occupied, false)
	return plan.ChildX, plan.ChildY, plan.Side
}

func planWindowPlacement(parentRect windowSwitchRect, childWidth, childHeight int32, workRect windowSwitchRect, occupied []windowSwitchRect, allowParentMove bool) windowPlacementPlan {
	y := clampInt32(parentRect.Top, workRect.Top, workRect.Bottom-childHeight)

	rightX := parentRect.Right
	if rightX+childWidth <= workRect.Right && !windowPlacementOccupied(rightX, y, occupied) {
		return windowPlacementPlan{ParentX: parentRect.Left, ChildX: rightX, ChildY: y, Side: windowPlacementRight}
	}

	leftX := parentRect.Left - childWidth
	if leftX >= workRect.Left && !windowPlacementOccupied(leftX, y, occupied) {
		return windowPlacementPlan{ParentX: parentRect.Left, ChildX: leftX, ChildY: y, Side: windowPlacementLeft}
	}

	parentWidth := parentRect.Right - parentRect.Left
	workWidth := workRect.Right - workRect.Left
	if allowParentMove && len(occupied) == 0 && parentWidth > 0 && childWidth > 0 && parentWidth+childWidth <= workWidth {
		rightParentX := clampInt32(parentRect.Left, workRect.Left, workRect.Right-parentWidth-childWidth)
		leftParentX := clampInt32(parentRect.Left, workRect.Left+childWidth, workRect.Right-parentWidth)
		rightMove := absInt32(rightParentX - parentRect.Left)
		leftMove := absInt32(leftParentX - parentRect.Left)
		if rightMove <= leftMove {
			return windowPlacementPlan{
				ParentX:    rightParentX,
				ChildX:     rightParentX + parentWidth,
				ChildY:     y,
				Side:       windowPlacementRight,
				MoveParent: rightMove != 0,
			}
		}
		return windowPlacementPlan{
			ParentX:    leftParentX,
			ChildX:     leftParentX - childWidth,
			ChildY:     y,
			Side:       windowPlacementLeft,
			MoveParent: leftMove != 0,
		}
	}

	fallbackX := clampInt32(parentRect.Left+windowPlacementNearThreshold, workRect.Left, workRect.Right-childWidth)
	return windowPlacementPlan{ParentX: parentRect.Left, ChildX: fallbackX, ChildY: y, Side: windowPlacementFallback}
}

func selectWindowPositionInWorkRect(requestX, requestY, windowWidth, windowHeight int32, workRect windowSwitchRect) (int32, int32) {
	return clampInt32(requestX, workRect.Left, workRect.Right-windowWidth),
		clampInt32(requestY, workRect.Top, workRect.Bottom-windowHeight)
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
