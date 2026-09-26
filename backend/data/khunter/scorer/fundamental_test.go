package scorer

import "testing"

func TestFundamentalNormal(t *testing.T) {
	// yoy 40%→+20, ROE 18→+20, ocf/income 1.2→+20 → 50+60=110 → clamp 100
	d := ScoreFundamental(FundamentalInput{Available: true, HasYoy: true, NetProfitYoy: 40,
		HasROE: true, ROE: 18, HasOcf: true, OcfToIncome: 1.2})
	if d.Score != 100 || d.Veto {
		t.Fatalf("expect 100, got %+v", d)
	}
}

func TestFundamentalVeto(t *testing.T) {
	// 净利同比 -60% < -50% → 否决
	d := ScoreFundamental(FundamentalInput{Available: true, HasYoy: true, NetProfitYoy: -60})
	if !d.Veto || d.Score != -100 {
		t.Fatalf("应否决, got %+v", d)
	}
	// ROE -6% < -5% → 否决
	d = ScoreFundamental(FundamentalInput{Available: true, HasROE: true, ROE: -6})
	if !d.Veto {
		t.Fatalf("ROE<-5 应否决, got %+v", d)
	}
}

func TestFundamentalMissing(t *testing.T) {
	d := ScoreFundamental(FundamentalInput{Available: false})
	if !d.Degraded || d.Score != 50 {
		t.Fatalf("缺失应 degraded 基准 50, got %+v", d)
	}
	// 单项缺失计 0 分：只有 yoy=-10% → 50-20=30
	d = ScoreFundamental(FundamentalInput{Available: true, HasYoy: true, NetProfitYoy: -10})
	if d.Score != 30 || d.Veto {
		t.Fatalf("expect 30, got %+v", d)
	}
}
