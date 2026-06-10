package main

import "testing"

func TestSelectWindowPlacementUsesRightWhenAvailable(t *testing.T) {
	parent := windowSwitchRect{Left: 100, Top: 50, Right: 500, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1200, Bottom: 900}

	x, y, side := selectWindowPlacement(parent, 400, 400, work, nil)

	if x != 500 || y != 50 || side != windowPlacementRight {
		t.Fatalf("placement = %d, %d, %s; want 500, 50, %s", x, y, side, windowPlacementRight)
	}
}

func TestSelectWindowPlacementUsesLeftWhenRightIsOccupied(t *testing.T) {
	parent := windowSwitchRect{Left: 500, Top: 50, Right: 900, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1300, Bottom: 900}
	occupied := []windowSwitchRect{
		{Left: 900, Top: 50, Right: 1300, Bottom: 450},
	}

	x, y, side := selectWindowPlacement(parent, 400, 400, work, occupied)

	if x != 100 || y != 50 || side != windowPlacementLeft {
		t.Fatalf("placement = %d, %d, %s; want 100, 50, %s", x, y, side, windowPlacementLeft)
	}
}

func TestSelectWindowPlacementFallsBackWhenBothSidesAreOccupied(t *testing.T) {
	parent := windowSwitchRect{Left: 500, Top: 50, Right: 900, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1300, Bottom: 900}
	occupied := []windowSwitchRect{
		{Left: 900, Top: 50, Right: 1300, Bottom: 450},
		{Left: 100, Top: 50, Right: 500, Bottom: 450},
	}

	x, y, side := selectWindowPlacement(parent, 400, 400, work, occupied)

	if x != 532 || y != 50 || side != windowPlacementFallback {
		t.Fatalf("placement = %d, %d, %s; want 532, 50, %s", x, y, side, windowPlacementFallback)
	}
}

func TestSelectWindowPlacementFallsBackWhenRightOccupiedAndLeftDoesNotFit(t *testing.T) {
	parent := windowSwitchRect{Left: 100, Top: 50, Right: 500, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 900, Bottom: 900}
	occupied := []windowSwitchRect{
		{Left: 500, Top: 50, Right: 900, Bottom: 450},
	}

	x, y, side := selectWindowPlacement(parent, 400, 400, work, occupied)

	if x != 132 || y != 50 || side != windowPlacementFallback {
		t.Fatalf("placement = %d, %d, %s; want 132, 50, %s", x, y, side, windowPlacementFallback)
	}
}

func TestSelectWindowPlacementUsesLeftWhenRightDoesNotFit(t *testing.T) {
	parent := windowSwitchRect{Left: 500, Top: 50, Right: 900, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1000, Bottom: 900}

	x, y, side := selectWindowPlacement(parent, 400, 400, work, nil)

	if x != 100 || y != 50 || side != windowPlacementLeft {
		t.Fatalf("placement = %d, %d, %s; want 100, 50, %s", x, y, side, windowPlacementLeft)
	}
}

func TestWindowPlacementOccupiedUsesLeftTopNearThreshold(t *testing.T) {
	occupied := []windowSwitchRect{{Left: 100, Top: 100, Right: 500, Bottom: 500}}

	if !windowPlacementOccupied(132, 68, occupied) {
		t.Fatal("position within 32px of occupied left/top should be occupied")
	}
	if windowPlacementOccupied(133, 100, occupied) {
		t.Fatal("position 33px away on x should not be occupied")
	}
	if windowPlacementOccupied(100, 67, occupied) {
		t.Fatal("position 33px away on y should not be occupied")
	}
}

func TestSelectWindowPositionInWorkRectKeepsVisible(t *testing.T) {
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}

	x, y := selectWindowPositionInWorkRect(1800, 1000, 800, 600, work)

	if x != 1120 || y != 440 {
		t.Fatalf("position = %d,%d; want 1120,440", x, y)
	}
}

func TestSelectWindowPositionInWorkRectSupportsNegativeMonitorCoordinates(t *testing.T) {
	work := windowSwitchRect{Left: -1280, Top: -200, Right: 0, Bottom: 800}

	x, y := selectWindowPositionInWorkRect(-1400, -300, 900, 700, work)

	if x != -1280 || y != -200 {
		t.Fatalf("position = %d,%d; want -1280,-200", x, y)
	}
}
