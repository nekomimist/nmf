package main

import "sync"

type navigationHistoryListener func(string)

type navigationHistoryEventHub struct {
	sync.Mutex
	nextID    int
	listeners map[int]navigationHistoryListener
}

func newNavigationHistoryEventHub() *navigationHistoryEventHub {
	return &navigationHistoryEventHub{listeners: make(map[int]navigationHistoryListener)}
}

func (h *navigationHistoryEventHub) subscribe(listener navigationHistoryListener) func() {
	if listener == nil {
		return func() {}
	}
	if h == nil {
		return func() {}
	}

	h.Lock()
	id := h.nextID
	h.nextID++
	h.listeners[id] = listener
	h.Unlock()

	return func() {
		h.Lock()
		delete(h.listeners, id)
		h.Unlock()
	}
}

func (h *navigationHistoryEventHub) notify(path string) {
	if h == nil {
		return
	}
	h.Lock()
	listeners := make([]navigationHistoryListener, 0, len(h.listeners))
	for _, listener := range h.listeners {
		listeners = append(listeners, listener)
	}
	h.Unlock()

	for _, listener := range listeners {
		listener(path)
	}
}

func (fm *FileManager) subscribeNavigationHistoryChanged(listener navigationHistoryListener) func() {
	if fm == nil || fm.runtime == nil {
		return func() {}
	}
	return fm.runtime.navigationHistoryEvents.subscribe(listener)
}

func (fm *FileManager) notifyNavigationHistoryChanged(path string) {
	if fm == nil || fm.runtime == nil {
		return
	}
	fm.runtime.navigationHistoryEvents.notify(path)
}
