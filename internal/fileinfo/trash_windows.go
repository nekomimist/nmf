//go:build windows

package fileinfo

import (
	"context"
	"fmt"
	"unicode/utf16"
	"unsafe"
)

const (
	foDelete          = 0x0003
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004
)

var (
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

func trashPath(ctx context.Context, displayPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	native := NormalizeInputPath(displayPath)
	from := doubleNullUTF16(native)
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent,
	}
	ret, _, err := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed for %s: code=%d err=%v", displayPath, ret, err)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("delete to recycle bin was aborted for %s", displayPath)
	}
	return nil
}

func doubleNullUTF16(s string) []uint16 {
	encoded := utf16.Encode([]rune(s))
	return append(encoded, 0, 0)
}
