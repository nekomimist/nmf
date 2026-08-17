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

func TestPlanWindowPlacementMovesParentTowardLeftForRightChild(t *testing.T) {
	parent := windowSwitchRect{Left: 400, Top: 50, Right: 1000, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1500, Bottom: 900}

	plan := planWindowPlacement(parent, 600, 400, work, nil, true)

	if plan.ParentX != 300 || plan.ChildX != 900 || plan.ChildY != 50 || plan.Side != windowPlacementRight || !plan.MoveParent {
		t.Fatalf("plan = %+v; want parent x=300 and right child at 900,50", plan)
	}
}

func TestPlanWindowPlacementMovesParentTowardRightForLeftChild(t *testing.T) {
	parent := windowSwitchRect{Left: 500, Top: 50, Right: 1100, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1500, Bottom: 900}

	plan := planWindowPlacement(parent, 600, 400, work, nil, true)

	if plan.ParentX != 600 || plan.ChildX != 0 || plan.ChildY != 50 || plan.Side != windowPlacementLeft || !plan.MoveParent {
		t.Fatalf("plan = %+v; want parent x=600 and left child at 0,50", plan)
	}
}

func TestPlanWindowPlacementPrefersRightWhenParentMovesAreEqual(t *testing.T) {
	parent := windowSwitchRect{Left: 400, Top: 50, Right: 1000, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1400, Bottom: 900}

	plan := planWindowPlacement(parent, 600, 400, work, nil, true)

	if plan.ParentX != 200 || plan.ChildX != 800 || plan.Side != windowPlacementRight || !plan.MoveParent {
		t.Fatalf("plan = %+v; want equal-move tie to place child on right", plan)
	}
}

func TestPlanWindowPlacementSupportsNegativeWorkAreaWhenMovingParent(t *testing.T) {
	parent := windowSwitchRect{Left: -1100, Top: -100, Right: -500, Bottom: 400}
	work := windowSwitchRect{Left: -1600, Top: -200, Right: -300, Bottom: 800}

	plan := planWindowPlacement(parent, 600, 500, work, nil, true)

	if plan.ParentX != -1000 || plan.ChildX != -1600 || plan.ChildY != -100 || plan.Side != windowPlacementLeft || !plan.MoveParent {
		t.Fatalf("plan = %+v; want parent x=-1000 and left child at -1600,-100", plan)
	}
}

func TestPlanWindowPlacementFallsBackWhenPairDoesNotFit(t *testing.T) {
	parent := windowSwitchRect{Left: 300, Top: 50, Right: 1000, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1200, Bottom: 900}

	plan := planWindowPlacement(parent, 600, 400, work, nil, true)

	if plan.ChildX != 332 || plan.Side != windowPlacementFallback || plan.MoveParent {
		t.Fatalf("plan = %+v; want unchanged-parent fallback at x=332", plan)
	}
}

func TestPlanWindowPlacementDoesNotMoveParentAroundOccupiedWindows(t *testing.T) {
	parent := windowSwitchRect{Left: 500, Top: 50, Right: 1100, Bottom: 450}
	work := windowSwitchRect{Left: 0, Top: 0, Right: 1500, Bottom: 900}
	occupied := []windowSwitchRect{{Left: 0, Top: 50, Right: 400, Bottom: 450}}

	plan := planWindowPlacement(parent, 600, 400, work, occupied, true)

	if plan.ChildX != 532 || plan.Side != windowPlacementFallback || plan.MoveParent {
		t.Fatalf("plan = %+v; want occupied multi-window fallback at x=532", plan)
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
