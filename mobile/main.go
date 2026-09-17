// go-stock Android 移动端入口（Wails v3）。
//
// 复用主模块 backend/ 的 handler 与数据层（见 go.mod 的 replace go-stock => ../），
// 通过 backend/emitter 注入 v3 app.Event.Emit 适配，实现与桌面端同一套业务代码。
package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"go-stock/backend/db"
	"go-stock/backend/emitter"
	"go-stock/backend/handler"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Android 上把工作目录切到应用私有目录，使 db.Init("") 的默认相对路径
	// data/stock.db 落在应用沙箱内；桌面(开发模式)StoragePath 为空，行为不变。
	if storage := application.Mobile.StoragePath(); storage != "" {
		if err := os.MkdirAll(filepath.Join(storage, "data"), 0o755); err != nil {
			log.Printf("create data dir failed: %v", err)
		}
		if err := os.Chdir(storage); err != nil {
			log.Printf("chdir to storage failed: %v", err)
		}
	}
	db.Init("")

	// v3 事件发射适配：backend 层所有流式/告警事件经此推送到 WebView。
	// application.Get() 在 app.Run 之前为 nil，故在调用期惰性取用。
	emit := emitter.Emitter(func(event string, payload ...any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(event, payload...)
		}
	})
	ctxFn := func() context.Context { return context.Background() }

	stockHandler := handler.NewStockHandler()
	agentHandler := handler.NewAgentHandler(ctxFn, emit)
	mobileSvc := NewMobileService()

	app := application.New(application.Options{
		Name:        "go-stock",
		Description: "go-stock AI 股票分析 移动端",
		Services: []application.Service{
			application.NewService(stockHandler),
			application.NewService(agentHandler),
			application.NewService(mobileSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Android: application.AndroidOptions{},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "go-stock",
		BackgroundColour: application.NewRGB(18, 18, 18),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
