package main

import (
	"reflect"
	"testing"
)

func TestSelectFileManagerWindowZOrderAlreadyFront(t *testing.T) {
	eligible := map[uintptr]struct{}{2: {}, 3: {}}
	zOrder := []windowZOrderEntry{
		{hwnd: 2, visible: true},
		{hwnd: 10, visible: false},
		{hwnd: 3, visible: true},
		{hwnd: 20, visible: true},
	}

	order, alreadyFront := selectFileManagerWindowZOrder(eligible, zOrder)

	if !reflect.DeepEqual(order, []uintptr{2, 3}) || !alreadyFront {
		t.Fatalf("order = %v, alreadyFront = %t; want [2 3], true", order, alreadyFront)
	}
}

func TestSelectFileManagerWindowZOrderRaisesPastOtherVisibleWindows(t *testing.T) {
	eligible := map[uintptr]struct{}{2: {}, 3: {}}
	zOrder := []windowZOrderEntry{
		{hwnd: 20, visible: true},
		{hwnd: 3, visible: true},
		{hwnd: 30, visible: true, iconified: true},
		{hwnd: 2, visible: true},
	}

	order, alreadyFront := selectFileManagerWindowZOrder(eligible, zOrder)

	if !reflect.DeepEqual(order, []uintptr{3, 2}) || alreadyFront {
		t.Fatalf("order = %v, alreadyFront = %t; want [3 2], false", order, alreadyFront)
	}
}

func TestSelectFileManagerWindowZOrderReportsMissingWindows(t *testing.T) {
	eligible := map[uintptr]struct{}{2: {}, 3: {}}
	zOrder := []windowZOrderEntry{{hwnd: 2, visible: true}}

	order, alreadyFront := selectFileManagerWindowZOrder(eligible, zOrder)

	if !reflect.DeepEqual(order, []uintptr{2}) || alreadyFront {
		t.Fatalf("order = %v, alreadyFront = %t; want [2], false", order, alreadyFront)
	}
}
