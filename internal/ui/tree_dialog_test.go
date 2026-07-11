package ui

import (
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"
)

func TestDirectoryTreeChildrenLoadDoesNotBlockDataSource(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	dialog := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
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
}

func TestDirectoryTreeChildrenCacheReturnsCopy(t *testing.T) {
	dialog := NewDirectoryTreeDialog("/tmp", nil, func(string, ...interface{}) {})
	dialog.children["/tmp"] = []string{"/tmp/child"}

	children := dialog.getDirectoryChildren("/tmp")
	children[0] = "changed"

	if got := dialog.children["/tmp"][0]; got != "/tmp/child" {
		t.Fatalf("cached child changed through returned slice: %q", got)
	}
}
