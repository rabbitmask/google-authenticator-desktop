package main

import (
	"embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google-authenticator/internal/platform"
	"google-authenticator/internal/tray"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

var app *App
var lockFilePath string

// getLockFilePath 获取锁文件路径
func getLockFilePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)
	dataDir := filepath.Join(execDir, "data")
	return filepath.Join(dataDir, ".lock")
}

// acquireLock 获取单实例锁
func acquireLock() bool {
	lockFilePath = getLockFilePath()
	if lockFilePath == "" {
		return true // 无法获取路径，允许启动
	}

	// 确保 data 目录存在
	dataDir := filepath.Dir(lockFilePath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return true // 无法创建目录，允许启动
	}

	// 检查锁文件是否存在
	if data, err := os.ReadFile(lockFilePath); err == nil {
		// 锁文件存在，检查 PID
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			if platform.IsProcessRunning(pid) {
				// 进程仍在运行
				return false
			}
		}
		// 进程已退出，删除残留锁文件
		err := os.Remove(lockFilePath)
		if err != nil {
			return false
		}
	}

	// 创建锁文件，写入当前 PID
	pid := os.Getpid()
	if err := os.WriteFile(lockFilePath, []byte(strconv.Itoa(pid)), 0600); err != nil {
		return true // 写入失败，允许启动
	}

	return true
}

// releaseLock 释放单实例锁
func releaseLock() {
	if lockFilePath != "" {
		err := os.Remove(lockFilePath)
		if err != nil {
			return
		}
	}
}

func main() {
	// 单实例检测
	if !acquireLock() {
		platform.ShowMessage("Google Authenticator", "程序已在运行中，请检查系统托盘。")
		os.Exit(0)
	}
	defer releaseLock()

	app = NewApp()

	appMenu := menu.NewMenu()

	// === 文件菜单 ===
	fileMenu := appMenu.AddSubmenu("文件")

	addSubmenu := fileMenu.AddSubmenu("添加账户")
	addSubmenu.AddText("手动输入", keys.CmdOrCtrl("N"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:add-manual")
	})
	addSubmenu.AddText("扫描二维码", keys.CmdOrCtrl("I"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:scan-qr")
	})

	transferSubmenu := fileMenu.AddSubmenu("转移验证码")
	transferSubmenu.AddText("导入迁移码", keys.CmdOrCtrl("O"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:transfer-import")
	})
	transferSubmenu.AddText("导出迁移码", keys.CmdOrCtrl("E"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:transfer-export")
	})

	fileMenu.AddSeparator()
	fileMenu.AddText("退出", keys.CmdOrCtrl("Q"), func(_ *menu.CallbackData) {
		app.CloseDB()
		releaseLock()
		tray.Quit()
		os.Exit(0)
	})

	// === 编辑菜单 ===
	editMenu := appMenu.AddSubmenu("编辑")
	editMenu.AddText("全选", keys.CmdOrCtrl("A"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:select-all")
	})
	editMenu.AddSeparator()
	editMenu.AddText("设置", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:settings")
	})

	// === 帮助菜单 ===
	helpMenu := appMenu.AddSubmenu("帮助")
	helpMenu.AddText("关于", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:about")
	})

	err := wails.Run(&options.App{
		Title:  "Google Authenticator",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Menu:             appMenu,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		HideWindowOnClose: true,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
