package ui

import (
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"
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
