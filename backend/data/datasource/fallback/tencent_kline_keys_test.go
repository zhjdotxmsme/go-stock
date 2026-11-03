package fallback

import "testing"

func TestTencentKLineFieldKeys(t *testing.T) {
	qfq, raw := tencentKLineFieldKeys("week")
	if qfq != "qfqweek" || raw != "week" {
		t.Fatalf("week keys = %s/%s", qfq, raw)
	}
	qfq, raw = tencentKLineFieldKeys("month")
	if qfq != "qfqmonth" || raw != "month" {
		t.Fatalf("month keys = %s/%s", qfq, raw)
	}
	qfq, raw = tencentKLineFieldKeys("day")
	if qfq != "qfqday" || raw != "day" {
		t.Fatalf("day keys = %s/%s", qfq, raw)
	}
}
