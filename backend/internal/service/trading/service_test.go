package trading

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic,可暴露 service 对 port 的误用。
type stubRepo struct {
	repository.StockRepository
	records []stock.TradingRecord
	added   *stock.TradingRecord
	addErr  error
	countErr error
}

func (s *stubRepo) AddTradingRecord(ctx context.Context, record *stock.TradingRecord) error {
	if s.addErr != nil {
		return s.addErr
	}
	record.ID = 42
	s.added = record
	s.records = append(s.records, *record)
	return nil
}

func (s *stubRepo) ListAllTradingRecords(ctx context.Context) ([]stock.TradingRecord, error) {
	// 调用方保证有序;测试按时间升序构造
	return s.records, nil
}

func (s *stubRepo) CountBuyTradingRecords(ctx context.Context, stockCode string, since time.Time) (int64, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}
	var count int64
	for _, r := range s.records {
		if r.Direction != "买入" || !r.TradingTime.After(since) {
			continue
		}
		if stockCode != "" && r.StockCode != stockCode {
			continue
		}
		count++
	}
	return count, nil
}

func buyRecord(code string, price float64, volume int64, t time.Time) stock.TradingRecord {
	return stock.TradingRecord{
		StockCode:   code,
		StockName:   code,
		Direction:   "买入",
		Price:       price,
		Volume:      volume,
		TradingTime: t,
	}
}

func TestAddTradingRecord_CalculatesAmountAndDefaultsTime(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, nil)

	record := &stock.TradingRecord{StockCode: "sh600519", Direction: "买入", Price: 10, Volume: 100}
	id, err := svc.AddTradingRecord(context.Background(), record)
	if err != nil {
		t.Fatalf("AddTradingRecord() error = %v", err)
	}
	if id != 42 {
		t.Errorf("AddTradingRecord() id = %d, want 42", id)
	}
	if repo.added == nil {
		t.Fatal("repo.AddTradingRecord was not called")
	}
	if repo.added.Amount != 1000 {
		t.Errorf("Amount = %v, want 1000", repo.added.Amount)
	}
	if repo.added.TradingTime.IsZero() {
		t.Error("TradingTime should be defaulted to now")
	}
}

func TestAddTradingRecord_RejectsBuyWithin24h(t *testing.T) {
	repo := &stubRepo{
		records: []stock.TradingRecord{buyRecord("sh600519", 10, 100, time.Now().Add(-time.Hour))},
	}
	svc := NewService(repo, nil)

	record := &stock.TradingRecord{StockCode: "sh600519", Direction: "买入", Price: 11, Volume: 100}
	_, err := svc.AddTradingRecord(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "最近24小时内已对该股票进行过买入操作") {
		t.Errorf("AddTradingRecord() error = %v, want 24h rejection", err)
	}
	if repo.added != nil {
		t.Error("repo.AddTradingRecord should not be called when rejected")
	}
}

func TestAddTradingRecord_SellNotRestricted(t *testing.T) {
	repo := &stubRepo{
		records: []stock.TradingRecord{buyRecord("sh600519", 10, 100, time.Now().Add(-time.Hour))},
	}
	svc := NewService(repo, nil)

	record := &stock.TradingRecord{StockCode: "sh600519", Direction: "卖出", Price: 12, Volume: 50}
	if _, err := svc.AddTradingRecord(context.Background(), record); err != nil {
		t.Fatalf("AddTradingRecord(卖出) error = %v, want nil", err)
	}
}

func TestAddTradingRecord_RejectsWhen5BuysIn7Days(t *testing.T) {
	var records []stock.TradingRecord
	for i, code := range []string{"sh600001", "sh600002", "sh600003", "sh600004", "sh600005"} {
		records = append(records, buyRecord(code, 10, 100, time.Now().Add(-time.Duration(48+i)*time.Hour)))
	}
	repo := &stubRepo{records: records}
	svc := NewService(repo, nil)

	record := &stock.TradingRecord{StockCode: "sh600519", Direction: "买入", Price: 10, Volume: 100}
	_, err := svc.AddTradingRecord(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "最近7天内交易次数已达上限") {
		t.Errorf("AddTradingRecord() error = %v, want 7-day rejection", err)
	}
}

func TestCheckFrequentTrading(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		canTrade, msg := svc.CheckFrequentTrading(context.Background(), "sh600519")
		if !canTrade || msg != "可以交易" {
			t.Errorf("CheckFrequentTrading() = (%v, %q), want (true, \"可以交易\")", canTrade, msg)
		}
	})

	t.Run("count error defaults to allow", func(t *testing.T) {
		svc := NewService(&stubRepo{countErr: errors.New("db down")}, nil)
		canTrade, msg := svc.CheckFrequentTrading(context.Background(), "sh600519")
		if !canTrade || msg != "检查频繁交易失败，默认允许交易" {
			t.Errorf("CheckFrequentTrading() = (%v, %q), want (true, \"检查频繁交易失败，默认允许交易\")", canTrade, msg)
		}
	})
}

func TestGetTradingRecordStatistics(t *testing.T) {
	now := time.Now()
	records := []stock.TradingRecord{
		buyRecord("sh600519", 10, 100, now.Add(-72*time.Hour)),
		buyRecord("sh600519", 12, 100, now.Add(-48*time.Hour)),
		{
			StockCode:   "sh600519",
			Direction:   "卖出",
			Price:       15,
			Volume:      150,
			TradingTime: now.Add(-24 * time.Hour),
		},
	}

	t.Run("with realtime price", func(t *testing.T) {
		svc := NewService(&stubRepo{records: records}, func(code string) (float64, error) {
			if code != "sh600519" {
				t.Errorf("priceFn called with %q, want sh600519", code)
			}
			return 20, nil
		})
		stats, err := svc.GetTradingRecordStatistics(context.Background())
		if err != nil {
			t.Fatalf("GetTradingRecordStatistics() error = %v", err)
		}
		if stats.TotalBuyAmount != 2200 {
			t.Errorf("TotalBuyAmount = %v, want 2200", stats.TotalBuyAmount)
		}
		if stats.TotalSellAmount != 2250 {
			t.Errorf("TotalSellAmount = %v, want 2250", stats.TotalSellAmount)
		}
		// FIFO: 卖出150 消耗 100@10 + 50@12 = 1600; 剩余 50@12 = 600
		if stats.HoldingsAmount != 600 {
			t.Errorf("HoldingsAmount = %v, want 600", stats.HoldingsAmount)
		}
		if stats.CurrentValue != 1000 {
			t.Errorf("CurrentValue = %v, want 1000", stats.CurrentValue)
		}
		if stats.StockCount != 1 {
			t.Errorf("StockCount = %v, want 1", stats.StockCount)
		}
		// 2250-1600 + (1000-600) = 1050
		if stats.TotalProfit != 1050 {
			t.Errorf("TotalProfit = %v, want 1050", stats.TotalProfit)
		}
		// 1050/600*100 = 175
		if stats.ProfitRate != 175 {
			t.Errorf("ProfitRate = %v, want 175", stats.ProfitRate)
		}
	})

	t.Run("without realtime price", func(t *testing.T) {
		svc := NewService(&stubRepo{records: records}, func(code string) (float64, error) {
			return 0, nil
		})
		stats, err := svc.GetTradingRecordStatistics(context.Background())
		if err != nil {
			t.Fatalf("GetTradingRecordStatistics() error = %v", err)
		}
		if stats.CurrentValue != 0 {
			t.Errorf("CurrentValue = %v, want 0", stats.CurrentValue)
		}
		// 2250-1600 + (0-600) = 50
		if stats.TotalProfit != 50 {
			t.Errorf("TotalProfit = %v, want 50", stats.TotalProfit)
		}
		if math.Abs(stats.ProfitRate-50.0/600.0*100) > 1e-9 {
			t.Errorf("ProfitRate = %v, want %v", stats.ProfitRate, 50.0/600.0*100)
		}
	})

	t.Run("nil priceFn", func(t *testing.T) {
		svc := NewService(&stubRepo{records: records}, nil)
		stats, err := svc.GetTradingRecordStatistics(context.Background())
		if err != nil {
			t.Fatalf("GetTradingRecordStatistics() error = %v", err)
		}
		if stats.CurrentValue != 0 {
			t.Errorf("CurrentValue = %v, want 0", stats.CurrentValue)
		}
	})

	t.Run("empty records", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		stats, err := svc.GetTradingRecordStatistics(context.Background())
		if err != nil {
			t.Fatalf("GetTradingRecordStatistics() error = %v", err)
		}
		if *stats != (stock.TradingRecordStatistics{}) {
			t.Errorf("GetTradingRecordStatistics() = %+v, want all zero", *stats)
		}
	})
}

func TestNormalizeAPICode(t *testing.T) {
	cases := map[string]string{
		"600519.SH":        "sh600519",
		"000001.SZ":        "sz000001",
		"430047.BJ":        "bj430047",
		"sh600519 - 贵州茅台": "sh600519",
		// 注意:len==6 的规则优先于 0/3 前缀,裸 6 位代码一律补 sh(与 data 层行为一致)
		"600519": "sh600519",
		"300750": "sh300750",
	}
	for in, want := range cases {
		if got := normalizeAPICode(in); got != want {
			t.Errorf("normalizeAPICode(%q) = %q, want %q", in, got, want)
		}
	}
}
