package fallback

import (
	"context"
	"os"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

// 集成测试（真实东财接口，网络依赖）：锚定取数必须返回 endDate 之前的 K 线，
// 而不是"最新 N 根"——这是历史缺口回补的根因修复（旧实现补中段缺口永远拉到尾部数据）。
// 门控 EM_KLINE_IT=1：东财 kline 域限流时连接被掐断（CLAUDE.md 已知噪音），
// 环境不通时跳过；确定性覆盖见 TestFetchKLineNoPersistBefore_PrefersAnchored。
func TestFetchKLineNoPersistBefore_EMIntegration(t *testing.T) {
	if os.Getenv("EM_KLINE_IT") != "1" {
		t.Skip("set EM_KLINE_IT=1 to run against real eastmoney API")
	}
	router := datasource.GetRouter()
	RegisterKLineChain(router)
	endDate := time.Now().AddDate(-1, 0, -30).Format("2006-01-02") // 一年多前
	k, err := router.FetchKLineNoPersistBefore(context.Background(), "sz300780", "day", 60, endDate)
	if err != nil {
		t.Fatalf("FetchKLineNoPersistBefore: %v", err)
	}
	if k == nil || len(k.Bars) == 0 {
		t.Fatal("empty bars")
	}
	first, last := k.Bars[0].Time.Format("2006-01-02"), k.Bars[len(k.Bars)-1].Time.Format("2006-01-02")
	if last >= endDate {
		t.Fatalf("锚定语义破坏：最后一根 %s 应早于 endDate %s", last, endDate)
	}
	if first > endDate {
		t.Fatalf("首根 %s 晚于 endDate %s", first, endDate)
	}
	t.Logf("anchored fetch ok: %d bars [%s ~ %s], endDate=%s", len(k.Bars), first, last, endDate)
}
