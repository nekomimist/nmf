//go:build !windows

package main

func installWindowActivationHandler(*ApplicationRuntime) {
}

func bringFileManagerWindowsToFront(*ApplicationRuntime) {
}
