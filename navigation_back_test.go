package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestNavigationBackStackIsLIFO(t *testing.T) {
	fm := &FileManager{}
	normal := directoryNavigation{}

	fm.acceptDirectoryNavigation("/a", "/b", normal)
	fm.acceptDirectoryNavigation("/b", "/c", normal)

	if got, want := fm.navigationBackStack, []string{"/a", "/b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack = %#v, want %#v", got, want)
	}

	target, ok := fm.peekNavigationBack()
	if !ok || target != "/b" {
		t.Fatalf("peek = %q, %t, want /b, true", target, ok)
	}
	fm.acceptDirectoryNavigation("/c", "/b", directoryNavigation{kind: directoryNavigationBack, target: target})

	target, ok = fm.peekNavigationBack()
	if !ok || target != "/a" {
		t.Fatalf("peek after back = %q, %t, want /a, true", target, ok)
	}
}

func TestNavigationBackDoesNotPushDeparturePath(t *testing.T) {
	fm := &FileManager{navigationBackStack: []string{"/a", "/b"}}

	fm.acceptDirectoryNavigation("/c", "/b", directoryNavigation{kind: directoryNavigationBack, target: "/b"})

	if got, want := fm.navigationBackStack, []string{"/a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack = %#v, want %#v", got, want)
	}
}

func TestNavigationBackStackDropsOldestBeyondLimit(t *testing.T) {
	fm := &FileManager{}
	for i := 0; i < navigationBackStackLimit+3; i++ {
		fm.acceptDirectoryNavigation(fmt.Sprintf("/%03d", i), fmt.Sprintf("/%03d", i+1), directoryNavigation{})
	}

	if got := len(fm.navigationBackStack); got != navigationBackStackLimit {
		t.Fatalf("back stack length = %d, want %d", got, navigationBackStackLimit)
	}
	if got, want := fm.navigationBackStack[0], "/003"; got != want {
		t.Fatalf("oldest retained path = %q, want %q", got, want)
	}
	if got, want := fm.navigationBackStack[len(fm.navigationBackStack)-1], fmt.Sprintf("/%03d", navigationBackStackLimit+2); got != want {
		t.Fatalf("newest retained path = %q, want %q", got, want)
	}
}

func TestNavigationBackFailureDropsOnlyMatchingTop(t *testing.T) {
	fm := &FileManager{navigationBackStack: []string{"/a", "/b"}}

	fm.rejectDirectoryNavigation(directoryNavigation{kind: directoryNavigationBack, target: "/other"})
	if got, want := fm.navigationBackStack, []string{"/a", "/b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack after mismatched failure = %#v, want %#v", got, want)
	}

	fm.rejectDirectoryNavigation(directoryNavigation{kind: directoryNavigationBack, target: "/b"})
	if got, want := fm.navigationBackStack, []string{"/a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack after matching failure = %#v, want %#v", got, want)
	}
}

func TestNavigationBackStackIgnoresReload(t *testing.T) {
	fm := &FileManager{}
	fm.acceptDirectoryNavigation("/same", "/same", directoryNavigation{})

	if len(fm.navigationBackStack) != 0 {
		t.Fatalf("reload added back entries: %#v", fm.navigationBackStack)
	}
}
