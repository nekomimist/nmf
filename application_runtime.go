package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
	"nmf/internal/secret"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

var errNoInteractiveWindow = errors.New("no open window is available for an interactive prompt")

// ApplicationRuntime owns services shared by every FileManager window.
type ApplicationRuntime struct {
	app                  fyne.App
	watchHub             *watcher.WatchHub
	jobManager           *jobs.Manager
	jobsWindowController *JobsWindowController
	promptBroker         *applicationPromptBroker
	closeOnce            sync.Once
}

func newApplicationRuntime(app fyne.App) *ApplicationRuntime {
	broker := newApplicationPromptBroker()
	runtime := &ApplicationRuntime{
		app:                  app,
		watchHub:             watcher.NewWatchHub(debugPrint),
		jobManager:           jobs.GetManager(),
		jobsWindowController: NewJobsWindowController(app, debugPrint),
		promptBroker:         broker,
	}

	// These package-level hooks bridge VFS code to the one application-scoped
	// cache and broker. New windows register prompt targets with the broker;
	// they never replace the hooks themselves.
	fileinfo.SetCredentialsProvider(fileinfo.NewCachedCredentialsProvider(broker))
	fileinfo.SetArchivePasswordProvider(fileinfo.NewCachedArchivePasswordProvider(broker))
	fileinfo.SetSecretStore(nil)
	if store, err := secret.NewKeyringStore(); err == nil {
		fileinfo.SetSecretStore(store)
	}

	return runtime
}

func (r *ApplicationRuntime) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		if r.jobsWindowController != nil {
			r.jobsWindowController.Close()
		}
	})
}

func (r *ApplicationRuntime) registerWindowPrompts(fm *FileManager) {
	if r == nil || r.promptBroker == nil || fm == nil {
		return
	}
	target := applicationPromptTarget{
		smb:      ui.NewSMBCredentialsProvider(fm.window, fm.keyManager, fm.config.UI.KeyBindings),
		archive:  ui.NewArchivePasswordProvider(fm.window, fm.keyManager, fm.config.UI.KeyBindings),
		conflict: newWindowConflictResolver(fm.window, fm.keyManager, fm.config.UI.KeyBindings),
	}
	fm.promptTargetID, fm.promptUnregister = r.promptBroker.Register(target)
	r.promptBroker.SetActive(fm.promptTargetID, true)
}

func newWindowConflictResolver(window fyne.Window, km *keymanager.KeyManager, bindings []config.KeyBindingEntry) jobs.ConflictResolver {
	return func(ctx context.Context, req jobs.ConflictRequest) jobs.ConflictResolution {
		done := make(chan jobs.ConflictResolution, 1)
		dlg := ui.NewConflictDialog(req, km, bindings)
		fyne.Do(func() {
			dlg.ShowDialog(window, func(res jobs.ConflictResolution) {
				done <- res
			})
		})

		select {
		case <-ctx.Done():
			fyne.Do(dlg.CancelJob)
			<-done
			return jobs.ConflictResolution{Action: jobs.ConflictCancelJob}
		case res := <-done:
			return res
		}
	}
}

type applicationPromptTarget struct {
	smb      fileinfo.CredentialsProvider
	archive  fileinfo.ArchivePasswordProvider
	conflict jobs.ConflictResolver
}

// applicationPromptBroker serializes interactive prompts and selects their
// window at request time. Jobs retain this broker, not a FileManager window.
type applicationPromptBroker struct {
	mu       sync.Mutex
	targets  map[uint64]applicationPromptTarget
	order    []uint64
	nextID   uint64
	activeID uint64
	prompt   chan struct{}
}

func newApplicationPromptBroker() *applicationPromptBroker {
	return &applicationPromptBroker{
		targets: make(map[uint64]applicationPromptTarget),
		prompt:  make(chan struct{}, 1),
	}
}

func (b *applicationPromptBroker) Register(target applicationPromptTarget) (uint64, func()) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.targets[id] = target
	b.order = append(b.order, id)
	b.mu.Unlock()

	var once sync.Once
	return id, func() {
		once.Do(func() {
			b.unregister(id)
		})
	}
}

func (b *applicationPromptBroker) unregister(id uint64) {
	b.mu.Lock()
	delete(b.targets, id)
	if b.activeID == id {
		b.activeID = 0
	}
	for i, candidate := range b.order {
		if candidate == id {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
	b.mu.Unlock()
}

func (b *applicationPromptBroker) SetActive(id uint64, active bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if active {
		if _, ok := b.targets[id]; ok {
			b.activeID = id
		}
		return
	}
	if b.activeID == id {
		b.activeID = 0
	}
}

func (b *applicationPromptBroker) target() (applicationPromptTarget, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if target, ok := b.targets[b.activeID]; ok {
		return target, true
	}
	for i := len(b.order) - 1; i >= 0; i-- {
		if target, ok := b.targets[b.order[i]]; ok {
			return target, true
		}
	}
	return applicationPromptTarget{}, false
}

func (b *applicationPromptBroker) acquire(ctx context.Context) bool {
	select {
	case b.prompt <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (b *applicationPromptBroker) release() {
	<-b.prompt
}

func (b *applicationPromptBroker) Get(host, share, relPath string) (fileinfo.Credentials, error) {
	b.prompt <- struct{}{}
	defer b.release()
	target, ok := b.target()
	if !ok || target.smb == nil {
		return fileinfo.Credentials{}, errNoInteractiveWindow
	}
	return target.smb.Get(host, share, relPath)
}

func (b *applicationPromptBroker) GetArchivePassword(ctx context.Context, req fileinfo.ArchivePasswordRequest) (string, error) {
	if !b.acquire(ctx) {
		return "", ctx.Err()
	}
	defer b.release()
	target, ok := b.target()
	if !ok || target.archive == nil {
		return "", fmt.Errorf("%w: %w", fileinfo.ErrArchivePasswordRequired, errNoInteractiveWindow)
	}
	return target.archive.GetArchivePassword(ctx, req)
}

func (b *applicationPromptBroker) ResolveConflict(ctx context.Context, req jobs.ConflictRequest) jobs.ConflictResolution {
	if !b.acquire(ctx) {
		return jobs.ConflictResolution{Action: jobs.ConflictCancelJob}
	}
	defer b.release()
	target, ok := b.target()
	if !ok || target.conflict == nil {
		return jobs.ConflictResolution{Action: jobs.ConflictCancelJob}
	}
	return target.conflict(ctx, req)
}
