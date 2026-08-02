package ui

import (
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func waitForTreeTest(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for tree state")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDirectoryTreeChildrenLoadDoesNotBlockDataSource(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	dialog := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	dialog.tree = nil
	dialog.loadChildren = func(path string) ([]string, error) {
		if path != "/tmp" {
			return nil, nil
		}
		close(started)
		<-release
		close(finished)
		return []string{"/tmp/child"}, nil
	}

	begin := time.Now()
	children := dialog.getDirectoryChildren("/tmp")
	if elapsed := time.Since(begin); elapsed > 100*time.Millisecond {
		t.Fatalf("tree datasource blocked for %s", elapsed)
	}
	if len(children) != 0 {
		t.Fatalf("uncached children = %#v, want empty while loading", children)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background child loader did not start")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("background child loader did not finish")
	}
	waitForTreeTest(t, func() bool {
		got := dialog.getDirectoryChildren("/tmp")
		return len(got) == 1 && got[0] == "/tmp/child"
	})
}

func TestDirectoryTreeChildrenCacheReturnsCopy(t *testing.T) {
	dialog := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	dialog.tree = nil
	dialog.children["/tmp"] = []string{"/tmp/child"}

	children := dialog.getDirectoryChildren("/tmp")
	children[0] = "changed"

	if got := dialog.children["/tmp"][0]; got != "/tmp/child" {
		t.Fatalf("cached child changed through returned slice: %q", got)
	}
}

func TestDirectoryTreeCachesPlatformBranchClassification(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	dialog := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	dialog.tree = nil
	classified := make(chan string, 1)
	dialog.loadChildren = func(string) ([]string, error) {
		return []string{"X:\\"}, nil
	}
	dialog.classifyBranch = func(path string) (bool, bool) {
		classified <- path
		return false, true
	}

	if children := dialog.getDirectoryChildren("root"); len(children) != 0 {
		t.Fatalf("uncached children = %#v", children)
	}
	select {
	case path := <-classified:
		if path != "X:\\" {
			t.Fatalf("classified path = %q", path)
		}
	case <-time.After(time.Second):
		t.Fatal("platform branch classification did not finish")
	}
	waitForTreeTest(t, func() bool { return !dialog.isDirectory("X:\\") })
}

// TestDirectoryTreeToggleRootModeResetsSelectionForNavigation is a regression
// test for a bug where ToggleRootMode (and the radio-group mode switch) never
// reset dtd.selectedPath/dtd.tree selection to the new root. MoveUp/MoveDown
// locate the cursor by searching getVisibleNodes() (rooted at the new
// currentRoot) for a node matching dtd.selectedPath; a stale path from the
// old root's subtree can never appear there, so both became permanent
// no-ops after a mode toggle.
func TestDirectoryTreeToggleRootModeResetsSelectionForNavigation(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()

	dialog := NewDirectoryTreeDialog("/tmp/project", nil, func(string, ...interface{}) {})

	systemRoot := GetSystemRoot()
	parentRoot := dialog.parentPath // GetPlatformParent("/tmp/project") == "/tmp" on Unix

	// Seed both subtrees directly, following the pattern used by the other
	// tests in this file, so no background loader/goroutine is involved.
	dialog.children[systemRoot] = []string{systemRoot + "root-child"}
	dialog.branches[systemRoot+"root-child"] = true
	dialog.children[parentRoot] = []string{parentRoot + "/parent-child"}
	dialog.branches[parentRoot+"/parent-child"] = true

	// Replicate ShowDialog's selection initialization (see ShowDialog: it
	// expands the initial level, then selects and records the root).
	dialog.expandInitialLevel()
	dialog.tree.Select(widget.TreeNodeID(dialog.currentRoot))
	dialog.selectedPath = dialog.currentRoot

	if dialog.selectedPath != systemRoot {
		t.Fatalf("initial selectedPath = %q, want %q", dialog.selectedPath, systemRoot)
	}

	dialog.ToggleRootMode()

	if dialog.currentRoot != parentRoot {
		t.Fatalf("currentRoot after toggle = %q, want %q", dialog.currentRoot, parentRoot)
	}
	if dialog.selectedPath != parentRoot {
		t.Fatalf("selectedPath after toggle = %q, want %q (reset to new root)", dialog.selectedPath, parentRoot)
	}

	staleSelection := dialog.selectedPath
	dialog.MoveDown()

	if dialog.selectedPath == staleSelection {
		t.Fatalf("MoveDown() did not move selection; selectedPath stayed at %q", dialog.selectedPath)
	}
	wantSelection := parentRoot + "/parent-child"
	if dialog.selectedPath != wantSelection {
		t.Fatalf("selectedPath after MoveDown() = %q, want %q (a node under the new root)", dialog.selectedPath, wantSelection)
	}
}
