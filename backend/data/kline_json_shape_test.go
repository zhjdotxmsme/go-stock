package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

// TestKLineBarJSONShape 是前后端契约回归护：
// Wails 用 encoding/json 序列化绑定返回值，KLineBar 的 JSON 键名必须是小写
// （time/open/high/low/close/...），前端 CommodityKlineChart 依此读取。
// 若未来有人给 KLineBar 加了驼峰 json tag，此测试会失败，及时暴露契约漂移。
func TestKLineBarJSONShape(t *testing.T) {
	bar := datasource.KLineBar{
		Time:   time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		Open:   9000, High: 9100, Low: 8900, Close: 9050,
		Volume: 123456,
	}
	b, err := json.Marshal(bar)
	if err != nil {
		t.Fatalf("marshal KLineBar: %v", err)
	}
	s := string(b)
	for _, key := range []string{`"time"`, `"open"`, `"high"`, `"low"`, `"close"`, `"volume"`} {
		if !strings.Contains(s, key) {
			t.Errorf("KLineBar JSON 缺少小写键 %s，实际=%s（前端按小写读取，契约漂移）", key, s)
		}
	}
	// 不应出现驼峰键（防止被误改成驼峰）
	for _, key := range []string{`"Time"`, `"Open"`, `"Close"`, `"Volume"`} {
		if strings.Contains(s, key) {
			t.Errorf("KLineBar JSON 出现驼峰键 %s，实际=%s，与前端契约冲突", key, s)
		}
	}
}
