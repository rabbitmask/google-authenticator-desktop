//go:build windows

package platform

import (
	_ "embed"
	"syscall"
	"unsafe"
)

//go:embed icon.ico
var TrayIcon []byte

// ShowMessage 显示消息提示框（Windows）
func ShowMessage(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), 0x40)
}

// IsProcessRunning 检查进程是否存在（Windows）
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == syscall.Getpid() {
		return false
	}

	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")

	handle, _, _ := openProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(pid),
	)

	if handle != 0 {
		syscall.CloseHandle(syscall.Handle(handle))
		return true
	}
	return false
}
