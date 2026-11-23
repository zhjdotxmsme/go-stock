package khunter

import (
	"os"
	"testing"

	"go-stock/backend/db"
)

// 手动验证（默认 Skip）：对真实库副本跑一轮复盘，再抽查已回填记录，
// 与 daily_picks 中同 (code, 日期) 的窗口收盘价比对（收益因基准不同不比）。
// 运行：KHUNTER_VERIFY_DB=/path/copy.db go test ./backend/data/khunter/ -run TestVerifyReviewAgainstDB -v
func TestVerifyReviewAgainstDB(t *testing.T) {
	path := os.Getenv("KHUNTER_VERIFY_DB")
	if path == "" {
		t.Skip("set KHUNTER_VERIFY_DB to a DB copy to run manual verification")
	}
	db.Init(path)
	if err := EnsureMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	res, err := NewReview().RunDailyReview(t.Context())
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	t.Logf("review result: %+v", res)
	if res.HuntingReviewed+res.HuntingPartial == 0 && res.PredJudged == 0 {
		t.Log("nothing to backfill in this copy; pick a DB with unreviewed holdings/predictions")
	}
}
