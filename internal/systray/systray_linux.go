//go:build linux

package systray

import (
	"google-authenticator/internal/platform"

	"github.com/energye/systray"
)

// initPlatformSystray Linux 系统托盘初始化
func initPlatformSystray() {
	systray.Run(onReady, onExit)
}

// quitPlatformSystray 退出系统托盘
func quitPlatformSystray() {
	systray.Quit()
}

func onReady() {
	systray.SetIcon(platform.TrayIcon)
	systray.SetTitle("Google Authenticator")
	systray.SetTooltip("Google Authenticator - 双因素验证")

	mShow := systray.AddMenuItem("显示主窗口", "显示应用程序窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "完全退出应用程序")

	mShow.Click(func() {
		if callbacks.OnShowWindow != nil {
			callbacks.OnShowWindow()
		}
	})

	mQuit.Click(func() {
		if callbacks.OnQuit != nil {
			callbacks.OnQuit()
		}
	})

	// 左键点击托盘图标
	systray.SetOnClick(func(menu systray.IMenu) {
		if callbacks.OnShowWindow != nil {
			callbacks.OnShowWindow()
		}
	})

	// 双击托盘图标
	systray.SetOnDClick(func(menu systray.IMenu) {
		if callbacks.OnShowWindow != nil {
			callbacks.OnShowWindow()
		}
	})
}

func onExit() {
	if callbacks.OnQuit != nil {
		callbacks.OnQuit()
	}
}
