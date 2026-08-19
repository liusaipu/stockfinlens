package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data/stocks.json
var stockDB embed.FS

//go:embed ml_models/inference.py ml_models/engine_a_sentiment/*.onnx ml_models/engine_b_financial/*.onnx ml_models/engine_b_financial/*.pkl ml_models/engine_d_risk/model/*.pkl
var mlModels embed.FS

// readStockJSON 读取股票代码库，开发模式优先读本地文件，打包后 fallback 到 embed
func readStockJSON() ([]byte, error) {
	if b, err := os.ReadFile("data/stocks.json"); err == nil {
		return b, nil
	}
	return stockDB.ReadFile("data/stocks.json")
}

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// 创建系统菜单，启用 macOS 复制/粘贴快捷键（Cmd+C / Cmd+V）
	appMenu := menu.NewMenu()
	appMenu.Append(menu.EditMenu())

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "StockFinLens",
		Width:  1600,
		Height: 900,
		Menu:   appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:         &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:                app.startup,
		EnableDefaultContextMenu: true,
		Windows: &windows.Options{
			// 保持标题栏为深色，避免窗口顶部出现白色/空白条带
			Theme: windows.Dark,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
