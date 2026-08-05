package stockchange

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic,可暴露 service 对 port 的误用。
type stubRepo struct {
	repository.StockRepository
	saved         []stock.StockChangeHistory
	dedupSaved    []stock.StockChangeHistory
	saveErr       error
	dedupN        int
	dedupErr      error
	deletedCutoff string
	deleteErr     error
	lastQuery     stock.StockChangeHistoryQuery
	lastStartDate string
	lastTopN      int
	rankResult    *stock.ChangeRankResult
}

func (s *stubRepo) SaveStockChangesToHistory(ctx context.Context, changes []stock.StockChangeHistory) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, changes...)
	return nil
}

func (s *stubRepo) SaveStockChangesToHistoryWithDedup(ctx context.Context, changes []stock.StockChangeHistory) (int, error) {
	if s.dedupErr != nil {
		return 0, s.dedupErr
	}
	s.dedupSaved = append(s.dedupSaved, changes...)
	return s.dedupN, nil
}

func (s *stubRepo) DeleteStockChangeHistoryBefore(ctx context.Context, cutoffDate string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedCutoff = cutoffDate
	return nil
}

func (s *stubRepo) GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error) {
	s.lastQuery = query
	return stock.StockChangeHistoryPageData{Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *stubRepo) GetChangeRank(ctx context.Context, startDate string, topN int) (*stock.ChangeRankResult, error) {
	s.lastStartDate = startDate
	s.lastTopN = topN
	if s.rankResult != nil {
		return s.rankResult, nil
	}
	return &stock.ChangeRankResult{}, nil
}

func TestSaveStockChangesToHistory_EmptyItems(t *testing.T) {
	svc := NewService(&stubRepo{})
	if got := svc.SaveStockChangesToHistory(context.Background(), nil); got != "没有获取到异动数据" {
		t.Errorf("got %q, want %q", got, "没有获取到异动数据")
	}
}

func TestSaveStockChangesToHistory_SaveError(t *testing.T) {
	repo := &stubRepo{saveErr: errors.New("db down")}
	svc := NewService(repo)
	items := []stock.StockChangeItem{{Code: "sh600519"}}
	if got := svc.SaveStockChangesToHistory(context.Background(), items); got != "保存失败: db down" {
		t.Errorf("got %q, want %q", got, "保存失败: db down")
	}
}

func TestSaveStockChangesToHistory_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	items := []stock.StockChangeItem{
		{Time: "09:30:01", Code: "sh600519", Name: "贵州茅台", ChangeType: 4, TypeName: "涨停板", Industry: "白酒", Concept: "消费"},
		{Time: "09:30:02", Code: "sz000001", Name: "平安银行", ChangeType: 8, TypeName: "跌停板"},
	}
	got := svc.SaveStockChangesToHistory(context.Background(), items)
	if got != "成功保存 2 条异动数据" {
		t.Errorf("got %q, want %q", got, "成功保存 2 条异动数据")
	}
	if len(repo.saved) != 2 {
		t.Fatalf("repo saved %d items, want 2", len(repo.saved))
	}
	today := time.Now().Format("2006-01-02")
	for i, h := range repo.saved {
		if h.ChangeDate != today {
			t.Errorf("saved[%d].ChangeDate = %q, want today %q", i, h.ChangeDate, today)
		}
	}
	// 三列去重口径不带 Industry/Concept(与原 data.SaveStockChanges 一致)
	if repo.saved[0].Industry != "" || repo.saved[0].Concept != "" {
		t.Errorf("SaveStockChangesToHistory should not persist Industry/Concept, got %q/%q", repo.saved[0].Industry, repo.saved[0].Concept)
	}
	if repo.saved[0].ChangeTime != "09:30:01" || repo.saved[0].StockCode != "sh600519" || repo.saved[0].TypeName != "涨停板" {
		t.Errorf("field mapping mismatch: %+v", repo.saved[0])
	}
}

func TestSaveStockChangesWithDedup_KeepsDimensions(t *testing.T) {
	repo := &stubRepo{dedupN: 1}
	svc := NewService(repo)
	items := []stock.StockChangeItem{{Code: "sh600519", Industry: "白酒", Concept: "消费"}}
	n, err := svc.SaveStockChangesWithDedup(context.Background(), items)
	if err != nil || n != 1 {
		t.Fatalf("got (%d, %v), want (1, nil)", n, err)
	}
	if len(repo.dedupSaved) != 1 {
		t.Fatal("repo dedup save was not called")
	}
	if repo.dedupSaved[0].Industry != "白酒" || repo.dedupSaved[0].Concept != "消费" {
		t.Errorf("WithDedup should persist Industry/Concept, got %q/%q", repo.dedupSaved[0].Industry, repo.dedupSaved[0].Concept)
	}
	// 空 items 不触达 repo
	n, err = svc.SaveStockChangesWithDedup(context.Background(), nil)
	if err != nil || n != 0 || len(repo.dedupSaved) != 1 {
		t.Fatalf("empty items: got (%d, %v), repo calls %d", n, err, len(repo.dedupSaved))
	}
}

func TestDeleteStockChangeHistory(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	if got := svc.DeleteStockChangeHistory(context.Background(), 30); got != "已删除 30 天前的历史数据" {
		t.Errorf("got %q", got)
	}
	wantCutoff := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	if repo.deletedCutoff != wantCutoff {
		t.Errorf("cutoff = %q, want %q", repo.deletedCutoff, wantCutoff)
	}

	repoErr := &stubRepo{deleteErr: errors.New("db down")}
	if got := NewService(repoErr).DeleteStockChangeHistory(context.Background(), 30); got != "删除失败: db down" {
		t.Errorf("got %q", got)
	}
}

func TestGetStockChangeHistory_PageDefaultsToOne(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	page, err := svc.GetStockChangeHistory(context.Background(), stock.StockChangeHistoryQuery{Page: 0, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastQuery.Page != 1 {
		t.Errorf("repo got Page = %d, want 1", repo.lastQuery.Page)
	}
	if page.Page != 1 {
		t.Errorf("result Page = %d, want 1", page.Page)
	}
}

func TestGetChangeRank_TopNDefaultsTo20(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	if _, err := svc.GetChangeRank(context.Background(), 7, 0); err != nil {
		t.Fatal(err)
	}
	if repo.lastTopN != 20 {
		t.Errorf("topN = %d, want 20", repo.lastTopN)
	}
	wantStart := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	if repo.lastStartDate != wantStart {
		t.Errorf("startDate = %q, want %q", repo.lastStartDate, wantStart)
	}
}

func TestGetDailyDimensionStats_UnsupportedDimension(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.GetDailyDimensionStats(context.Background(), "bogus", "x", 7)
	if err == nil || !strings.Contains(err.Error(), "unsupported dimension: bogus") {
		t.Errorf("err = %v, want unsupported dimension error", err)
	}
}
