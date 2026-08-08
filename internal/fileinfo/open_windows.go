//go:build windows

package fileinfo

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	shell32        = syscall.NewLazyDLL("shell32.dll")
	procShellExecW = shell32.NewProc("ShellExecuteW")
)

// openNativeWithDefaultApp opens the given native path with the OS-associated application.
// On Windows, this uses ShellExecuteW with the "open" verb. If the path is an
// smb:// display path, it is converted to UNC first.
func openNativeWithDefaultApp(p string) error {
	native := NormalizeInputPath(p)
	workingDirectory := nativeOpenWorkingDirectory(native)

	lpOperation, _ := syscall.UTF16PtrFromString("open")
	lpFile, _ := syscall.UTF16PtrFromString(native)
	lpDirectory, _ := syscall.UTF16PtrFromString(workingDirectory)

	// SW_SHOWNORMAL = 1
	ret, _, err := procShellExecW.Call(
		0,
		uintptr(unsafe.Pointer(lpOperation)),
		uintptr(unsafe.Pointer(lpFile)),
		0,
		uintptr(unsafe.Pointer(lpDirectory)),
		1,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed, code=%d err=%v", ret, err)
	}
	return nil
}

func nativeOpenWorkingDirectory(native string) string {
	return filepath.Dir(native)
}
