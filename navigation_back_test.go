package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNavigationBackStackIsLIFO(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	fm := &FileManager{}
	normal := directoryNavigation{}

	fm.acceptDirectoryNavigation(path("a"), path("b"), normal)
	fm.acceptDirectoryNavigation(path("b"), path("c"), normal)

	if got, want := fm.navigationBackStack, []string{path("a"), path("b")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack = %#v, want %#v", got, want)
	}

	target, ok := fm.peekNavigationBack()
	if !ok || target != path("b") {
		t.Fatalf("peek = %q, %t, want /b, true", target, ok)
	}
	fm.acceptDirectoryNavigation(path("c"), path("b"), directoryNavigation{kind: directoryNavigationBack, target: target})

	target, ok = fm.peekNavigationBack()
	if !ok || target != path("a") {
		t.Fatalf("peek after back = %q, %t, want /a, true", target, ok)
	}
}

func TestNavigationBackDoesNotPushDeparturePath(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	fm := &FileManager{navigationBackStack: []string{path("a"), path("b")}}

	fm.acceptDirectoryNavigation(path("c"), path("b"), directoryNavigation{kind: directoryNavigationBack, target: path("b")})

	if got, want := fm.navigationBackStack, []string{path("a")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack = %#v, want %#v", got, want)
	}
}

func TestNavigationBackStackDropsOldestBeyondLimit(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	fm := &FileManager{}
	for i := 0; i < navigationBackStackLimit+3; i++ {
		fm.acceptDirectoryNavigation(path(fmt.Sprintf("%03d", i)), path(fmt.Sprintf("%03d", i+1)), directoryNavigation{})
	}

	if got := len(fm.navigationBackStack); got != navigationBackStackLimit {
		t.Fatalf("back stack length = %d, want %d", got, navigationBackStackLimit)
	}
	if got, want := fm.navigationBackStack[0], path("003"); got != want {
		t.Fatalf("oldest retained path = %q, want %q", got, want)
	}
	if got, want := fm.navigationBackStack[len(fm.navigationBackStack)-1], path(fmt.Sprintf("%03d", navigationBackStackLimit+2)); got != want {
		t.Fatalf("newest retained path = %q, want %q", got, want)
	}
}

func TestNavigationBackFailureDropsOnlyMatchingTop(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	fm := &FileManager{navigationBackStack: []string{path("a"), path("b")}}

	fm.rejectDirectoryNavigation(directoryNavigation{kind: directoryNavigationBack, target: path("other")})
	if got, want := fm.navigationBackStack, []string{path("a"), path("b")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack after mismatched failure = %#v, want %#v", got, want)
	}

	fm.rejectDirectoryNavigation(directoryNavigation{kind: directoryNavigationBack, target: path("b")})
	if got, want := fm.navigationBackStack, []string{path("a")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("back stack after matching failure = %#v, want %#v", got, want)
	}
}

func TestNavigationBackStackIgnoresReload(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	fm := &FileManager{}
	fm.acceptDirectoryNavigation(path("same"), path("same"), directoryNavigation{})

	if len(fm.navigationBackStack) != 0 {
		t.Fatalf("reload added back entries: %#v", fm.navigationBackStack)
	}
}
