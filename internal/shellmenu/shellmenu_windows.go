//go:build windows

package shellmenu

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrUnsupported indicates that the shell context menu is unavailable.
var ErrUnsupported = errors.New("shell context menu is unsupported on this platform")

// Debugf receives optional debug messages from this package.
var Debugf func(format string, args ...interface{})

func dbg(format string, args ...interface{}) {
	if Debugf != nil {
		Debugf("shellmenu: "+format, args...)
	}
}

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	sFalse                  = 0x1
	rpcEChangedMode         = 0x80010106
	sOK                     = 0x0

	cmfNormal = 0x0

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	mfByPosition = 0x00000400

	miimState    = 0x00000001
	miimID       = 0x00000002
	miimSubMenu  = 0x00000004
	miimString   = 0x00000040
	miimFType    = 0x00000100
	mftSeparator = 0x00000800
	mfsDisabled  = 0x00000003

	gcsVerbW = 0x00000004

	dropEffectCopy             = 0x00000001
	dragdropSCancel            = 0x00040101
	dragdropSDrop              = 0x00040100
	dragdropSUseDefaultCursors = 0x00040102
	mouseKeyStateLeftButton    = 0x00000001
	eNoInterface               = 0x80004002
	ePointer                   = 0x80004003
	swShowNormal               = 1

	wmInitMenuPopup = 0x0117
	wmDrawItem      = 0x002B
	wmMeasureItem   = 0x002C
	wmMenuChar      = 0x0120
	wmNCDestroy     = 0x0082

	cwUseDefault = 0x80000000

	csidlNetwork = 0x0012

	shcontfFolders       = 0x0020
	shcontfNonFolders    = 0x0040
	shcontfIncludeHidden = 0x0080

	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004

	fofxRecycleOnDelete = 0x00080000
)

const gwlUserData = ^uintptr(20)

var (
	iidIUnknown        = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDataObject     = windows.GUID{Data1: 0x0000010E, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDropSource     = windows.GUID{Data1: 0x00000121, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellFolder    = windows.GUID{Data1: 0x000214E6, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu    = windows.GUID{Data1: 0x000214E4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu2   = windows.GUID{Data1: 0x000214F4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu3   = windows.GUID{Data1: 0xBCFCE0A0, Data2: 0xEC17, Data3: 0x11D0, Data4: [8]byte{0x8D, 0x10, 0x00, 0xA0, 0xC9, 0x0F, 0x27, 0x19}}
	iidIFileOperation  = windows.GUID{Data1: 0x947AAB5F, Data2: 0x0A5C, Data3: 0x4C13, Data4: [8]byte{0xB4, 0xD6, 0x4B, 0xF7, 0x83, 0x6F, 0xC9, 0xF8}}
	clsidFileOperation = windows.GUID{Data1: 0x3AD05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xB8, 0x5B, 0xDB, 0x8E, 0x09}}

	modShell32 = windows.NewLazySystemDLL("shell32.dll")
	modUser32  = windows.NewLazySystemDLL("user32.dll")
	modOle32   = windows.NewLazySystemDLL("ole32.dll")
	modShlwapi = windows.NewLazySystemDLL("shlwapi.dll")

	procSHParseDisplayName         = modShell32.NewProc("SHParseDisplayName")
	procSHBindToParent             = modShell32.NewProc("SHBindToParent")
	procSHGetSpecialFolderLocation = modShell32.NewProc("SHGetSpecialFolderLocation")
	procSHCreateShellItem          = modShell32.NewProc("SHCreateShellItem")
	procStrRetToBufW               = modShlwapi.NewProc("StrRetToBufW")

	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = modOle32.NewProc("CoTaskMemFree")
	procOleInitialize    = modOle32.NewProc("OleInitialize")
	procOleUninitialize  = modOle32.NewProc("OleUninitialize")
	procDoDragDrop       = modOle32.NewProc("DoDragDrop")

	procCreatePopupMenu   = modUser32.NewProc("CreatePopupMenu")
	procDestroyMenu       = modUser32.NewProc("DestroyMenu")
	procTrackPopupMenuEx  = modUser32.NewProc("TrackPopupMenuEx")
	procGetMenuItemCount  = modUser32.NewProc("GetMenuItemCount")
	procGetMenuItemInfoW  = modUser32.NewProc("GetMenuItemInfoW")
	procDeleteMenu        = modUser32.NewProc("DeleteMenu")
	procGetCursorPos      = modUser32.NewProc("GetCursorPos")
	procClientToScreen    = modUser32.NewProc("ClientToScreen")
	procSetForegroundWnd  = modUser32.NewProc("SetForegroundWindow")
	procRegisterClassExW  = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW   = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow     = modUser32.NewProc("DestroyWindow")
	procDefWindowProcW    = modUser32.NewProc("DefWindowProcW")
	procSetWindowLongPtrW = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW = modUser32.NewProc("GetWindowLongPtrW")

	menuOwnerWndProcPtr = syscall.NewCallback(menuOwnerWndProc)

	dropSourceVtblInst = dropSourceVtbl{
		queryInterface:    syscall.NewCallback(dropSourceQueryInterface),
		addRef:            syscall.NewCallback(dropSourceAddRef),
		release:           syscall.NewCallback(dropSourceRelease),
		queryContinueDrag: syscall.NewCallback(dropSourceQueryContinueDrag),
		giveFeedback:      syscall.NewCallback(dropSourceGiveFeedback),
	}
)

type point struct {
	x int32
	y int32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type unknownVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
}

type unknown struct {
	vtbl *unknownVtbl
}

type dataObject struct {
	vtbl *unknownVtbl
}

type dropSourceVtbl struct {
	queryInterface    uintptr
	addRef            uintptr
	release           uintptr
	queryContinueDrag uintptr
	giveFeedback      uintptr
}

type dropSource struct {
	vtbl *dropSourceVtbl
	refs uint32
}

type shellFolderVtbl struct {
	queryInterface   uintptr
	addRef           uintptr
	release          uintptr
	parseDisplayName uintptr
	enumObjects      uintptr
	bindToObject     uintptr
	bindToStorage    uintptr
	compareIDs       uintptr
	createViewObject uintptr
	getAttributesOf  uintptr
	getUIObjectOf    uintptr
	getDisplayNameOf uintptr
	setNameOf        uintptr
}

type shellFolder struct {
	vtbl *shellFolderVtbl
}

type shellItem struct {
	vtbl *unknownVtbl
}

type fileOperationVtbl struct {
	queryInterface          uintptr
	addRef                  uintptr
	release                 uintptr
	advise                  uintptr
	unadvise                uintptr
	setOperationFlags       uintptr
	setProgressMessage      uintptr
	setProgressDialog       uintptr
	setProperties           uintptr
	setOwnerWindow          uintptr
	applyPropertiesToItem   uintptr
	applyPropertiesToItems  uintptr
	renameItem              uintptr
	renameItems             uintptr
	moveItem                uintptr
	moveItems               uintptr
	copyItem                uintptr
	copyItems               uintptr
	deleteItem              uintptr
	deleteItems             uintptr
	newItem                 uintptr
	performOperations       uintptr
	getAnyOperationsAborted uintptr
}

type fileOperation struct {
	vtbl *fileOperationVtbl
}

type enumIDListVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	next           uintptr
	skip           uintptr
	reset          uintptr
	clone          uintptr
}

type enumIDList struct {
	vtbl *enumIDListVtbl
}

// strret has the same layout as STRRET. The union is MAX_PATH bytes on both
// 32-bit and 64-bit Windows; the explicit padding aligns its pointer variant.
type strret struct {
	typ  uint32
	_pad uint32
	data [260]byte
}

type contextMenuVtbl struct {
	queryInterface   uintptr
	addRef           uintptr
	release          uintptr
	queryContextMenu uintptr
	invokeCommand    uintptr
	getCommandString uintptr
}

type contextMenu struct {
	vtbl *contextMenuVtbl
}

type contextMenu2Vtbl struct {
	queryInterface   uintptr
	addRef           uintptr
	release          uintptr
	queryContextMenu uintptr
	invokeCommand    uintptr
	getCommandString uintptr
	handleMenuMsg    uintptr
}

type contextMenu2 struct {
	vtbl *contextMenu2Vtbl
}

type contextMenu3Vtbl struct {
	queryInterface   uintptr
	addRef           uintptr
	release          uintptr
	queryContextMenu uintptr
	invokeCommand    uintptr
	getCommandString uintptr
	handleMenuMsg    uintptr
	handleMenuMsg2   uintptr
}

type contextMenu3 struct {
	vtbl *contextMenu3Vtbl
}

type cmInvokeCommandInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       uintptr
	lpParameters uintptr
	lpDirectory  uintptr
	nShow        int32
	dwHotKey     uint32
	hIcon        uintptr
}

type menuItemInfo struct {
	cbSize        uint32
	fMask         uint32
	fType         uint32
	fState        uint32
	wID           uint32
	hSubMenu      uintptr
	hbmpChecked   uintptr
	hbmpUnchecked uintptr
	dwItemData    uintptr
	dwTypeData    *uint16
	cch           uint32
	hbmpItem      uintptr
}

// Show opens the Explorer shell context menu for paths at the current mouse position.
func Show(hwnd uintptr, paths []string) error {
	var pt point
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return logErr(fmt.Errorf("GetCursorPos failed: %w", err))
	}
	return showAtScreenPosition(hwnd, paths, pt)
}

// ShowAtClientPosition opens the Explorer shell context menu at a window client coordinate.
func ShowAtClientPosition(hwnd uintptr, paths []string, x, y int) error {
	if hwnd == 0 {
		return logErr(ErrUnsupported)
	}
	pt := point{x: int32(x), y: int32(y)}
	ret, _, err := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return logErr(fmt.Errorf("ClientToScreen failed: %w", err))
	}
	return showAtScreenPosition(hwnd, paths, pt)
}

// StartFileDrag starts a Windows Shell drag operation for the provided paths.
// Only copy effects are advertised; the source files are never removed by nmf.
func StartFileDrag(hwnd uintptr, paths []string) error {
	if hwnd == 0 {
		return ErrUnsupported
	}
	nativePaths := normalizePaths(paths)
	dbg("StartFileDrag hwnd=%d sources=%d", hwnd, len(nativePaths))
	if len(nativePaths) == 0 {
		return nil
	}
	for i, p := range nativePaths {
		dbg("StartFileDrag source[%d]=%s", i, p)
	}
	if err := ensureSameParent(nativePaths); err != nil {
		dbg("StartFileDrag rejected: %v", err)
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oleInited, err := initializeOLE()
	if err != nil {
		dbg("StartFileDrag OleInitialize error=%v", err)
		return err
	}
	dbg("StartFileDrag ole_initialized=%t", oleInited)
	if oleInited {
		defer procOleUninitialize.Call()
	}

	folder, childPIDLs, err := shellFolderAndChildren(nativePaths)
	if err != nil {
		dbg("StartFileDrag shell folder error=%v", err)
		return err
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(folder)))
	for _, pidl := range childPIDLs {
		defer procCoTaskMemFree.Call(pidl)
	}

	data, err := shellDataObject(hwnd, folder, childPIDLs)
	if err != nil {
		dbg("StartFileDrag data object error=%v", err)
		return err
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(data)))

	source := newDropSource()
	defer dropSourceRelease(uintptr(unsafe.Pointer(source)))
	var effect uint32
	hr, _, _ := procDoDragDrop.Call(
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(source)),
		dropEffectCopy,
		uintptr(unsafe.Pointer(&effect)),
	)
	dbg("StartFileDrag DoDragDrop hr=0x%x effect=0x%x", uint32(hr), effect)
	if failed(hr) && uint32(hr) != dragdropSCancel {
		return fmt.Errorf("DoDragDrop failed: 0x%x", uint32(hr))
	}
	return nil
}

// Trash moves a file system item to the Windows Recycle Bin. It resolves the
// item from its parent folder and relative PIDL, avoiding legacy full-path
// parsing for long local and UNC paths.
func Trash(path string) error {
	nativePaths := normalizePaths([]string{path})
	if len(nativePaths) == 0 {
		return fmt.Errorf("Trash requires a path")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	coinited, err := initializeCOM()
	if err != nil {
		return err
	}
	if coinited {
		defer procCoUninitialize.Call()
	}

	folder, childPIDLs, err := shellFolderAndChildren(nativePaths)
	if err != nil {
		return err
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(folder)))
	defer freePIDLs(childPIDLs)

	item, err := shellItemFromFolderAndPIDL(folder, childPIDLs[0])
	if err != nil {
		return err
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(item)))

	op, err := newFileOperation()
	if err != nil {
		return err
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(op)))

	if err := setFileOperationFlags(op, fofAllowUndo|fofNoConfirmation|fofSilent|fofxRecycleOnDelete); err != nil {
		return err
	}
	if err := queueRecycleDelete(op, item); err != nil {
		return err
	}
	if err := performFileOperations(op); err != nil {
		return err
	}
	if err := checkFileOperationAborted(op); err != nil {
		return err
	}
	dbg("Trash completed with relative PIDL")
	return nil
}

func showAtScreenPosition(hwnd uintptr, paths []string, pt point) error {
	if hwnd == 0 {
		return logErr(ErrUnsupported)
	}
	nativePaths := normalizePaths(paths)
	if len(nativePaths) == 0 {
		dbg("Show skipped: no native paths")
		return nil
	}
	if err := ensureSameParent(nativePaths); err != nil {
		return logErr(err)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	coinited, err := initializeCOM()
	if err != nil {
		return logErr(err)
	}
	if coinited {
		defer procCoUninitialize.Call()
	}

	menu, _, err := procCreatePopupMenu.Call()
	if menu == 0 {
		return logErr(fmt.Errorf("CreatePopupMenu failed: %w", err))
	}
	defer procDestroyMenu.Call(menu)

	folder, childPIDLs, err := shellFolderAndChildren(nativePaths)
	if err != nil {
		return logErr(err)
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(folder)))
	for _, pidl := range childPIDLs {
		defer procCoTaskMemFree.Call(pidl)
	}

	var menuPtr *contextMenu
	hr, _, _ := syscall.SyscallN(
		folder.vtbl.getUIObjectOf,
		uintptr(unsafe.Pointer(folder)),
		hwnd,
		uintptr(len(childPIDLs)),
		uintptr(unsafe.Pointer(&childPIDLs[0])),
		uintptr(unsafe.Pointer(&iidIContextMenu)),
		0,
		uintptr(unsafe.Pointer(&menuPtr)),
	)
	if failed(hr) {
		return logErr(fmt.Errorf("IShellFolder.GetUIObjectOf(IContextMenu) failed: 0x%x", uint32(hr)))
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(menuPtr)))

	var menu2 *contextMenu2
	if queryInterface((*unknown)(unsafe.Pointer(menuPtr)), &iidIContextMenu2, unsafe.Pointer(&menu2)) != nil {
		menu2 = nil
	} else {
		defer releaseUnknown((*unknown)(unsafe.Pointer(menu2)))
	}
	var menu3 *contextMenu3
	if queryInterface((*unknown)(unsafe.Pointer(menuPtr)), &iidIContextMenu3, unsafe.Pointer(&menu3)) != nil {
		menu3 = nil
	} else {
		defer releaseUnknown((*unknown)(unsafe.Pointer(menu3)))
	}

	const firstID = 1
	const lastID = 0x7fff
	hr, _, _ = syscall.SyscallN(
		menuPtr.vtbl.queryContextMenu,
		uintptr(unsafe.Pointer(menuPtr)),
		menu,
		0,
		firstID,
		lastID,
		cmfNormal,
	)
	if failed(hr) {
		return logErr(fmt.Errorf("IContextMenu.QueryContextMenu failed: 0x%x", uint32(hr)))
	}
	pruneDuplicateMenuItems(menuPtr, menu, firstID)

	owner, err := newMenuOwner(hwnd, menu2, menu3)
	if err != nil {
		return logErr(err)
	}
	defer owner.destroy()

	procSetForegroundWnd.Call(hwnd)
	cmd, _, _ := procTrackPopupMenuEx.Call(
		menu,
		tpmReturnCmd|tpmRightButton,
		uintptr(pt.x),
		uintptr(pt.y),
		owner.hwnd,
		0,
	)
	if cmd == 0 {
		dbg("Show canceled/no command selected")
		return nil
	}

	invoke := cmInvokeCommandInfo{
		cbSize: uint32(unsafe.Sizeof(cmInvokeCommandInfo{})),
		hwnd:   hwnd,
		lpVerb: cmd - firstID,
		nShow:  swShowNormal,
	}
	hr, _, _ = syscall.SyscallN(
		menuPtr.vtbl.invokeCommand,
		uintptr(unsafe.Pointer(menuPtr)),
		uintptr(unsafe.Pointer(&invoke)),
	)
	if failed(hr) {
		return logErr(fmt.Errorf("IContextMenu.InvokeCommand failed: 0x%x", uint32(hr)))
	}
	return nil
}

func logErr(err error) error {
	if err != nil {
		dbg("Show error=%v", err)
	}
	return err
}

func pruneDuplicateMenuItems(menuPtr *contextMenu, menu uintptr, firstID uintptr) {
	entries := readMenuDedupeEntries(menuPtr, menu, firstID)
	if len(entries) == 0 {
		return
	}
	positions := duplicateMenuPositions(entries)
	if len(positions) == 0 {
		return
	}

	removed := make(map[int]struct{}, len(positions))
	for _, pos := range positions {
		removed[pos] = struct{}{}
	}
	positions = append(positions, separatorCleanupPositions(entries, removed)...)
	sortDescending(positions)

	for _, pos := range positions {
		procDeleteMenu.Call(menu, uintptr(pos), mfByPosition)
	}
	dbg("Show pruned duplicate menu items count=%d", len(positions))
}

func readMenuDedupeEntries(menuPtr *contextMenu, menu uintptr, firstID uintptr) []menuDedupeEntry {
	count, _, _ := procGetMenuItemCount.Call(menu)
	if int32(count) <= 0 {
		return nil
	}

	entries := make([]menuDedupeEntry, 0, int(count))
	for pos := 0; pos < int(count); pos++ {
		info, ok := getMenuItemInfo(menu, pos)
		if !ok {
			continue
		}
		entry := menuDedupeEntry{
			position:  pos,
			commandID: uintptr(info.wID),
			label:     menuItemLabel(info),
			separator: info.fType&mftSeparator != 0,
			enabled:   info.fState&mfsDisabled == 0,
		}
		if !entry.separator && entry.commandID >= firstID && info.hSubMenu == 0 {
			entry.verb = contextMenuVerb(menuPtr, entry.commandID-firstID)
		}
		entries = append(entries, entry)
	}
	return entries
}

func getMenuItemInfo(menu uintptr, pos int) (menuItemInfo, bool) {
	const labelChars = 512
	label := make([]uint16, labelChars)
	info := menuItemInfo{
		cbSize:     uint32(unsafe.Sizeof(menuItemInfo{})),
		fMask:      miimFType | miimState | miimID | miimSubMenu | miimString,
		dwTypeData: &label[0],
		cch:        uint32(len(label)),
	}
	ret, _, _ := procGetMenuItemInfoW.Call(
		menu,
		uintptr(pos),
		1,
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return menuItemInfo{}, false
	}
	return info, true
}

func menuItemLabel(info menuItemInfo) string {
	if info.dwTypeData == nil || info.cch == 0 {
		return ""
	}
	return windows.UTF16PtrToString(info.dwTypeData)
}

func contextMenuVerb(menuPtr *contextMenu, offset uintptr) string {
	const verbChars = 256
	buf := make([]uint16, verbChars)
	hr, _, _ := syscall.SyscallN(
		menuPtr.vtbl.getCommandString,
		uintptr(unsafe.Pointer(menuPtr)),
		offset,
		gcsVerbW,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if failed(hr) {
		return ""
	}
	return windows.UTF16PtrToString(&buf[0])
}

func shellDataObject(hwnd uintptr, folder *shellFolder, childPIDLs []uintptr) (*dataObject, error) {
	if folder == nil || len(childPIDLs) == 0 {
		return nil, ErrUnsupported
	}
	var data *dataObject
	hr, _, _ := syscall.SyscallN(
		folder.vtbl.getUIObjectOf,
		uintptr(unsafe.Pointer(folder)),
		hwnd,
		uintptr(len(childPIDLs)),
		uintptr(unsafe.Pointer(&childPIDLs[0])),
		uintptr(unsafe.Pointer(&iidIDataObject)),
		0,
		uintptr(unsafe.Pointer(&data)),
	)
	if failed(hr) {
		return nil, fmt.Errorf("IShellFolder.GetUIObjectOf(IDataObject) failed: 0x%x", uint32(hr))
	}
	return data, nil
}

func normalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, normalizeWindowsShellPath(p))
	}
	return out
}

func ensureSameParent(paths []string) error {
	if len(paths) < 2 {
		return nil
	}
	parent := strings.ToLower(filepath.Clean(filepath.Dir(paths[0])))
	for _, p := range paths[1:] {
		if strings.ToLower(filepath.Clean(filepath.Dir(p))) != parent {
			return fmt.Errorf("Explorer context menu requires files in the same folder")
		}
	}
	return nil
}

func initializeCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	switch uint32(hr) {
	case 0, sFalse:
		return true, nil
	case rpcEChangedMode:
		return false, nil
	default:
		if failed(hr) {
			return false, fmt.Errorf("CoInitializeEx failed: 0x%x", uint32(hr))
		}
		return true, nil
	}
}

func initializeOLE() (bool, error) {
	hr, _, _ := procOleInitialize.Call(0)
	switch uint32(hr) {
	case 0, sFalse:
		return true, nil
	case rpcEChangedMode:
		return false, nil
	default:
		if failed(hr) {
			return false, fmt.Errorf("OleInitialize failed: 0x%x", uint32(hr))
		}
		return true, nil
	}
}

func shellFolderAndChildren(paths []string) (*shellFolder, []uintptr, error) {
	if len(paths) == 0 {
		return nil, nil, ErrUnsupported
	}

	firstPath, err := parseWindowsShellPath(paths[0])
	if err != nil {
		return nil, nil, err
	}
	parentPath, firstName, err := firstPath.parentAndName()
	if err != nil {
		return nil, nil, err
	}

	folder, err := shellFolderForPath(parentPath)
	if err != nil {
		return nil, nil, err
	}
	dbg("Resolved Shell folder with relative PIDLs root=%s parentComponents=%d items=%d", parentPath.root, len(parentPath.components), len(paths))
	childPIDLs := make([]uintptr, 0, len(paths))

	for i, p := range paths {
		path := firstPath
		name := firstName
		if i > 0 {
			path, err = parseWindowsShellPath(p)
			if err != nil {
				freePIDLs(childPIDLs)
				releaseUnknown((*unknown)(unsafe.Pointer(folder)))
				return nil, nil, err
			}
			var parent windowsShellPath
			parent, name, err = path.parentAndName()
			if err != nil {
				freePIDLs(childPIDLs)
				releaseUnknown((*unknown)(unsafe.Pointer(folder)))
				return nil, nil, err
			}
			if !parentPath.sameFolder(parent) {
				freePIDLs(childPIDLs)
				releaseUnknown((*unknown)(unsafe.Pointer(folder)))
				return nil, nil, fmt.Errorf("Explorer context menu requires files in the same folder")
			}
		}

		pidl, err := childPIDL(folder, name)
		if err != nil {
			freePIDLs(childPIDLs)
			releaseUnknown((*unknown)(unsafe.Pointer(folder)))
			return nil, nil, err
		}
		childPIDLs = append(childPIDLs, pidl)
	}

	return folder, childPIDLs, nil
}

func shellFolderForPath(path windowsShellPath) (*shellFolder, error) {
	folder, err := shellFolderForRoot(path)
	if err != nil {
		return nil, err
	}
	for _, component := range path.components {
		pidl, err := childPIDL(folder, component)
		if err != nil {
			releaseUnknown((*unknown)(unsafe.Pointer(folder)))
			return nil, err
		}
		next, err := bindChildFolder(folder, pidl)
		procCoTaskMemFree.Call(pidl)
		releaseUnknown((*unknown)(unsafe.Pointer(folder)))
		if err != nil {
			return nil, err
		}
		folder = next
	}
	return folder, nil
}

func shellFolderForRoot(path windowsShellPath) (*shellFolder, error) {
	folder, err := shellFolderFromAbsolutePath(path.root)
	if err == nil || !path.unc {
		return folder, err
	}

	// Some UNC roots cannot be parsed as one display name. Start from the
	// Network shell folder and resolve server/share one component at a time.
	dbg("Shell root parse failed; resolving UNC components server=%s share=%s", path.server, path.share)
	folder, networkErr := shellNetworkFolder()
	if networkErr != nil {
		return nil, fmt.Errorf("%w; Network shell folder failed: %v", err, networkErr)
	}
	for _, component := range []string{path.server, path.share} {
		pidl, childErr := childPIDL(folder, component)
		if childErr != nil {
			releaseUnknown((*unknown)(unsafe.Pointer(folder)))
			return nil, fmt.Errorf("%w; resolving UNC component %q failed: %v", err, component, childErr)
		}
		next, bindErr := bindChildFolder(folder, pidl)
		procCoTaskMemFree.Call(pidl)
		releaseUnknown((*unknown)(unsafe.Pointer(folder)))
		if bindErr != nil {
			return nil, fmt.Errorf("%w; binding UNC component %q failed: %v", err, component, bindErr)
		}
		folder = next
	}
	return folder, nil
}

func shellFolderFromAbsolutePath(path string) (*shellFolder, error) {
	pidl, err := parseDisplayName(path)
	if err != nil {
		return nil, err
	}
	defer procCoTaskMemFree.Call(pidl)
	return shellFolderFromAbsolutePIDL(pidl)
}

func shellNetworkFolder() (*shellFolder, error) {
	var pidl uintptr
	hr, _, _ := procSHGetSpecialFolderLocation.Call(0, csidlNetwork, uintptr(unsafe.Pointer(&pidl)))
	if failed(hr) {
		return nil, fmt.Errorf("SHGetSpecialFolderLocation(CSIDL_NETWORK) failed: 0x%x", uint32(hr))
	}
	defer procCoTaskMemFree.Call(pidl)
	return shellFolderFromAbsolutePIDL(pidl)
}

func shellFolderFromAbsolutePIDL(pidl uintptr) (*shellFolder, error) {
	var parent *shellFolder
	var child uintptr
	hr, _, _ := procSHBindToParent.Call(
		pidl,
		uintptr(unsafe.Pointer(&iidIShellFolder)),
		uintptr(unsafe.Pointer(&parent)),
		uintptr(unsafe.Pointer(&child)),
	)
	if failed(hr) {
		return nil, fmt.Errorf("SHBindToParent failed: 0x%x", uint32(hr))
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(parent)))
	return bindChildFolder(parent, child)
}

func bindChildFolder(parent *shellFolder, child uintptr) (*shellFolder, error) {
	var folder *shellFolder
	hr, _, _ := syscall.SyscallN(
		parent.vtbl.bindToObject,
		uintptr(unsafe.Pointer(parent)),
		child,
		0,
		uintptr(unsafe.Pointer(&iidIShellFolder)),
		uintptr(unsafe.Pointer(&folder)),
	)
	if failed(hr) {
		return nil, fmt.Errorf("IShellFolder.BindToObject failed: 0x%x", uint32(hr))
	}
	return folder, nil
}

func shellItemFromFolderAndPIDL(folder *shellFolder, pidl uintptr) (*shellItem, error) {
	var item *shellItem
	hr, _, _ := procSHCreateShellItem.Call(
		0,
		uintptr(unsafe.Pointer(folder)),
		pidl,
		uintptr(unsafe.Pointer(&item)),
	)
	if failed(hr) {
		return nil, fmt.Errorf("SHCreateShellItem failed: 0x%x", uint32(hr))
	}
	return item, nil
}

func newFileOperation() (*fileOperation, error) {
	var op *fileOperation
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOperation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOperation)),
		uintptr(unsafe.Pointer(&op)),
	)
	if failed(hr) {
		return nil, fmt.Errorf("CoCreateInstance(CLSID_FileOperation) failed: 0x%x", uint32(hr))
	}
	return op, nil
}

func setFileOperationFlags(op *fileOperation, flags uint32) error {
	hr, _, _ := syscall.SyscallN(
		op.vtbl.setOperationFlags,
		uintptr(unsafe.Pointer(op)),
		uintptr(flags),
	)
	if failed(hr) {
		return fmt.Errorf("IFileOperation.SetOperationFlags failed: 0x%x", uint32(hr))
	}
	return nil
}

func queueRecycleDelete(op *fileOperation, item *shellItem) error {
	hr, _, _ := syscall.SyscallN(
		op.vtbl.deleteItem,
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(item)),
		0,
	)
	if failed(hr) {
		return fmt.Errorf("IFileOperation.DeleteItem failed: 0x%x", uint32(hr))
	}
	return nil
}

func performFileOperations(op *fileOperation) error {
	hr, _, _ := syscall.SyscallN(
		op.vtbl.performOperations,
		uintptr(unsafe.Pointer(op)),
	)
	if failed(hr) {
		return fmt.Errorf("IFileOperation.PerformOperations failed: 0x%x", uint32(hr))
	}
	return nil
}

func checkFileOperationAborted(op *fileOperation) error {
	var aborted int32
	hr, _, _ := syscall.SyscallN(
		op.vtbl.getAnyOperationsAborted,
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(&aborted)),
	)
	if failed(hr) {
		return fmt.Errorf("IFileOperation.GetAnyOperationsAborted failed: 0x%x", uint32(hr))
	}
	if aborted != 0 {
		return fmt.Errorf("delete to recycle bin was aborted")
	}
	return nil
}

func childPIDL(folder *shellFolder, name string) (uintptr, error) {
	wideName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	var eaten uint32
	var pidl uintptr
	var attributes uint32
	hr, _, _ := syscall.SyscallN(
		folder.vtbl.parseDisplayName,
		uintptr(unsafe.Pointer(folder)),
		0,
		0,
		uintptr(unsafe.Pointer(wideName)),
		uintptr(unsafe.Pointer(&eaten)),
		uintptr(unsafe.Pointer(&pidl)),
		uintptr(unsafe.Pointer(&attributes)),
	)
	if !failed(hr) {
		return pidl, nil
	}

	parseErr := fmt.Errorf("IShellFolder.ParseDisplayName failed for %s: 0x%x", name, uint32(hr))
	dbg("Child PIDL parse failed; enumerating name=%s", name)
	pidl, enumErr := findChildPIDLByName(folder, name)
	if enumErr == nil {
		dbg("Child PIDL enumeration fallback succeeded name=%s", name)
		return pidl, nil
	}
	return 0, fmt.Errorf("%w; directory enumeration fallback failed: %v", parseErr, enumErr)
}

func findChildPIDLByName(folder *shellFolder, name string) (uintptr, error) {
	var enumerator *enumIDList
	hr, _, _ := syscall.SyscallN(
		folder.vtbl.enumObjects,
		uintptr(unsafe.Pointer(folder)),
		0,
		shcontfFolders|shcontfNonFolders|shcontfIncludeHidden,
		uintptr(unsafe.Pointer(&enumerator)),
	)
	if failed(hr) {
		return 0, fmt.Errorf("IShellFolder.EnumObjects failed: 0x%x", uint32(hr))
	}
	defer releaseUnknown((*unknown)(unsafe.Pointer(enumerator)))

	var caseInsensitiveMatch uintptr
	for {
		var pidl uintptr
		var fetched uint32
		hr, _, _ = syscall.SyscallN(
			enumerator.vtbl.next,
			uintptr(unsafe.Pointer(enumerator)),
			1,
			uintptr(unsafe.Pointer(&pidl)),
			uintptr(unsafe.Pointer(&fetched)),
		)
		if uint32(hr) == sFalse || fetched == 0 {
			break
		}
		if failed(hr) {
			if caseInsensitiveMatch != 0 {
				procCoTaskMemFree.Call(caseInsensitiveMatch)
			}
			return 0, fmt.Errorf("IEnumIDList.Next failed: 0x%x", uint32(hr))
		}

		displayName, err := childDisplayName(folder, pidl)
		if err == nil && displayName == name {
			if caseInsensitiveMatch != 0 {
				procCoTaskMemFree.Call(caseInsensitiveMatch)
			}
			return pidl, nil
		}
		if err == nil && caseInsensitiveMatch == 0 && strings.EqualFold(displayName, name) {
			caseInsensitiveMatch = pidl
			continue
		}
		procCoTaskMemFree.Call(pidl)
	}
	if caseInsensitiveMatch != 0 {
		return caseInsensitiveMatch, nil
	}
	return 0, fmt.Errorf("child %q was not found", name)
}

func childDisplayName(folder *shellFolder, pidl uintptr) (string, error) {
	var value strret
	hr, _, _ := syscall.SyscallN(
		folder.vtbl.getDisplayNameOf,
		uintptr(unsafe.Pointer(folder)),
		pidl,
		0x0001, // SHGDN_INFOLDER
		uintptr(unsafe.Pointer(&value)),
	)
	if failed(hr) {
		return "", fmt.Errorf("IShellFolder.GetDisplayNameOf failed: 0x%x", uint32(hr))
	}
	buf := make([]uint16, 1024)
	hr, _, _ = procStrRetToBufW.Call(
		uintptr(unsafe.Pointer(&value)),
		pidl,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if failed(hr) {
		return "", fmt.Errorf("StrRetToBufW failed: 0x%x", uint32(hr))
	}
	return windows.UTF16PtrToString(&buf[0]), nil
}

func freePIDLs(pidls []uintptr) {
	for _, pidl := range pidls {
		procCoTaskMemFree.Call(pidl)
	}
}

func parseDisplayName(path string) (uintptr, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var pidl uintptr
	hr, _, _ := procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		0,
	)
	if failed(hr) {
		return 0, fmt.Errorf("SHParseDisplayName failed for %s: 0x%x", path, uint32(hr))
	}
	return pidl, nil
}

func queryInterface(obj *unknown, iid *windows.GUID, out unsafe.Pointer) error {
	hr, _, _ := syscall.SyscallN(
		obj.vtbl.queryInterface,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(out),
	)
	if failed(hr) {
		return fmt.Errorf("QueryInterface failed: 0x%x", uint32(hr))
	}
	return nil
}

func releaseUnknown(obj *unknown) {
	if obj == nil {
		return
	}
	syscall.SyscallN(obj.vtbl.release, uintptr(unsafe.Pointer(obj)))
}

func failed(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}

func newDropSource() *dropSource {
	return &dropSource{vtbl: &dropSourceVtblInst, refs: 1}
}

func dropSourceQueryInterface(this uintptr, riid uintptr, ppv uintptr) uintptr {
	if ppv == 0 {
		return uintptr(ePointer)
	}
	out := (*uintptr)(unsafe.Pointer(ppv))
	*out = 0
	if riid == 0 {
		return uintptr(eNoInterface)
	}
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidIUnknown || *guid == iidIDropSource {
		*out = this
		dropSourceAddRef(this)
		return sOK
	}
	return uintptr(eNoInterface)
}

func dropSourceAddRef(this uintptr) uintptr {
	if this == 0 {
		return 0
	}
	source := (*dropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddUint32(&source.refs, 1))
}

func dropSourceRelease(this uintptr) uintptr {
	if this == 0 {
		return 0
	}
	source := (*dropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddUint32(&source.refs, ^uint32(0)))
}

func dropSourceQueryContinueDrag(_ uintptr, escapePressed uintptr, keyState uintptr) uintptr {
	if escapePressed != 0 {
		return uintptr(dragdropSCancel)
	}
	if keyState&mouseKeyStateLeftButton == 0 {
		return uintptr(dragdropSDrop)
	}
	return sOK
}

func dropSourceGiveFeedback(_ uintptr, _ uintptr) uintptr {
	return uintptr(dragdropSUseDefaultCursors)
}

type menuOwner struct {
	hwnd uintptr
	data *menuOwnerData
}

func newMenuOwner(parent uintptr, menu2 *contextMenu2, menu3 *contextMenu3) (*menuOwner, error) {
	className, _ := windows.UTF16PtrFromString("nmfShellContextMenuOwner")
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   menuOwnerWndProcPtr,
		lpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	owner := &menuOwner{}
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		parent,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW failed: %w", err)
	}
	owner.hwnd = hwnd
	owner.data = &menuOwnerData{menu2: menu2, menu3: menu3}
	setMenuOwnerData(hwnd, owner.data)
	return owner, nil
}

func (o *menuOwner) destroy() {
	if o == nil || o.hwnd == 0 {
		return
	}
	data := getMenuOwnerData(o.hwnd)
	if data != nil {
		data.free()
		setMenuOwnerData(o.hwnd, nil)
	}
	procDestroyWindow.Call(o.hwnd)
	o.data = nil
	o.hwnd = 0
}

type menuOwnerData struct {
	menu2 *contextMenu2
	menu3 *contextMenu3
}

func (d *menuOwnerData) free() {
	// The COM objects are owned and released by Show; this just drops the Go wrapper.
}

func setMenuOwnerData(hwnd uintptr, data *menuOwnerData) {
	var ptr uintptr
	if data != nil {
		ptr = uintptr(unsafe.Pointer(data))
	}
	procSetWindowLongPtrW.Call(hwnd, gwlUserData, ptr)
}

func getMenuOwnerData(hwnd uintptr) *menuOwnerData {
	ptr, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlUserData)
	if ptr == 0 {
		return nil
	}
	return (*menuOwnerData)(unsafe.Pointer(ptr))
}

func menuOwnerWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmInitMenuPopup, wmDrawItem, wmMeasureItem, wmMenuChar:
		if data := getMenuOwnerData(hwnd); data != nil {
			if data.menu3 != nil {
				var result uintptr
				hr, _, _ := syscall.SyscallN(
					data.menu3.vtbl.handleMenuMsg2,
					uintptr(unsafe.Pointer(data.menu3)),
					uintptr(msg),
					wParam,
					lParam,
					uintptr(unsafe.Pointer(&result)),
				)
				if !failed(hr) {
					return result
				}
			}
			if data.menu2 != nil {
				hr, _, _ := syscall.SyscallN(
					data.menu2.vtbl.handleMenuMsg,
					uintptr(unsafe.Pointer(data.menu2)),
					uintptr(msg),
					wParam,
					lParam,
				)
				if !failed(hr) {
					return 0
				}
			}
		}
	case wmNCDestroy:
		setMenuOwnerData(hwnd, nil)
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}
