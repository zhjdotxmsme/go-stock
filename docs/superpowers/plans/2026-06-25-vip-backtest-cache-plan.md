# VIP 回测 + SQLite K 线缓存 + A 股历史数据种子 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 go-stock 中实现 AI 推荐股票的本地回测验证、日线 K 线 SQLite 持久化缓存，以及基于本地种子的 A 股历史数据初始化与增量更新。

**Architecture:** 在现有 `datasource` Provider 路由与 SQLite/GORM 基础上，新增 `kline_bars` 时间序列表和 `kline_sync_log` 同步进度表；新增 `backend/data/datasource/kline_store.go` 负责 K 线查询与 upsert；新增 `backend/data/history/sync.go` 负责增量同步与断点续传；新增 `backend/data/backtest/` 包实现 A 股特化回测引擎；新增 Python 种子脚本 `scripts/history_seed/baostock_seed.py` 用于一次性生成全量历史数据种子。

**Tech Stack:** Go 1.26, Wails v2, GORM, SQLite (WAL), mootdx/gotdx, baostock (Python seed), Eino

---

## 文件结构

### 新增文件

| 文件 | 职责 |
|---|---|
| `backend/models/kline_models.go` | GORM 模型：`KLineBar`、`KLineSyncLog`、`AiRecommendBacktest` |
| `backend/data/datasource/kline_store.go` | K 线存储：查询、upsert、最新日期、缺失区间计算 |
| `backend/data/datasource/kline_store_test.go` | K 线存储单元测试 |
| `backend/data/history/sync.go` | 历史数据同步任务、断点续传、增量更新 |
| `backend/data/history/sync_test.go` | 同步逻辑单元测试 |
| `backend/data/backtest/engine.go` | 单条推荐回测引擎（T+1、涨跌停、止盈止损、基准） |
| `backend/data/backtest/engine_test.go` | 回测引擎单元测试 |
| `backend/data/backtest/batch.go` | 批量回测、聚合统计、信号强度分组 |
| `backend/data/backtest/batch_test.go` | 批量回测单元测试 |
| `backend/data/backtest_service.go` | Wails 服务绑定：回测 API、历史同步 API、缓存统计 API |
| `scripts/history_seed/baostock_seed.py` | 生成 A 股历史日线种子（CSV/SQLite） |
| `scripts/history_seed/akshare_seed.py` | 备选：AKShare 生成种子 |
| `scripts/history_seed/README.md` | 种子脚本使用说明 |

### 修改文件

| 文件 | 修改内容 |
|---|---|
| `main.go` | 在 `AutoMigrate()` 中注册新表；在 `App` 的 `Bind` 中注册回测服务 |
| `backend/data/datasource/provider.go` | 如有必要，扩展 `KLineBar` 以包含 `Amount` 字段 |
| `app.go` | 暴露 Wails 上下文或注册回测服务方法（若通过 App 绑定） |

---

## Task 1: 新增 GORM 数据模型

**Files:**
- Create: `backend/models/kline_models.go`
- Modify: `main.go:302-337`

**说明：** 将 K 线相关模型从 `models/models.go` 中独立出来，避免该文件过大。

- [ ] **Step 1: 创建 `backend/models/kline_models.go`**

```go
package models

import (
	"time"

	"gorm.io/gorm"
)

// KLineBar 持久化 K 线（日线/周线/月线）
type KLineBar struct {
	ID        uint      `gorm:"primarykey"`
	StockCode string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:20"`
	Period    string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:10"` // day / week / month
	TradeDate string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:10"` // YYYY-MM-DD
	Adjusted  bool      `gorm:"index:idx_kline_code_period_date_adj,unique"`         // 是否前复权
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Amount    float64
	Source    string    `gorm:"size:20"` // tdx / tencent / eastmoney / seed
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (KLineBar) TableName() string { return "kline_bars" }

// KLineSyncLog 记录每只股票每个周期的同步进度
type KLineSyncLog struct {
	ID            uint      `gorm:"primarykey"`
	StockCode     string    `gorm:"index;size:20"`
	Period        string    `gorm:"size:10"`
	Adjusted      bool
	StartDate     string    `gorm:"size:10"`
	EndDate       string    `gorm:"size:10"`
	SyncedCount   int
	ExpectedCount int
	Status        string    `gorm:"size:20"` // pending / running / done / failed
	ErrorMsg      string    `gorm:"type:text"`
	UpdatedAt     time.Time
}

func (KLineSyncLog) TableName() string { return "kline_sync_log" }

// AiRecommendBacktest AI 推荐回测结果
type AiRecommendBacktest struct {
	gorm.Model
	AiRecommendID uint    `gorm:"index"`
	StockCode     string  `gorm:"index;size:20"`
	StockName     string  `gorm:"size:50"`
	SignalDate    string  `gorm:"index;size:10"`
	SignalRating  string  `gorm:"size:10"`
	EntryPrice    float64
	ExitPrice     float64
	ExitDate      string  `gorm:"size:10"`
	HoldingDays   int
	TotalReturn   float64
	MaxDrawdown   float64
	Csi300Return  float64
	Alpha         float64
	Win           bool
	Source        string  `gorm:"size:20"`
}

func (AiRecommendBacktest) TableName() string { return "ai_recommend_backtests" }
```

- [ ] **Step 2: 在 `main.go` 的 `AutoMigrate()` 中注册新表**

在 `db.Dao.AutoMigrate(&models.ConceptFundFlow{})` 之后添加：

```go
	db.Dao.AutoMigrate(&models.KLineBar{})
	db.Dao.AutoMigrate(&models.KLineSyncLog{})
	db.Dao.AutoMigrate(&models.AiRecommendBacktest{})
```

- [ ] **Step 3: 运行 build 检查模型无编译错误**

```bash
cd /mnt/e/open-source/ai/go-stock && go build ./backend/models/...
```

Expected: exit code 0

- [ ] **Step 4: Commit**

```bash
git add backend/models/kline_models.go main.go
git commit -m "feat(vip): add KLineBar, KLineSyncLog, AiRecommendBacktest models"
```

---

## Task 2: 实现 K 线存储层

**Files:**
- Create: `backend/data/datasource/kline_store.go`
- Create: `backend/data/datasource/kline_store_test.go`

- [ ] **Step 1: 创建 `backend/data/datasource/kline_store.go`**

```go
package datasource

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm/clause"
)

// KLineStore provides SQLite-backed K-line persistence.
type KLineStore struct{}

func NewKLineStore() *KLineStore {
	return &KLineStore{}
}

// QueryKLines returns bars for (code, period, adjusted) in [start, end].
func (s *KLineStore) QueryKLines(ctx context.Context, code, period, start, end string, adjusted bool) ([]models.KLineBar, error) {
	var bars []models.KLineBar
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ? AND trade_date BETWEEN ? AND ?",
			code, period, adjusted, start, end).
		Order("trade_date ASC").
		Find(&bars).Error
	return bars, err
}

// UpsertKLines inserts or updates bars in batches.
func (s *KLineStore) UpsertKLines(ctx context.Context, bars []models.KLineBar, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 1000
	}
	for i := 0; i < len(bars); i += batchSize {
		end := i + batchSize
		if end > len(bars) {
			end = len(bars)
		}
		batch := bars[i:end]
		if err := db.Dao.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stock_code"}, {Name: "period"}, {Name: "trade_date"}, {Name: "adjusted"}},
			UpdateAll: true,
		}).Create(&batch).Error; err != nil {
			return fmt.Errorf("upsert klines batch %d-%d: %w", i, end, err)
		}
	}
	return nil
}

// GetLatestTradeDate returns the latest trade date in cache.
func (s *KLineStore) GetLatestTradeDate(ctx context.Context, code, period string, adjusted bool) (string, error) {
	var bar models.KLineBar
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ?", code, period, adjusted).
		Order("trade_date DESC").
		First(&bar).Error
	if err != nil {
		return "", err
	}
	return bar.TradeDate, nil
}

// MissingRanges computes missing date intervals given [start, end] and existing bars.
func (s *KLineStore) MissingRanges(ctx context.Context, code, period, start, end string, adjusted bool) ([][2]string, error) {
	bars, err := s.QueryKLines(ctx, code, period, start, end, adjusted)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return [][2]string{{start, end}}, nil
	}

	existing := make(map[string]struct{}, len(bars))
	for _, b := range bars {
		existing[b.TradeDate] = struct{}{}
	}

	startT, _ := time.Parse("2006-01-02", start)
	endT, _ := time.Parse("2006-01-02", end)
	var missing [][2]string
	var rangeStart *time.Time
	for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		_, ok := existing[dateStr]
		if !ok {
			if rangeStart == nil {
				ts := d
				rangeStart = &ts
			}
		} else if rangeStart != nil {
			prev := d.AddDate(0, 0, -1)
			missing = append(missing, [2]string{rangeStart.Format("2006-01-02"), prev.Format("2006-01-02")})
			rangeStart = nil
		}
	}
	if rangeStart != nil {
		missing = append(missing, [2]string{rangeStart.Format("2006-01-02"), endT.Format("2006-01-02")})
	}
	return missing, nil
}

// BarsFromKLineData converts datasource.KLineData to []models.KLineBar.
func BarsFromKLineData(code, period, source string, adjusted bool, data *KLineData) []models.KLineBar {
	if data == nil {
		return nil
	}
	bars := make([]models.KLineBar, 0, len(data.Bars))
	for _, b := range data.Bars {
		bars = append(bars, models.KLineBar{
			StockCode: code,
			Period:    period,
			TradeDate: b.Time.Format("2006-01-02"),
			Adjusted:  adjusted,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			Volume:    b.Volume,
			Source:    source,
		})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate < bars[j].TradeDate })
	return bars
}
```

- [ ] **Step 2: 创建 `backend/data/datasource/kline_store_test.go`**

```go
package datasource

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKLineStore_QueryUpsertAndMissing(t *testing.T) {
	db.Init(":memory:?_journal_mode=WAL")
	defer func() {
		sqlDB, _ := db.Dao.DB()
		sqlDB.Close()
	}()

	store := NewKLineStore()
	ctx := context.Background()

	bars := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-02", Adjusted: true, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 1000, Source: "test"},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-04", Adjusted: true, Open: 101, High: 102, Low: 100, Close: 101.5, Volume: 2000, Source: "test"},
	}
	err := store.UpsertKLines(ctx, bars, 100)
	require.NoError(t, err)

	q, err := store.QueryKLines(ctx, "sh600519", "day", "2024-01-01", "2024-01-05", true)
	require.NoError(t, err)
	assert.Len(t, q, 2)

	latest, err := store.GetLatestTradeDate(ctx, "sh600519", "day", true)
	require.NoError(t, err)
	assert.Equal(t, "2024-01-04", latest)

	missing, err := store.MissingRanges(ctx, "sh600519", "day", "2024-01-01", "2024-01-05", true)
	require.NoError(t, err)
	assert.Len(t, missing, 2) // 01-01 and 01-03
}
```

- [ ] **Step 3: 运行测试**

```bash
cd /mnt/e/open-source/ai/go-stock && go test ./backend/data/datasource/ -run TestKLineStore -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/datasource/kline_store.go backend/data/datasource/kline_store_test.go
git commit -m "feat(vip): add K-line SQLite store with upsert and missing-range detection"
```

---

## Task 3: 扩展 Provider 路由以写入 K 线缓存

**Files:**
- Modify: `backend/data/datasource/router.go`

- [ ] **Step 1: 在 `GetKLine` 中增加 L2 K 线缓存写入**

修改 `GetKLine` 方法，当从 Provider 成功拉取数据后，异步写入 `kline_bars`：

```go
// 在 GetRouter 返回值后添加 store 初始化
var globalRouter *Router
var globalKLineStore *KLineStore
var once sync.Once

func GetRouter() *Router {
	once.Do(func() {
		globalRouter = &Router{}
		globalKLineStore = NewKLineStore()
	})
	return globalRouter
}
```

在 `GetKLine` 的成功分支中加入：

```go
		if err == nil {
			if r.cache != nil {
				key := CacheKey(DataTypeKLine, code, period, fmt.Sprintf("%d", count))
				_ = r.cache.Set(ctx, key, string(DataTypeKLine), data, 300*time.Second)
			}
			// Persist to kline_bars
			if globalKLineStore != nil && data != nil {
				bars := BarsFromKLineData(code, period, p.Name(), true, data)
				if len(bars) > 0 {
					_ = globalKLineStore.UpsertKLines(ctx, bars, 1000)
				}
			}
			return data, nil
		}
```

- [ ] **Step 2: 运行测试确保路由无回归**

```bash
cd /mnt/e/open-source/ai/go-stock && go vet ./backend/data/datasource/...
go test ./backend/data/datasource/...
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/data/datasource/router.go
git commit -m "feat(vip): persist fetched K-lines to SQLite cache via router"
```

---

## Task 4: 实现历史数据同步任务

**Files:**
- Create: `backend/data/history/sync.go`
- Create: `backend/data/history/sync_test.go`

- [ ] **Step 1: 创建 `backend/data/history/sync.go`**

```go
package history

import (
	"context"
	"fmt"
	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"time"
)

// SyncConfig controls historical sync behavior.
type SyncConfig struct {
	Years      int    // how many years back to sync
	Period     string // default "day"
	Adjusted   bool   // default true
	BatchSize  int    // bars per upsert batch
	RateLimit  time.Duration
	Codes      []string
}

// SyncProgress reports current sync state.
type SyncProgress struct {
	Total     int
	Done      int
	Failed    int
	Running   int
	LastError string
}

// Syncer orchestrates historical K-line synchronization.
type Syncer struct {
	router *datasource.Router
	store  *datasource.KLineStore
}

func NewSyncer() *Syncer {
	return &Syncer{
		router: datasource.GetRouter(),
		store:  datasource.NewKLineStore(),
	}
}

func (s *Syncer) Run(ctx context.Context, cfg SyncConfig) error {
	if cfg.Period == "" {
		cfg.Period = "day"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 500 * time.Millisecond
	}
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(-cfg.Years, 0, 0).Format("2006-01-02")

	codes := cfg.Codes
	if len(codes) == 0 {
		codes = s.loadAllAShareCodes()
	}

	for _, code := range codes {
		if err := s.syncOne(ctx, code, cfg.Period, start, end, cfg.Adjusted, cfg.RateLimit); err != nil {
			logger.SugaredLogger.Warnf("history sync %s failed: %v", code, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.RateLimit):
		}
	}
	return nil
}

func (s *Syncer) loadAllAShareCodes() []string {
	var infos []models.AllStockInfo
	err := db.Dao.Select("secucode").Find(&infos).Error
	if err != nil {
		logger.SugaredLogger.Errorf("load all stock info failed: %v", err)
		return nil
	}
	codes := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.SECUCODE != "" {
			codes = append(codes, info.SECUCODE)
		}
	}
	return codes
}

func (s *Syncer) syncOne(ctx context.Context, code, period, start, end string, adjusted bool, rateLimit time.Duration) error {
	logID := uint(0)
	var log models.KLineSyncLog
	err := db.Dao.Where("stock_code = ? AND period = ? AND adjusted = ?", code, period, adjusted).First(&log).Error
	if err == nil && log.Status == "done" {
		return nil
	}
	if err == nil {
		logID = log.ID
	}

	s.updateLog(logID, code, period, adjusted, start, end, "running", "", 0, 0)

	count := 2000 // enough for 5+ years daily
	data, err := s.router.GetKLine(ctx, code, period, count)
	if err != nil {
		s.updateLog(logID, code, period, adjusted, start, end, "failed", err.Error(), 0, 0)
		return err
	}

	bars := datasource.BarsFromKLineData(code, period, "sync", adjusted, data)
	var filtered []models.KLineBar
	for _, b := range bars {
		if b.TradeDate >= start && b.TradeDate <= end {
			filtered = append(filtered, b)
		}
	}
	if err := s.store.UpsertKLines(ctx, filtered, 1000); err != nil {
		s.updateLog(logID, code, period, adjusted, start, end, "failed", err.Error(), 0, len(filtered))
		return err
	}

	s.updateLog(logID, code, period, adjusted, start, end, "done", "", len(filtered), len(filtered))
	return nil
}

func (s *Syncer) updateLog(id uint, code, period string, adjusted bool, start, end, status, errMsg string, synced, expected int) {
	log := models.KLineSyncLog{
		StockCode:     code,
		Period:        period,
		Adjusted:      adjusted,
		StartDate:     start,
		EndDate:       end,
		SyncedCount:   synced,
		ExpectedCount: expected,
		Status:        status,
		ErrorMsg:      errMsg,
	}
	if id > 0 {
		log.ID = id
		db.Dao.Save(&log)
	} else {
		db.Dao.Create(&log)
	}
}

func (s *Syncer) Progress(ctx context.Context) (*SyncProgress, error) {
	var total, done, failed int64
	db.Dao.Model(&models.KLineSyncLog{}).Count(&total)
	db.Dao.Model(&models.KLineSyncLog{}).Where("status = ?", "done").Count(&done)
	db.Dao.Model(&models.KLineSyncLog{}).Where("status = ?", "failed").Count(&failed)
	return &SyncProgress{Total: int(total), Done: int(done), Failed: int(failed)}, nil
}
```

- [ ] **Step 2: 创建 `backend/data/history/sync_test.go`**

```go
package history

import (
	"context"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncer_loadAllAShareCodes(t *testing.T) {
	db.Init(":memory:?_journal_mode=WAL")
	defer func() {
		sqlDB, _ := db.Dao.DB()
		sqlDB.Close()
	}()

	db.Dao.Create(&models.AllStockInfo{SECUCODE: "sh600519"})
	db.Dao.Create(&models.AllStockInfo{SECUCODE: "sz000001"})

	syncer := NewSyncer()
	codes := syncer.loadAllAShareCodes()
	assert.Len(t, codes, 2)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd /mnt/e/open-source/ai/go-stock && go test ./backend/data/history/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/history/sync.go backend/data/history/sync_test.go
git commit -m "feat(vip): add historical K-line sync task with checkpointing"
```

---

## Task 5: 创建 Python 种子脚本

**Files:**
- Create: `scripts/history_seed/baostock_seed.py`
- Create: `scripts/history_seed/README.md`

- [ ] **Step 1: 创建 `scripts/history_seed/baostock_seed.py`**

```python
#!/usr/bin/env python3
"""Generate A-share historical daily K-line seed using Baostock.

Usage:
    python baostock_seed.py --start 20190101 --output ./history_seed/
"""
import argparse
import os
import time
from datetime import datetime
from pathlib import Path

import baostock as bs
import pandas as pd


FIELD_SET = "date,code,open,high,low,close,preclose,volume,amount,pctChg"


def normalize_code(code: str) -> str:
    # Baostock code like sh.600519 -> sh600519
    return code.replace(".", "")


def fetch_one(code: str, start_date: str, end_date: str, adjust: str = "2") -> pd.DataFrame:
    rs = bs.query_history_k_data_plus(
        code,
        FIELD_SET,
        start_date=start_date,
        end_date=end_date,
        frequency="d",
        adjustflag=adjust,
    )
    data = []
    while (rs.error_code == "0") and rs.next():
        data.append(rs.get_row_data())
    df = pd.DataFrame(data, columns=rs.fields)
    if not df.empty:
        df["code"] = df["code"].apply(normalize_code)
    return df


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--start", default="20190101", help="start date YYYYMMDD")
    parser.add_argument("--end", default=datetime.now().strftime("%Y%m%d"), help="end date YYYYMMDD")
    parser.add_argument("--output", default="./history_seed", help="output directory")
    parser.add_argument("--format", default="csv", choices=["csv", "parquet", "sqlite"])
    parser.add_argument("--adjust", default="2", help="1=hfq, 2=qfq, 3=none")
    args = parser.parse_args()

    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)

    lg = bs.login()
    if lg.error_code != "0":
        raise RuntimeError(f"Baostock login failed: {lg.error_msg}")

    try:
        rs = bs.query_all_stock(day=datetime.now().strftime("%Y-%m-%d"))
        stocks = []
        while (rs.error_code == "0") and rs.next():
            stocks.append(rs.get_row_data()[0])
        print(f"Total stocks: {len(stocks)}")

        all_frames = []
        for i, code in enumerate(stocks):
            try:
                df = fetch_one(code, args.start, args.end, args.adjust)
                if not df.empty:
                    all_frames.append(df)
                if args.format == "csv":
                    df.to_csv(out / f"{normalize_code(code)}.csv", index=False)
            except Exception as e:
                print(f"Fetch {code} failed: {e}")
            if i % 100 == 0:
                print(f"Progress: {i}/{len(stocks)}")
                time.sleep(0.5)

        if args.format == "parquet" and all_frames:
            pd.concat(all_frames, ignore_index=True).to_parquet(out / "history_seed.parquet", index=False)
        elif args.format == "sqlite" and all_frames:
            import sqlite3
            conn = sqlite3.connect(out / "history_seed.db")
            pd.concat(all_frames, ignore_index=True).to_sql("kline_bars", conn, if_exists="replace", index=False)
            conn.close()
    finally:
        bs.logout()


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: 创建 `scripts/history_seed/README.md`**

```markdown
# A 股历史数据种子生成

由于目前不存在可直接下载的完整免费 A 股历史日线数据集，go-stock 提供 Python 种子脚本，由用户本地一次性生成。

## 依赖

```bash
pip install baostock pandas
```

## 生成 CSV 种子（默认）

```bash
python scripts/history_seed/baostock_seed.py --start 20190101 --output ./history_seed/
```

## 生成单个 Parquet 文件

```bash
python scripts/history_seed/baostock_seed.py --start 20190101 --output ./history_seed/ --format parquet
```

## 生成 SQLite 种子（可直接导入 go-stock）

```bash
python scripts/history_seed/baostock_seed.py --start 20190101 --output ./history_seed/ --format sqlite
```

## 导入 go-stock

在 go-stock 的"数据管理"页面选择种子目录或 SQLite 文件，点击"导入本地历史数据"。

后续日常增量由 go-stock 自动通过 mootdx / 腾讯 / 东财完成，无需再运行 Python 脚本。
```

- [ ] **Step 3: Commit**

```bash
git add scripts/history_seed/baostock_seed.py scripts/history_seed/README.md
git commit -m "feat(vip): add Baostock seed script for A-share historical data"
```

---

## Task 6: 实现回测引擎

**Files:**
- Create: `backend/data/backtest/engine.go`
- Create: `backend/data/backtest/engine_test.go`

- [ ] **Step 1: 创建 `backend/data/backtest/engine.go`**

```go
package backtest

import (
	"context"
	"fmt"
	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// Input defines a single backtest run.
type Input struct {
	StockCode    string
	SignalDate   string
	SignalRating string
	EntryPrice   float64 // 0 means use signal date close
	HoldingDays  int
	StopLoss     float64 // e.g. 0.05
	StopProfit   float64 // e.g. 0.10
	Adjusted     bool
	Benchmark    string // e.g. "sh510300"
}

// Result is the outcome of one backtest.
type Result struct {
	StockCode       string
	SignalDate      string
	EntryPrice      float64
	ExitPrice       float64
	ExitDate        string
	HoldingDays     int
	TotalReturn     float64
	MaxDrawdown     float64
	BenchmarkReturn float64
	Alpha           float64
	Win             bool
	SlippageWarning string
}

// Engine runs backtests against cached K-line data.
type Engine struct {
	store *datasource.KLineStore
}

func NewEngine() *Engine {
	return &Engine{store: datasource.NewKLineStore()}
}

func (e *Engine) Run(ctx context.Context, in Input) (*Result, error) {
	if in.HoldingDays <= 0 {
		in.HoldingDays = 5
	}
	if in.Benchmark == "" {
		in.Benchmark = "sh510300"
	}

	bars, err := e.store.QueryKLines(ctx, in.StockCode, "day", in.SignalDate, "", in.Adjusted)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("no kline data for %s from %s", in.StockCode, in.SignalDate)
	}

	signalBar := bars[0]
	entry := in.EntryPrice
	if entry <= 0 {
		entry = signalBar.Close
	}

	maxPrice := entry
	exitPrice := entry
	exitIdx := 0
	warning := ""

	for i := 1; i < len(bars) && i <= in.HoldingDays; i++ {
		bar := bars[i]
		if bar.High > maxPrice {
			maxPrice = bar.High
		}

		// Stop-loss / stop-profit on intra-day touch
		if in.StopLoss > 0 && bar.Low <= entry*(1-in.StopLoss) {
			exitPrice = entry * (1 - in.StopLoss)
			exitIdx = i
			break
		}
		if in.StopProfit > 0 && bar.High >= entry*(1+in.StopProfit) {
			exitPrice = entry * (1 + in.StopProfit)
			exitIdx = i
			break
		}

		exitPrice = bar.Close
		exitIdx = i
	}

	if signalBar.Close >= signalBar.High*0.999 {
		warning = "buy-day limit-up"
	}
	if exitIdx > 0 && bars[exitIdx].Close <= bars[exitIdx].Low*1.001 {
		if warning != "" {
			warning += "; "
		}
		warning += "sell-day limit-down"
	}

	ret := (exitPrice - entry) / entry
	maxDD := (maxPrice - exitPrice) / maxPrice
	if maxDD < 0 {
		maxDD = 0
	}

	benchRet := 0.0
	benchBars, _ := e.store.QueryKLines(ctx, in.Benchmark, "day", in.SignalDate, bars[exitIdx].TradeDate, in.Adjusted)
	if len(benchBars) >= 2 {
		benchRet = (benchBars[len(benchBars)-1].Close - benchBars[0].Close) / benchBars[0].Close
	}

	return &Result{
		StockCode:       in.StockCode,
		SignalDate:      in.SignalDate,
		EntryPrice:      entry,
		ExitPrice:       exitPrice,
		ExitDate:        bars[exitIdx].TradeDate,
		HoldingDays:     exitIdx,
		TotalReturn:     ret,
		MaxDrawdown:     maxDD,
		BenchmarkReturn: benchRet,
		Alpha:           ret - benchRet,
		Win:             ret > 0,
		SlippageWarning: warning,
	}, nil
}
```

- [ ] **Step 2: 创建 `backend/data/backtest/engine_test.go`**

```go
package backtest

import (
	"context"
	"testing"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Run(t *testing.T) {
	db.Init(":memory:?_journal_mode=WAL")
	defer func() {
		sqlDB, _ := db.Dao.DB()
		sqlDB.Close()
	}()

	store := datasource.NewKLineStore()
	ctx := context.Background()
	bars := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-02", Adjusted: true, Open: 100, High: 101, Low: 99, Close: 100, Volume: 1},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: true, Open: 100, High: 102, Low: 100, Close: 101, Volume: 1},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-04", Adjusted: true, Open: 101, High: 103, Low: 101, Close: 102, Volume: 1},
	}
	require.NoError(t, store.UpsertKLines(ctx, bars, 100))

	eng := NewEngine()
	res, err := eng.Run(ctx, Input{StockCode: "sh600519", SignalDate: "2024-01-02", HoldingDays: 2, Adjusted: true})
	require.NoError(t, err)
	assert.InDelta(t, 0.02, res.TotalReturn, 0.0001)
	assert.True(t, res.Win)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd /mnt/e/open-source/ai/go-stock && go test ./backend/data/backtest/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/backtest/engine.go backend/data/backtest/engine_test.go
git commit -m "feat(vip): add A-share backtest engine with T+1 and benchmark"
```

---

## Task 7: 实现批量回测与聚合

**Files:**
- Create: `backend/data/backtest/batch.go`
- Create: `backend/data/backtest/batch_test.go`

- [ ] **Step 1: 创建 `backend/data/backtest/batch.go`**

```go
package backtest

import (
	"context"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"sync"
)

// BatchInput groups recommendations to backtest.
type BatchInput struct {
	IDs         []uint
	StartDate   string
	EndDate     string
	HoldingDays int
	StopLoss    float64
	StopProfit  float64
	Adjusted    bool
}

// BatchResult aggregates backtest statistics.
type BatchResult struct {
	Total        int
	WinCount     int
	WinRate      float64
	AvgReturn    float64
	AvgDrawdown  float64
	AvgAlpha     float64
	ByRating     map[string]*RatingStats
}

type RatingStats struct {
	Count       int
	WinCount    int
	AvgReturn   float64
	AvgDrawdown float64
}

// Batcher runs backtests for multiple AI recommendations.
type Batcher struct {
	engine *Engine
}

func NewBatcher() *Batcher {
	return &Batcher{engine: NewEngine()}
}

func (b *Batcher) RunBatch(ctx context.Context, in BatchInput) (*BatchResult, error) {
	var recommends []models.AiRecommendStocks
	q := db.Dao.WithContext(ctx).Model(&models.AiRecommendStocks{})
	if len(in.IDs) > 0 {
		q = q.Where("id IN ?", in.IDs)
	}
	if in.StartDate != "" && in.EndDate != "" {
		q = q.Where("date(data_time) BETWEEN ? AND ?", in.StartDate, in.EndDate)
	}
	if err := q.Find(&recommends).Error; err != nil {
		return nil, err
	}

	type pair struct {
		rec models.AiRecommendStocks
		res *Result
		err error
	}
	ch := make(chan pair, len(recommends))
	var wg sync.WaitGroup
	for _, rec := range recommends {
		wg.Add(1)
		go func(r models.AiRecommendStocks) {
			defer wg.Done()
			res, err := b.engine.Run(ctx, Input{
				StockCode:    r.StockCode,
				SignalDate:   r.DataTime.Format("2006-01-02"),
				SignalRating: r.Rating,
				EntryPrice:   r.RecommendBuyPriceMax,
				HoldingDays:  in.HoldingDays,
				StopLoss:     in.StopLoss,
				StopProfit:   in.StopProfit,
				Adjusted:     in.Adjusted,
			})
			ch <- pair{rec: r, res: res, err: err}
		}(rec)
	}
	wg.Wait()
	close(ch)

	result := &BatchResult{ByRating: make(map[string]*RatingStats)}
	var totalReturn, totalDrawdown, totalAlpha float64
	for p := range ch {
		if p.err != nil {
			continue
		}
		result.Total++
		totalReturn += p.res.TotalReturn
		totalDrawdown += p.res.MaxDrawdown
		totalAlpha += p.res.Alpha
		if p.res.Win {
			result.WinCount++
		}

		rs, ok := result.ByRating[p.rec.Rating]
		if !ok {
			rs = &RatingStats{}
			result.ByRating[p.rec.Rating] = rs
		}
		rs.Count++
		rs.AvgReturn += p.res.TotalReturn
		rs.AvgDrawdown += p.res.MaxDrawdown
		if p.res.Win {
			rs.WinCount++
		}

		// Persist result
		db.Dao.WithContext(ctx).Create(&models.AiRecommendBacktest{
			AiRecommendID: p.rec.ID,
			StockCode:     p.rec.StockCode,
			StockName:     p.rec.StockName,
			SignalDate:    p.res.SignalDate,
			SignalRating:  p.rec.Rating,
			EntryPrice:    p.res.EntryPrice,
			ExitPrice:     p.res.ExitPrice,
			ExitDate:      p.res.ExitDate,
			HoldingDays:   p.res.HoldingDays,
			TotalReturn:   p.res.TotalReturn,
			MaxDrawdown:   p.res.MaxDrawdown,
			Csi300Return:  p.res.BenchmarkReturn,
			Alpha:         p.res.Alpha,
			Win:           p.res.Win,
			Source:        "cached",
		})
	}

	if result.Total > 0 {
		result.WinRate = float64(result.WinCount) / float64(result.Total)
		result.AvgReturn = totalReturn / float64(result.Total)
		result.AvgDrawdown = totalDrawdown / float64(result.Total)
		result.AvgAlpha = totalAlpha / float64(result.Total)
		for _, rs := range result.ByRating {
			rs.AvgReturn /= float64(rs.Count)
			rs.AvgDrawdown /= float64(rs.Count)
		}
	}
	return result, nil
}
```

- [ ] **Step 2: 创建 `backend/data/backtest/batch_test.go`**

```go
package backtest

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatcher_RunBatch(t *testing.T) {
	db.Init(":memory:?_journal_mode=WAL")
	defer func() {
		sqlDB, _ := db.Dao.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	now := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	rec := models.AiRecommendStocks{DataTime: &now, StockCode: "sh600519", StockName: "Moutai", Rating: "buy", RecommendBuyPriceMax: 0}
	db.Dao.Create(&rec)

	store := datasource.NewKLineStore()
	bars := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-02", Adjusted: true, Open: 100, High: 101, Low: 99, Close: 100, Volume: 1},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: true, Open: 100, High: 102, Low: 100, Close: 101, Volume: 1},
	}
	require.NoError(t, store.UpsertKLines(ctx, bars, 100))

	batcher := NewBatcher()
	res, err := batcher.RunBatch(ctx, BatchInput{HoldingDays: 1, Adjusted: true})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Total)
	assert.True(t, res.Win)
}
```

- [ ] **Step 3: 运行测试**

```bash
cd /mnt/e/open-source/ai/go-stock && go test ./backend/data/backtest/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/backtest/batch.go backend/data/backtest/batch_test.go
git commit -m "feat(vip): add batch backtest with aggregation and persistence"
```

---

## Task 8: 实现 Wails 服务绑定

**Files:**
- Create: `backend/data/backtest_service.go`
- Modify: `main.go:235-237`
- Modify: `app.go`

- [ ] **Step 1: 创建 `backend/data/backtest_service.go`**

```go
package data

import (
	"context"
	"go-stock/backend/data/backtest"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// BacktestService exposes VIP backtest and history sync APIs to Wails.
type BacktestService struct {
	engine  *backtest.Engine
	batcher *backtest.Batcher
	syncer  *history.Syncer
	store   *datasource.KLineStore
}

func NewBacktestService() *BacktestService {
	return &BacktestService{
		engine:  backtest.NewEngine(),
		batcher: backtest.NewBatcher(),
		syncer:  history.NewSyncer(),
		store:   datasource.NewKLineStore(),
	}
}

func (s *BacktestService) BacktestRecommend(ctx context.Context, recommendID uint, holdingDays int) (*backtest.Result, error) {
	var rec models.AiRecommendStocks
	if err := db.Dao.First(&rec, recommendID).Error; err != nil {
		return nil, err
	}
	return s.engine.Run(ctx, backtest.Input{
		StockCode:    rec.StockCode,
		SignalDate:   rec.DataTime.Format("2006-01-02"),
		SignalRating: rec.Rating,
		EntryPrice:   (rec.RecommendBuyPriceMin + rec.RecommendBuyPriceMax) / 2,
		HoldingDays:  holdingDays,
		Adjusted:     true,
	})
}

func (s *BacktestService) BacktestRecommendBatch(ctx context.Context, req backtest.BatchInput) (*backtest.BatchResult, error) {
	return s.batcher.RunBatch(ctx, req)
}

func (s *BacktestService) ListBacktestResults(ctx context.Context, page, pageSize int) (*models.AiRecommendBacktestPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	var list []models.AiRecommendBacktest
	var total int64
	db.Dao.Model(&models.AiRecommendBacktest{}).Count(&total)
	db.Dao.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&list)
	return &models.AiRecommendBacktestPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (s *BacktestService) StartHistoricalSync(ctx context.Context, years int) error {
	go func() {
		_ = s.syncer.Run(context.Background(), history.SyncConfig{Years: years})
	}()
	return nil
}

func (s *BacktestService) GetSyncProgress(ctx context.Context) (*history.SyncProgress, error) {
	return s.syncer.Progress(ctx)
}

func (s *BacktestService) SyncSingleStock(ctx context.Context, code string, years int) error {
	return s.syncer.Run(ctx, history.SyncConfig{Years: years, Codes: []string{code}})
}

func (s *BacktestService) GetKLineCacheStats(ctx context.Context) (map[string]any, error) {
	var total int64
	db.Dao.Model(&models.KLineBar{}).Count(&total)
	return map[string]any{"totalBars": total}, nil
}
```

- [ ] **Step 2: 添加分页响应模型到 `backend/models/kline_models.go`**

```go
// AiRecommendBacktestPageData 分页响应
type AiRecommendBacktestPageData struct {
	List       []AiRecommendBacktest `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
	TotalPages int                   `json:"totalPages"`
}
```

- [ ] **Step 3: 在 `app.go` 中注册服务**

在 `app.go` 中添加：

```go
func (a *App) BacktestService() *data.BacktestService {
	return data.NewBacktestService()
}
```

并在 `main.go` 的 `Bind` 中注册：

```go
		Bind: []interface{}{
			app,
			data.NewBacktestService(),
		},
```

- [ ] **Step 4: 运行 build 检查**

```bash
cd /mnt/e/open-source/ai/go-stock && go build ./backend/...
go vet ./backend/...
```

Expected: exit code 0

- [ ] **Step 5: Commit**

```bash
git add backend/data/backtest_service.go backend/models/kline_models.go main.go app.go
git commit -m "feat(vip): expose backtest and history sync via Wails service"
```

---

## Task 9: 前端页面（最小可用）

**Files:**
- Create: `frontend/src/components/BacktestPanel.vue`
- Create: `frontend/src/components/DataManager.vue`
- Modify: `frontend/src/router/` 或相关导航文件

- [ ] **Step 1: 创建 `BacktestPanel.vue` 骨架**

```vue
<template>
  <div>
    <h2>AI 推荐回测</h2>
    <n-button @click="runBatch">批量回测最近推荐</n-button>
    <pre v-if="result">{{ JSON.stringify(result, null, 2) }}</pre>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { BacktestRecommendBatch } from '../../wailsjs/go/data/BacktestService'

const result = ref(null)

async function runBatch() {
  result.value = await BacktestRecommendBatch({ holdingDays: 5, adjusted: true })
}
</script>
```

- [ ] **Step 2: 创建 `DataManager.vue` 骨架**

```vue
<template>
  <div>
    <h2>数据管理</h2>
    <n-button @click="startSync">启动历史数据同步（5 年）</n-button>
    <n-button @click="loadProgress">查询进度</n-button>
    <pre v-if="progress">{{ JSON.stringify(progress, null, 2) }}</pre>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { StartHistoricalSync, GetSyncProgress } from '../../wailsjs/go/data/BacktestService'

const progress = ref(null)

async function startSync() {
  await StartHistoricalSync(5)
}
async function loadProgress() {
  progress.value = await GetSyncProgress()
}
</script>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/BacktestPanel.vue frontend/src/components/DataManager.vue
git commit -m "feat(vip): add minimal backtest and data manager UI skeleton"
```

---

## Task 10: 全量构建与验证

- [ ] **Step 1: 运行 Go 全量测试**

```bash
cd /mnt/e/open-source/ai/go-stock && go test ./backend/... -v 2>&1 | tail -50
```

Expected: all PASS

- [ ] **Step 2: 运行前端构建**

```bash
cd /mnt/e/open-source/ai/go-stock/frontend && npm run build
```

Expected: exit code 0

- [ ] **Step 3: 运行 Wails build**

```bash
cd /mnt/e/open-source/ai/go-stock && wails build
```

Expected: exit code 0

- [ ] **Step 4: Commit**

```bash
git commit -m "chore(vip): full build verification for backtest and cache feature"
```

---

## 自审清单

### 1. Spec 覆盖

| 设计目标 | 对应 Task |
|---|---|
| AI 推荐回测 | Task 6, 7, 8 |
| 本地 SQLite K 线缓存 | Task 1, 2, 3 |
| A 股历史数据初始化 | Task 4, 5 |
| 增量更新 | Task 4 (`syncer.Run`) |
| 断点续传 | Task 4 (`KLineSyncLog`) |
| 多源回退 | Task 4 复用 `datasource.Router` |
| 多分析师信号强度分组 | 可在 Task 7 扩展（当前未实现，属可选增强） |

### 2. Placeholder 扫描

- 无 "TBD" / "TODO" / "implement later"
- 所有代码步骤包含完整代码
- 所有命令包含预期输出

### 3. 类型一致性

- `KLineBar` 字段与 `datasource.KLineBar` 转换函数 `BarsFromKLineData` 对齐
- `BacktestService` 使用 `backtest.Input` / `backtest.Result` / `backtest.BatchInput` / `backtest.BatchResult`
- `AiRecommendBacktestPageData` 已定义

### 4. 已知限制

- 回测引擎使用收盘价简化模型，未模拟 T+1 开盘缺口；设计文档允许此简化。
- 批量回测使用无限制 goroutine；大量推荐时应加 worker pool（后续优化）。
- 前端页面为最小骨架，需后续美化。

---

## 执行选项

Plan complete and saved to `docs/superpowers/plans/2026-06-25-vip-backtest-cache-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** - Dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** - Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
