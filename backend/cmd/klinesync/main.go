// 命令行触发 K 线同步（与桌面端「数据管理 → 历史同步」同路径），便于部署目录拉数后打包。
//
// 用法（在仓库根目录）:
//
//	go run ./backend/cmd/klinesync -db "D:\_installers\go-stock\data\stock.db" -years 5 -wait 90m
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/backtest"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/datasource/fallback"
	"go-stock/backend/data/datasource/freestockdb"
	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func initRouter() {
	router := datasource.GetRouter()
	cache := datasource.NewCacheLayer(256)
	cache.AutoMigrate()
	router.SetCache(cache)

	cfg := data.GetSettingConfigSafe()
	freestockdb.Setup(router, freestockdb.Config{
		Enabled:   cfg.FreeStockDBEnable,
		ExePath:   cfg.FreeStockDBPath,
		Addr:      cfg.FreeStockDBAddr,
		AutoStart: cfg.FreeStockDBAutoStart,
	})

	fallback.RegisterQuoteChain(router)
	fallback.RegisterKLineChain(router)
	fallback.RegisterNewsChain(router)
	fallback.RegisterFundamentalChain(router)
	fallback.RegisterSectorChain(router)
	fallback.RegisterSnapshotChain(router)
	fallback.RegisterFreeDataSources(router)
}

func dsn(path string) string {
	if strings.Contains(path, "?") {
		return path
	}
	return path + "?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
}

func countSync(status string) int64 {
	var n int64
	_ = db.Dao.Model(&models.KLineSyncLog{}).Where("status = ?", status).Count(&n).Error
	return n
}

func main() {
	dbPath := flag.String("db", "", "SQLite 路径（如 D:\\_installers\\go-stock\\data\\stock.db）")
	years := flag.Int("years", 5, "历史同步年数")
	wait := flag.Duration("wait", 90*time.Minute, "最长等待 worker 清空 pending/running")
	flag.Parse()
	if strings.TrimSpace(*dbPath) == "" {
		fmt.Fprintln(os.Stderr, "必须指定 -db")
		os.Exit(2)
	}

	db.Init(dsn(*dbPath))
	initRouter()

	svc := backtest.NewService()
	history.PrepareSyncQueueOnStartup(func() {
		svc.ResumePendingSyncTasks()
	})

	if err := svc.StartHistoricalSync(*years); err != nil {
		fmt.Fprintf(os.Stderr, "StartHistoricalSync: %v\n", err)
		os.Exit(1)
	}
	svc.ResumePendingSyncTasks()

	deadline := time.Now().Add(*wait)
	for time.Now().Before(deadline) {
		pending := countSync(history.SyncStatusPending)
		running := countSync(history.SyncStatusRunning)
		fmt.Printf("[%s] pending=%d running=%d\n", time.Now().Format("15:04:05"), pending, running)
		if pending == 0 && running == 0 {
			fmt.Println("同步队列已空，结束等待")
			break
		}
		time.Sleep(30 * time.Second)
	}

	pending := countSync(history.SyncStatusPending)
	running := countSync(history.SyncStatusRunning)
	if pending > 0 || running > 0 {
		fmt.Printf("等待超时或仍有任务：pending=%d running=%d（可再次运行本命令或启动客户端继续）\n", pending, running)
	}
}
