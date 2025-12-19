// Package systray 提供跨平台系统托盘支持
package systray

// Callbacks 定义系统托盘的回调函数
type Callbacks struct {
	OnShowWindow func() // 显示主窗口
	OnQuit       func() // 退出应用
}

var callbacks Callbacks

// Init 初始化系统托盘（平台特定实现）
// 在 systray_windows.go, systray_linux.go, systray_darwin.go 中实现
func Init(cb Callbacks) {
	callbacks = cb
	initPlatformSystray()
}

// Quit 退出系统托盘
// 在平台特定文件中实现
func Quit() {
	quitPlatformSystray()
}
