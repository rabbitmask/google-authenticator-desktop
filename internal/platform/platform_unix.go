//go:build !windows

package platform

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

//go:embed appicon.png
var TrayIcon []byte

type ProcessMetadata struct {
	PID        int
	Executable string
	StartedAt  int64
}

func SupportsSingleInstance() bool {
	return false
}

func AcquireSingleInstance(_ string) bool {
	return false
}

func ReleaseSingleInstance() {}

func ShowMessage(title, message string) {
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`display dialog "%s" with title "%s" buttons {"OK"} default button "OK"`, message, title)
		exec.Command("osascript", "-e", script).Run()
		return
	}

	if err := exec.Command("zenity", "--info", "--title="+title, "--text="+message).Run(); err != nil {
		exec.Command("xmessage", "-center", message).Run()
	}
}

func IsProcessRunning(pid int) bool {
	if pid == os.Getpid() {
		return false
	}
	_, ok := GetProcessMetadata(pid)
	return ok
}

func GetProcessMetadata(pid int) (ProcessMetadata, bool) {
	if pid <= 0 {
		return ProcessMetadata{}, false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return ProcessMetadata{}, false
	}
	if err := process.Signal(os.Signal(nil)); err != nil {
		return ProcessMetadata{}, false
	}

	return ProcessMetadata{PID: pid}, true
}
