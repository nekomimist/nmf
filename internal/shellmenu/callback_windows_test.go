//go:build windows

package shellmenu

import (
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestShellEnumerationEmptyOrFailed(t *testing.T) {
	emptyFolder := &shellFolder{vtbl: &shellFolderVtbl{
		enumObjects: syscall.NewCallback(func(_ *shellFolder, _, _ uintptr, out **enumIDList) uintptr {
			*out = nil
			return sFalse
		}),
	}}
	if _, err := findChildPIDLByName(emptyFolder, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("empty folder: %v", err)
	}

	enumerator := &enumIDList{vtbl: &enumIDListVtbl{
		next: syscall.NewCallback(func(_ *enumIDList, _ uintptr, _ *uintptr, fetched *uint32) uintptr {
			*fetched = 0
			return 0x80004005 // E_FAIL
		}),
		release: syscall.NewCallback(func(_ *enumIDList) uintptr { return 0 }),
	}}
	folder := &shellFolder{vtbl: &shellFolderVtbl{
		enumObjects: syscall.NewCallback(func(_ *shellFolder, _, _ uintptr, out **enumIDList) uintptr {
			*out = enumerator
			return sOK
		}),
	}}
	if _, err := findChildPIDLByName(folder, "missing"); err == nil || !strings.Contains(err.Error(), "0x80004005") {
		t.Fatalf("failed Next: %v", err)
	}
}

func TestDropSourceCOMCallbacks(t *testing.T) {
	for _, tt := range []struct {
		name      string
		iid       *windows.GUID
		nilOutput bool
		wantHR    uintptr
	}{
		{name: "IUnknown", iid: &iidIUnknown, wantHR: sOK},
		{name: "IDropSource", iid: &iidIDropSource, wantHR: sOK},
		{name: "unsupported interface", iid: &iidIDataObject, wantHR: eNoInterface},
		{name: "nil IID", wantHR: eNoInterface},
		{name: "nil output", iid: &iidIDropSource, nilOutput: true, wantHR: ePointer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := newDropSource()
			defer dropSourceRelease(source)
			out := source // Failure must clear a non-nil output slot.
			// Emulate the stable addresses provided by a native COM caller.
			var pin runtime.Pinner
			pin.Pin(source)
			pin.Pin(&out)
			defer pin.Unpin()
			outPointer := &out
			if tt.nilOutput {
				outPointer = nil
			}
			hr, _, _ := syscall.SyscallN(source.vtbl.queryInterface,
				uintptr(unsafe.Pointer(source)),
				uintptr(unsafe.Pointer(tt.iid)),
				uintptr(unsafe.Pointer(outPointer)))
			if hr != tt.wantHR {
				t.Fatalf("QueryInterface = %#x, want %#x", hr, tt.wantHR)
			}
			if tt.wantHR == sOK {
				if out != source {
					t.Fatal("QueryInterface did not return the same COM object")
				}
				runtime.GC()
				refs, _, _ := syscall.SyscallN(out.vtbl.addRef, uintptr(unsafe.Pointer(out)))
				if refs != 3 {
					t.Fatalf("AddRef = %d, want 3", refs)
				}
				for _, want := range []uintptr{2, 1} {
					refs, _, _ = syscall.SyscallN(out.vtbl.release, uintptr(unsafe.Pointer(out)))
					if refs != want {
						t.Fatalf("Release = %d, want %d", refs, want)
					}
				}
			} else if !tt.nilOutput && out != nil {
				t.Fatal("failed QueryInterface did not clear the output")
			}
			if refs := atomic.LoadUint32(&source.refs); refs != 1 {
				t.Fatalf("remaining references = %d, want 1", refs)
			}
		})
	}
}

type testContextMenu3 struct {
	contextMenu3
	result uintptr
}

var testContextMenu3Vtbl = contextMenu3Vtbl{
	handleMenuMsg2: syscall.NewCallback(func(menu *testContextMenu3, _ uint32, _, _ uintptr, result *uintptr) uintptr {
		growMenuCallbackStack(32)
		*result = menu.result
		return sOK
	}),
}

// Exercise a Go reentry that can move the caller's stack after its out pointer
// has crossed the native ABI as an integer.
//
//go:noinline
func growMenuCallbackStack(depth int) {
	var frame [1024]byte
	frame[0] = byte(depth)
	if depth > 0 {
		growMenuCallbackStack(depth - 1)
	}
	runtime.KeepAlive(&frame)
}

func TestMenuOwnerNativeLifecycle(t *testing.T) {
	// Window creation, synchronous messages, and destruction must share a thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sendMessage := modUser32.NewProc("SendMessageW")
	menuHandle, _, err := procCreatePopupMenu.Call()
	if menuHandle == 0 {
		t.Fatalf("CreatePopupMenu: %v", err)
	}
	defer procDestroyMenu.Call(menuHandle)
	const menuChar = 'a' | 0x10<<16                  // MF_POPUP in the high word.
	wantResults := []uintptr{3<<16 | 17, 3<<16 | 29} // MNC_SELECT and item index.
	var owners []*menuOwner
	for _, result := range wantResults {
		menu := &testContextMenu3{contextMenu3: contextMenu3{vtbl: &testContextMenu3Vtbl}, result: result}
		owner, err := newMenuOwner(0, nil, &menu.contextMenu3)
		if err != nil {
			t.Fatal(err)
		}
		defer owner.destroy()
		owners = append(owners, owner)
	}
	runtime.GC()
	for i, want := range wantResults {
		got, _, _ := sendMessage.Call(owners[i].hwnd, wmMenuChar, menuChar, menuHandle)
		if got != want {
			t.Fatalf("window %d menu result = %d, want %d", i, got, want)
		}
	}

	// Destruction by Windows must remove the entry even before owner cleanup.
	first := owners[0].hwnd
	if ok, _, err := procDestroyWindow.Call(first); ok == 0 {
		t.Fatalf("DestroyWindow: %v", err)
	}
	if getMenuOwnerData(first) != nil {
		t.Fatal("WM_NCDESTROY left menu state registered")
	}
	if got, _, _ := sendMessage.Call(owners[1].hwnd, wmMenuChar, menuChar, menuHandle); got != wantResults[1] {
		t.Fatalf("surviving window menu result = %d, want %d", got, wantResults[1])
	}
	second := owners[1].hwnd
	owners[1].destroy()
	if getMenuOwnerData(second) != nil || owners[1].hwnd != 0 || owners[1].data != nil {
		t.Fatal("owner cleanup left native window or menu state attached")
	}
}
