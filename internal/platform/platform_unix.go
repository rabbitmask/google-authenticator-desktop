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

// ShowMessage 显示消息提示框（macOS/Linux）
func ShowMessage(title, message string) {
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`display dialog "%s" with title "%s" buttons {"确定"} default button "确定"`, message, title)
		exec.Command("osascript", "-e", script).Run()
	} else {
		if err := exec.Command("zenity", "--info", "--title="+title, "--text="+message).Run(); err != nil {
			exec.Command("xmessage", "-center", message).Run()
		}
	}
}

// IsProcessRunning 检查进程是否存在（Unix）
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(os.Signal(nil))
	return err == nil
}
