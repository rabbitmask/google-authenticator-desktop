//go:build windows || linux

package tray

import (
	"os"

	"google-authenticator/internal/platform"

	"github.com/energye/systray"
)

// Init 初始化系统托盘 (Windows/Linux)
func Init(onShow, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(platform.TrayIcon)
		systray.SetTitle("Google Authenticator")
		systray.SetTooltip("Google Authenticator - 双因素验证")

		mShow := systray.AddMenuItem("显示主窗口", "显示应用程序窗口")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "完全退出应用程序")

		mShow.Click(func() {
			if onShow != nil {
				onShow()
			}
		})

		mQuit.Click(func() {
			if onQuit != nil {
				onQuit()
			}
		})

		systray.SetOnClick(func(menu systray.IMenu) {
			if onShow != nil {
				onShow()
			}
		})

		systray.SetOnDClick(func(menu systray.IMenu) {
			if onShow != nil {
				onShow()
			}
		})
	}, func() {
		os.Exit(0)
	})
}

// Quit 退出系统托盘
func Quit() {
	systray.Quit()
}
