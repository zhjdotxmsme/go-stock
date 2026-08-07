package datasource

import (
	"time"
)

// NewDefaultRouter 默认装配（方案 §6.1）：注册上述适配器并设置优先级。
// K 线 fallback 顺序对齐 data.FetchKLineWithFallback 的现有链：
//
//	tdx-mac(10) → eastmoney(20) → sina(30) → tencent(40) → tdx(50)
//
// （港美股在 sina/tencent/tdx 适配器内提前拒绝，与原链的提前返回一致。）
// 实时行情：tencent(10)——data 包内个股实时行情的唯一实现（qt.gtimg）。
//
// 装配函数位于 handler 可调用的位置：本包在 backend/internal 下，
// main 包不可直接 import，需经 handler 包中转（同 trading_handler 模式）。
func NewDefaultRouter() *Router {
	return NewRouter().withDefaults()
}

// withDefaults 注册默认数据源链（链式便于测试包裹）。
func (r *Router) withDefaults() *Router {
	r.Register(
		NewTdxMACKLineProvider(5*time.Second),
		NewEastMoneyKLineProvider(),
		NewSinaKLineProvider(),
		NewTencentKLineProvider(),
		NewTdxKLineProvider(),
		NewTencentQuoteProvider(),
	)
	return r
}
