package freestockdb

import (
	"context"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// Setup 预载因子/板块并注册 Provider；引擎不可用时不阻断启动。
func Setup(router *datasource.Router, cfg Config) *Manager {
	m := NewManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		logger.SugaredLogger.Warnf("freestockdb: %v（降级使用远程数据源）", err)
	}
	client := m.Client()
	factors := NewFactorStore()
	bi := NewBoardIndex()
	if m.Available(ctx) {
		if err := factors.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 复权因子加载失败: %v", err)
		}
		if err := bi.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 板块索引加载失败: %v", err)
		}
	}
	p := NewProvider(m, NewKLineService(client, factors), bi)
	router.RegisterKLineProvider(p)
	router.RegisterQuoteProvider(p)
	router.RegisterSectorProvider(p)
	return m
}
