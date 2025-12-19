//go:build darwin

package tray

// macOS 不使用 energye/systray，因为与 Wails 的 AppDelegate 冲突
// 使用 Wails 内置的 HideWindowOnClose 功能
// 用户通过 Cmd+Q 或菜单退出

// Init macOS 空实现
func Init(onShow, onQuit func()) {
	// macOS: 不启动系统托盘，避免 AppDelegate 符号冲突
}

// Quit macOS 空实现
func Quit() {
	// macOS: 无需退出托盘
}
