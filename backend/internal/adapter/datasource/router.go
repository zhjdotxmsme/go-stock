// Package datasource implements the outbound data-source ports
// (backend/internal/port/datasource) by wrapping the existing backend/data
// APIs — same "wrap data + explicit mapping" pattern as
// adapter/repository/sqlite. No files are moved out of backend/data.
//
// Router（方案 §6.1）按 Priority 排序注册多个 provider，调用时依次尝试，
// 失败/无数据自动 fallback 到下一个数据源。默认装配见 composite.go，
// 优先级顺序对齐 data.FetchKLineWithFallback 的现有 fallback 链。
package datasource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	portds "go-stock/backend/internal/port/datasource"
	"go-stock/backend/stockcode"
)

// errNoData 数据源返回空结果（视为失败，触发路由 fallback）。
var errNoData = errors.New("no data")

// errConfigUnavailable 依赖运行期配置（db.Dao/SettingConfig）但未初始化
// （如单元测试环境），调用方按失败处理并 fallback。
var errConfigUnavailable = errors.New("datasource config unavailable (db not initialized)")

// Router 数据源路由器（方案 §6.1）：注册多个 provider，按 Priority 升序
// （数值小=优先级高）依次尝试，任一成功即返回；全部失败返回聚合错误。
// Register 应在启动装配期完成，运行期并发只读。
type Router struct {
	quoteProviders  []portds.QuoteProvider
	klineProviders  []portds.KLineProvider
	sectorProviders []portds.SectorProvider
}

// NewRouter 创建空路由器。
func NewRouter() *Router {
	return &Router{}
}

// Register 注册 provider；按其实现的端口接口分别加入对应调用链，
// 每条链内按 Priority 升序保持稳定排序。同一 provider 可实现多个端口。
func (r *Router) Register(providers ...portds.DataSourceProvider) {
	for _, p := range providers {
		if qp, ok := p.(portds.QuoteProvider); ok {
			r.quoteProviders = append(r.quoteProviders, qp)
		}
		if kp, ok := p.(portds.KLineProvider); ok {
			r.klineProviders = append(r.klineProviders, kp)
		}
		if sp, ok := p.(portds.SectorProvider); ok {
			r.sectorProviders = append(r.sectorProviders, sp)
		}
	}
	sort.SliceStable(r.quoteProviders, func(i, j int) bool {
		return r.quoteProviders[i].Priority() < r.quoteProviders[j].Priority()
	})
	sort.SliceStable(r.klineProviders, func(i, j int) bool {
		return r.klineProviders[i].Priority() < r.klineProviders[j].Priority()
	})
	sort.SliceStable(r.sectorProviders, func(i, j int) bool {
		return r.sectorProviders[i].Priority() < r.sectorProviders[j].Priority()
	})
}

// QuoteProviders 返回已排序的实时行情调用链（装配校验/测试用）。
func (r *Router) QuoteProviders() []portds.QuoteProvider { return r.quoteProviders }

// KLineProviders 返回已排序的 K 线调用链（装配校验/测试用）。
func (r *Router) KLineProviders() []portds.KLineProvider { return r.klineProviders }

// SectorProviders 返回已排序的板块调用链（装配校验/测试用）。
func (r *Router) SectorProviders() []portds.SectorProvider { return r.sectorProviders }

// GetQuote 按优先级依次尝试实时行情数据源；代码先归一化为内部标准格式。
// 返回错误或 nil（无数据）触发 fallback；Price=0 不视为失败
// （停牌股现价可能为 0，是否可用由调用方判断）。
func (r *Router) GetQuote(ctx context.Context, code string) (*portds.QuoteData, error) {
	code = stockcode.Normalize(code)
	if code == "" {
		return nil, fmt.Errorf("股票代码为空")
	}

	var errs []string
	for _, p := range r.quoteProviders {
		if !p.Available(ctx) {
			errs = append(errs, p.Name()+": unavailable")
			continue
		}
		q, err := p.GetQuote(ctx, code)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		if q == nil {
			errs = append(errs, p.Name()+": 无数据")
			continue
		}
		return q, nil
	}
	return nil, fmt.Errorf("所有实时行情数据源均失败(%s): %s", code, strings.Join(errs, "; "))
}

// GetKLine 按优先级依次尝试 K 线数据源；period 兼容 "day"/"week"/"month"
// 与东方财富 klt 数值码（"101"/"102"/"103"/分钟码），内部统一换算。
func (r *Router) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	code = stockcode.Normalize(code)
	if code == "" {
		return nil, fmt.Errorf("股票代码为空")
	}

	var errs []string
	for _, p := range r.klineProviders {
		if !p.Available(ctx) {
			errs = append(errs, p.Name()+": unavailable")
			continue
		}
		kd, err := p.GetKLine(ctx, code, period, count)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		if kd == nil || len(kd.Bars) == 0 {
			errs = append(errs, p.Name()+": 无数据")
			continue
		}
		return kd, nil
	}
	return nil, fmt.Errorf("所有K线数据源均失败(%s %s): %s", code, period, strings.Join(errs, "; "))
}

// GetSectorData 按优先级依次尝试板块数据源；代码先归一化为内部标准格式。
// 返回错误或 nil（无数据）触发 fallback；全部失败返回聚合错误。
func (r *Router) GetSectorData(ctx context.Context, code string) (*portds.SectorData, error) {
	code = stockcode.Normalize(code)
	if code == "" {
		return nil, fmt.Errorf("股票代码为空")
	}

	var errs []string
	for _, p := range r.sectorProviders {
		if !p.Available(ctx) {
			errs = append(errs, p.Name()+": unavailable")
			continue
		}
		sd, err := p.GetSectorData(ctx, code)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		if sd == nil {
			errs = append(errs, p.Name()+": 无数据")
			continue
		}
		return sd, nil
	}
	return nil, fmt.Errorf("所有板块数据源均失败(%s): %s", code, strings.Join(errs, "; "))
}

// normalizePeriod 将自然语言周期映射为东方财富 klt 数值码
// （data 包各 K 线 API 统一使用该编码）；已是数值码或未知值原样透传。
func normalizePeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "", "day", "daily", "d", "101":
		return "101"
	case "week", "weekly", "w", "102":
		return "102"
	case "month", "monthly", "m", "103":
		return "103"
	case "quarter", "q", "104":
		return "104"
	default:
		return period // 分钟码（"1"/"5"/"15"/"30"/"60"/"120"）等原样透传
	}
}
