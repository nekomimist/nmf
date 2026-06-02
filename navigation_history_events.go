package main

import "sync"

type navigationHistoryListener func(string)

var navigationHistoryEvents = struct {
	sync.Mutex
	nextID    int
	listeners map[int]navigationHistoryListener
}{
	listeners: make(map[int]navigationHistoryListener),
}

func subscribeNavigationHistoryChanged(listener navigationHistoryListener) func() {
	if listener == nil {
		return func() {}
	}

	navigationHistoryEvents.Lock()
	id := navigationHistoryEvents.nextID
	navigationHistoryEvents.nextID++
	navigationHistoryEvents.listeners[id] = listener
	navigationHistoryEvents.Unlock()

	return func() {
		navigationHistoryEvents.Lock()
		delete(navigationHistoryEvents.listeners, id)
		navigationHistoryEvents.Unlock()
	}
}

func notifyNavigationHistoryChanged(path string) {
	navigationHistoryEvents.Lock()
	listeners := make([]navigationHistoryListener, 0, len(navigationHistoryEvents.listeners))
	for _, listener := range navigationHistoryEvents.listeners {
		listeners = append(listeners, listener)
	}
	navigationHistoryEvents.Unlock()

	for _, listener := range listeners {
		listener(path)
	}
}
