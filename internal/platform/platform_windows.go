//go:build windows

package platform

import (
	_ "embed"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed icon.ico
var TrayIcon []byte

var singleInstanceHandle syscall.Handle

type ProcessMetadata struct {
	PID        int
	Executable string
	StartedAt  int64
}

func ShowMessage(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), 0x40)
}

func IsProcessRunning(pid int) bool {
	if pid == syscall.Getpid() {
		return false
	}
	_, ok := GetProcessMetadata(pid)
	return ok
}

func GetProcessMetadata(pid int) (ProcessMetadata, bool) {
	if pid <= 0 {
		return ProcessMetadata{}, false
	}

	const processQueryLimitedInformation = 0x1000
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	getProcessTimes := kernel32.NewProc("GetProcessTimes")
	closeHandle := kernel32.NewProc("CloseHandle")
	queryFullProcessImageName := kernel32.NewProc("QueryFullProcessImageNameW")

	handle, _, _ := openProcess.Call(
		processQueryLimitedInformation,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return ProcessMetadata{}, false
	}
	defer closeHandle.Call(handle)

	var creationTime syscall.Filetime
	var exitTime syscall.Filetime
	var kernelTime syscall.Filetime
	var userTime syscall.Filetime

	ret, _, _ := getProcessTimes.Call(
		handle,
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return ProcessMetadata{}, false
	}

	buffer := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buffer))
	ret, _, _ = queryFullProcessImageName.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)

	metadata := ProcessMetadata{
		PID:       pid,
		StartedAt: creationTime.Nanoseconds(),
	}
	if ret == 0 {
		return metadata, true
	}

	metadata.Executable = normalizeWindowsPath(syscall.UTF16ToString(buffer[:size]))
	return metadata, true
}

func normalizeWindowsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func SupportsSingleInstance() bool {
	return true
}

func AcquireSingleInstance(name string) bool {
	if singleInstanceHandle != 0 {
		return true
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createMutex := kernel32.NewProc("CreateMutexW")
	namePtr, err := syscall.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return false
	}

	handle, _, callErr := createMutex.Call(
		0,
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	if handle == 0 {
		_ = callErr
		return false
	}

	const errorAlreadyExists = 183
	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		syscall.CloseHandle(syscall.Handle(handle))
		return false
	}

	singleInstanceHandle = syscall.Handle(handle)
	return true
}

func ReleaseSingleInstance() {
	if singleInstanceHandle == 0 {
		return
	}
	syscall.CloseHandle(singleInstanceHandle)
	singleInstanceHandle = 0
}
