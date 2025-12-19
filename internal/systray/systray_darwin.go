//go:build darwin

package systray

// macOS 上不使用 energye/systray，因为会与 Wails 框架的 AppDelegate 冲突
// 使用 Wails 内置的 HideWindowOnClose 功能替代
// 用户可以通过 Cmd+Q 或菜单栏退出应用

// initPlatformSystray macOS 空实现
func initPlatformSystray() {
	// macOS: 不启动系统托盘，避免 AppDelegate 符号冲突
}

// quitPlatformSystray macOS 空实现
func quitPlatformSystray() {
	// macOS: 无需退出托盘
}
