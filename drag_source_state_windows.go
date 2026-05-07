//go:build windows

package main

import (
	"reflect"
	"unsafe"

	"fyne.io/fyne/v2"
)

func resetNativeDragState(window fyne.Window) {
	value := reflect.ValueOf(window)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		debugPrint("FileManager: File drag state reset skipped type=%T", window)
		return
	}
	elem := value.Elem()
	if !elem.IsValid() || elem.Kind() != reflect.Struct {
		debugPrint("FileManager: File drag state reset skipped elem=%s", elem.Kind())
		return
	}

	reset := []string{
		"mouseButton",
		"mouseDragged",
		"mouseDragStarted",
		"mousePressed",
		"mouseCancelFunc",
	}
	count := 0
	for _, name := range reset {
		if resetReflectField(elem, name) {
			count++
		}
	}
	debugPrint("FileManager: File drag state reset fields=%d", count)
}

func resetReflectField(value reflect.Value, name string) bool {
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanAddr() {
		return false
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.Zero(field.Type()))
	return true
}
