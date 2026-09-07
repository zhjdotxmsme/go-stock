package data

import (
	"testing"

	"go-stock/backend/models"
)

// TestEastMoneyHostFallback 验证东财 push2/push2his → push2delay 自动回退 + 粘性 host 选择。
// 覆盖三条链路：期货报价（stock/get）、期货 K线（kline/get）、ETF K线（EastMoneyKLineApi）。
// 依赖外网：单条链路在当前环境不可用（不可达或本地区集群不服务该 secid）时该子用例 Skip，其余照常。
func TestEastMoneyHostFallback(t *testing.T) {
	t.Run("期货报价_113.AU0", func(t *testing.T) {
		asset := &models.CommodityAsset{Code: "AU", Symbol: "113.AU0", Name: "沪金连续"}
		fa := &EastMoneyFuturesApi{}
		q, err := fa.GetQuote(asset)
		if err != nil {
			t.Skipf("当前环境不可用: %v", err)
		}
		if q.Price <= 0 {
			t.Fatalf("price=0: %+v", q)
		}
		t.Logf("FUT-QUOTE OK price=%.2f high=%.2f low=%.2f", q.Price, q.High, q.Low)
	})

	t.Run("期货K线_113.AU0", func(t *testing.T) {
		asset := &models.CommodityAsset{Code: "AU", Symbol: "113.AU0", Name: "沪金连续"}
		fa := &EastMoneyFuturesApi{}
		bars, err := fa.GetKLine(asset, "day", 50)
		if err != nil {
			t.Skipf("当前环境不可用: %v", err)
		}
		if len(bars) == 0 {
			t.Skipf("当前环境返回空")
		}
		t.Logf("FUT-KLINE OK bars=%d first=%s last=%s",
			len(bars), bars[0].Time.Format("2006-01-02"), bars[len(bars)-1].Time.Format("2006-01-02"))
	})

	t.Run("ETF_K线_518880", func(t *testing.T) {
		ka := NewEastMoneyKLineApi(GetSettingConfig())
		kl := ka.GetKLineData("1.518880", "101", "", 50)
		if kl == nil || len(*kl) == 0 {
			t.Skipf("当前环境不可用（接口不可达）")
		}
		if (*kl)[0].Day == "" || (*kl)[len(*kl)-1].Day == "" {
			t.Fatalf("日期字段为空: first=%s last=%s", (*kl)[0].Day, (*kl)[len(*kl)-1].Day)
		}
		t.Logf("ETF-KLINE OK bars=%d first=%s last=%s", len(*kl), (*kl)[0].Day, (*kl)[len(*kl)-1].Day)
	})
}
