package ui

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
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

func TestTreeParentPreservesArchiveRoot(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "content.zip")
	root := fileinfo.ArchiveRootPath(archive)
	if got := GetPlatformParent(fileinfo.JoinPath(root, "folder")); got != root {
		t.Fatalf("parent = %q, want %q", got, root)
	}
	if got := GetPlatformParent(root); got != filepath.Dir(archive) {
		t.Fatalf("parent of archive = %q, want %q", got, filepath.Dir(archive))
	}
}

func TestTreeNavigationReflectsLoadedChildren(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	d := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	// Exercise the async loader and navigation index without a canvas renderer.
	root := d.currentRoot
	d.tree = &widget.Tree{Root: root}
	d.selectedPath = root
	release := make(chan struct{})
	d.loadChildren = func(string) ([]string, error) { <-release; return []string{"child"}, nil }
	d.getDirectoryChildren(root)
	if _, ok := d.adjacentVisibleNode(1); ok {
		t.Fatal("pending load unexpectedly supplied a child")
	}
	close(release)
	waitForTreeTest(t, func() bool { node, ok := d.adjacentVisibleNode(1); return ok && node == "child" })
	d.loadUIApplyMu.Lock()
	d.loadUIApplyMu.Unlock()
}

func TestTreeNavigationReflectsBranchChanges(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	d := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	d.children[d.currentRoot] = []string{"a", "b"}
	d.children["a"] = []string{"a1"}
	d.selectedPath = "a"
	if n, _ := d.adjacentVisibleNode(1); n != "b" {
		t.Fatalf("closed branch next = %q", n)
	}
	d.tree.OpenBranch("a")
	if n, _ := d.adjacentVisibleNode(1); n != "a1" {
		t.Fatalf("expanded branch next = %q", n)
	}
	d.tree.CloseBranch("a")
	if n, _ := d.adjacentVisibleNode(1); n != "b" {
		t.Fatalf("collapsed branch next = %q", n)
	}
}

func TestDirectoryTreeHidesSeparators(t *testing.T) {
	d := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	if !d.tree.HideSeparators {
		t.Fatal("tree widget should hide per-row separators for compact display")
	}
}

func BenchmarkDirectoryTreeAdjacentVisibleNode(b *testing.B) {
	d := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	children := make([]string, 10000)
	for i := range children {
		children[i] = "/folder" + strconv.Itoa(i)
	}
	d.children[d.currentRoot] = children
	d.selectedPath = d.currentRoot
	d.adjacentVisibleNode(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delta := 1
		if i%2 != 0 {
			delta = -1
		}
		d.selectedPath, _ = d.adjacentVisibleNode(delta)
	}
}
