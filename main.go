package main

import (
	"embed"
	"os"

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

func main() {
	if !platform.AcquireAppLock() {
		platform.ShowMessage("Google Authenticator", "程序已在运行中，请检查系统托盘。")
		os.Exit(0)
	}
	defer platform.ReleaseAppLock()

	app = NewApp()

	appMenu := menu.NewMenu()

	fileMenu := appMenu.AddSubmenu("文件")

	addSubmenu := fileMenu.AddSubmenu("添加账户")
	addSubmenu.AddText("手动输入", keys.CmdOrCtrl("N"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:add-manual")
	})
	addSubmenu.AddText("扫描二维码", keys.CmdOrCtrl("I"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:scan-qr")
	})

	transferSubmenu := fileMenu.AddSubmenu("迁移验证码")
	transferSubmenu.AddText("导入迁移码", keys.CmdOrCtrl("O"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:transfer-import")
	})
	transferSubmenu.AddText("导出迁移码", keys.CmdOrCtrl("E"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:transfer-export")
	})

	fileMenu.AddSeparator()
	fileMenu.AddText("退出", keys.CmdOrCtrl("Q"), func(_ *menu.CallbackData) {
		app.CloseDB()
		platform.ReleaseAppLock()
		tray.Quit()
		os.Exit(0)
	})

	editMenu := appMenu.AddSubmenu("编辑")
	editMenu.AddText("全选", keys.CmdOrCtrl("A"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:select-all")
	})
	editMenu.AddSeparator()
	editMenu.AddText("设置", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:settings")
	})

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
