//go:build windows

package fileinfo

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	shell32        = syscall.NewLazyDLL("shell32.dll")
	procShellExecW = shell32.NewProc("ShellExecuteW")
)

// OpenWithDefaultApp opens the given path with the OS-associated application.
// On Windows, this uses ShellExecuteW with the "open" verb. If the path is an
// smb:// display path, it is converted to UNC first.
func OpenWithDefaultApp(p string) error {
	// Convert smb:// to UNC if necessary
	native := NormalizeInputPath(p)

	lpOperation, _ := syscall.UTF16PtrFromString("open")
	lpFile, _ := syscall.UTF16PtrFromString(native)

	// SW_SHOWNORMAL = 1
	ret, _, err := procShellExecW.Call(
		0,
		uintptr(unsafe.Pointer(lpOperation)),
		uintptr(unsafe.Pointer(lpFile)),
		0,
		0,
		1,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed, code=%d err=%v", ret, err)
	}
	return nil
}
