package data

import "testing"

// TestTdxTurnoverFilter 回归：指数 FloatShares 垃圾值算出的天文换手率（>1000%）不得进入 K 线数据。
func TestTdxTurnoverFilter(t *testing.T) {
	if !tdxValidTurnover(0.3421) {
		t.Errorf("正常换手率 0.34%% 应被采纳")
	}
	if tdxValidTurnover(6.7e38) {
		t.Errorf("天文换手率 6.7e+38 应被过滤")
	}
	if tdxValidTurnover(-1) {
		t.Errorf("负换手率应被过滤")
	}
	if tdxValidTurnover(0) {
		t.Errorf("零换手率应被过滤")
	}
}
