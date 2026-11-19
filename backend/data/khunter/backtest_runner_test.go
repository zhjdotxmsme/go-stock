package khunter

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// ---------- 四组共享输入与时间切分（brief Step 1 原文测试） ----------

func TestBacktestVariantsShareUniverseAndDates(t *testing.T) {
	req := BacktestRequest{
		StartDate: "2026-01-01", EndDate: "2026-12-31", InitialCash: 1e6,
		Variants: []BacktestVariant{VariantBaseline, VariantDataOnly, VariantRiskOnly, VariantFull},
	}
	normalized, err := NormalizeBacktestRequest(req)
	if err != nil || len(normalized.Variants) != 4 {
		t.Fatalf("四组配置错误: %+v err=%v", normalized, err)
	}
	manifest := BacktestInputManifest{Codes: []string{"600000", "300780"}, StartDate: req.StartDate, EndDate: req.EndDate, CostHash: "same"}
	for _, variant := range normalized.Variants {
		if got := BuildVariantInput(variant, manifest); got.CostHash != "same" || len(got.Codes) != 2 {
			t.Fatalf("%s 改变了输入: %+v", variant, got)
		}
	}
}

func TestSplitsAreChronologicalAndKeepExitData(t *testing.T) {
	days := []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-12", "2026-01-13", "2026-01-14", "2026-01-15", "2026-01-16"}
	splits := SplitTradingDays(days, 120, 8)
	if len(splits) != 3 || splits[0].SignalEnd >= splits[1].SignalStart || splits[1].SignalEnd >= splits[2].SignalStart {
		t.Fatalf("时间切分交叉: %+v", splits)
	}
	if splits[2].ExitDataEnd != days[len(days)-1] {
		t.Fatalf("样本外缺少退出数据: %+v", splits[2])
	}
}

// ---------- 请求归一化、清单边界、切分细节 ----------

func TestNormalizeBacktestRequestDefaultsAndValidation(t *testing.T) {
	norm, err := NormalizeBacktestRequest(BacktestRequest{StartDate: "2026-01-01", EndDate: "2026-06-30"})
	if err != nil {
		t.Fatalf("默认归一化失败: %v", err)
	}
	if len(norm.Variants) != 4 ||
		norm.Variants[0] != VariantBaseline || norm.Variants[1] != VariantDataOnly ||
		norm.Variants[2] != VariantRiskOnly || norm.Variants[3] != VariantFull {
		t.Fatalf("空变体应默认四组: %+v", norm.Variants)
	}
	if norm.InitialCash != DefaultRiskControlConfig().InitialCash {
		t.Fatalf("初始资金应回退默认: %v", norm.InitialCash)
	}
	if _, err := NormalizeBacktestRequest(BacktestRequest{EndDate: "2026-01-01"}); err == nil {
		t.Fatal("缺开始日期应报错")
	}
	if _, err := NormalizeBacktestRequest(BacktestRequest{StartDate: "2026-01-02", EndDate: "2026-01-01"}); err == nil {
		t.Fatal("开始晚于结束应报错")
	}
	if _, err := NormalizeBacktestRequest(BacktestRequest{StartDate: "2026/01/01", EndDate: "2026-12-31"}); err == nil {
		t.Fatal("非法日期格式应报错")
	}
	if _, err := NormalizeBacktestRequest(BacktestRequest{StartDate: "2026-01-01", EndDate: "2026-12-31", Variants: []BacktestVariant{"nope"}}); err == nil {
		t.Fatal("未知变体应报错")
	}
}

func TestBuildVariantInputDeepCopiesAndMapsToggles(t *testing.T) {
	manifest := BacktestInputManifest{
		Codes: []string{"sh600000", "sz000001"}, StartDate: "2026-01-01", EndDate: "2026-06-30",
		CostHash: "hash-1", InitialCash: 1e6,
	}
	// 消融递进：baseline(无) ⊂ data_only(数据修复) ⊂ risk(数据修复+风险过滤) ⊂ full(+确认退出)
	want := map[BacktestVariant][3]bool{
		VariantBaseline: {false, false, false},
		VariantDataOnly: {true, false, false},
		VariantRiskOnly: {true, true, false},
		VariantFull:     {true, true, true},
	}
	for variant, toggles := range want {
		got := BuildVariantInput(variant, manifest)
		if got.UseDataFix != toggles[0] || got.UseRiskFilter != toggles[1] || got.UseConfirmExit != toggles[2] {
			t.Fatalf("%s 开关错误: %+v", variant, got)
		}
		if got.CostHash != "hash-1" || got.StartDate != manifest.StartDate || got.EndDate != manifest.EndDate {
			t.Fatalf("%s 改变了共享输入: %+v", variant, got)
		}
		// 深拷贝：改动返回值不得影响共享清单
		if len(got.Codes) > 0 {
			got.Codes[0] = "MUTATED"
		}
	}
	if manifest.Codes[0] == "MUTATED" {
		t.Fatal("BuildVariantInput 必须深拷贝 Codes")
	}
}

func TestSplitTradingDaysClampsAndPhases(t *testing.T) {
	if got := SplitTradingDays(nil, 120, 8); got != nil {
		t.Fatalf("空序列应返回空: %+v", got)
	}
	days := []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-12", "2026-01-13", "2026-01-14", "2026-01-15", "2026-01-16"}
	splits := SplitTradingDays(days, 120, 8)
	if len(splits) != 3 {
		t.Fatalf("应切成三段: %+v", splits)
	}
	phases := []string{PhaseTrain, PhaseValidate, PhaseSample}
	for i, s := range splits {
		if s.Phase != phases[i] {
			t.Fatalf("阶段错误: %+v", s)
		}
		if s.WarmupStart > s.SignalStart || s.SignalStart > s.SignalEnd || s.SignalEnd > s.ExitDataEnd {
			t.Fatalf("区间单调性错误: %+v", s)
		}
	}
	// 预热向前（更早）、退出数据向后（更晚）：中段两端都应越出信号区间
	if splits[1].WarmupStart >= splits[1].SignalStart || splits[1].ExitDataEnd <= splits[1].SignalEnd {
		t.Fatalf("中段未保留预热/退出数据: %+v", splits[1])
	}
	// 末段退出数据被截到最后一个交易日
	if splits[2].ExitDataEnd != days[len(days)-1] {
		t.Fatalf("末段退出数据越界: %+v", splits[2])
	}
}

// ---------- 数据种子 ----------

func bizDates(start string, n int) []string {
	d, err := time.Parse("2006-01-02", start)
	if err != nil {
		panic(err)
	}
	out := make([]string, 0, n)
	for len(out) < n {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d.Format("2006-01-02"))
		}
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func idxOf(dates []string, d string) int {
	for i, x := range dates {
		if x == d {
			return i
		}
	}
	return -1
}

type barOverride struct {
	idx        int
	o, h, l, c float64
}

// runnerBars 生成一段平滑上升的种子日 K；overrides 覆盖指定 bar 的开高低收
// （0 值字段沿用推导值）；invalidDates 的 bar 缺失量/额单位（数据闸门判无效）。
func runnerBars(code string, dates []string, start, step float64, overrides []barOverride, invalidDates map[string]bool) []models.KLineBar {
	closes := make([]float64, len(dates))
	for i := range dates {
		closes[i] = start + step*float64(i)
	}
	ovByIdx := map[int]barOverride{}
	for _, ov := range overrides {
		if ov.c != 0 {
			closes[ov.idx] = ov.c
		}
		ovByIdx[ov.idx] = ov
	}
	bars := make([]models.KLineBar, 0, len(dates))
	prev := start - step
	for i, d := range dates {
		c := closes[i]
		ov := ovByIdx[i]
		o := prev
		if ov.o != 0 {
			o = ov.o
		}
		h := math.Max(o, c) * 1.002
		if ov.h != 0 {
			h = ov.h
		}
		l := math.Min(o, c) * 0.998
		if ov.l != 0 {
			l = ov.l
		}
		bar := models.KLineBar{
			StockCode: code, Period: "day", TradeDate: d,
			Open: o, High: h, Low: l, Close: c, PrevClose: prev,
			Volume: 10000000, VolumeUnit: "share",
			Amount: c * 10000000, AmountUnit: "CNY", Source: "seed",
		}
		if invalidDates[d] {
			bar.VolumeUnit = ""
			bar.AmountUnit = ""
		}
		bars = append(bars, bar)
		prev = c
	}
	return bars
}

func seedBarsRawAndAdjusted(t *testing.T, bars []models.KLineBar) {
	t.Helper()
	rows := make([]models.KLineBar, 0, len(bars)*2)
	for _, b := range bars {
		rows = append(rows, b)
		adj := b
		adj.Adjusted = true
		rows = append(rows, adj)
	}
	if err := db.Dao.Create(&rows).Error; err != nil {
		t.Fatalf("种子日K失败: %v", err)
	}
}

func seedBenchBars(t *testing.T, dates []string) []models.KLineBar {
	t.Helper()
	bars := make([]models.KLineBar, 0, len(dates))
	for i, d := range dates {
		c := float64(4000 + i)
		bars = append(bars, models.KLineBar{
			StockCode: PortfolioBenchmarkCode, Period: "day", TradeDate: d,
			Open: c, High: c, Low: c, Close: c, PrevClose: c - 1,
			Volume: 1000000, VolumeUnit: "share", Amount: c * 1000000, AmountUnit: "CNY", Source: "seed",
		})
	}
	if err := db.Dao.Create(&bars).Error; err != nil {
		t.Fatalf("种子基准日K失败: %v", err)
	}
	return bars
}

func seedResearchRecord(t *testing.T, date string, hunted []models.KhunterHunting, notBacktestable []string) {
	t.Helper()
	payload := ResearchRecordResult{
		Result:          &PipelineResult{TradeDate: date, Hunted: len(hunted)},
		Hunted:          hunted,
		NotBacktestable: notBacktestable,
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("研究记录序列化失败: %v", err)
	}
	row := &models.KhunterResearchRecord{TradeDate: date, Status: "success", ResultJSON: string(blob)}
	if err := db.Dao.Create(row).Error; err != nil {
		t.Fatalf("种子研究记录失败: %v", err)
	}
}

func huntedRow(code, name, date string, score, support float64) models.KhunterHunting {
	return models.KhunterHunting{
		Code: code, Name: name, EnterDate: date, EnterScore: score,
		SupportPrice: support, Status: "追踪中",
	}
}

// seedFourCandidateScenario 四个候选：A 正常可确认、B 窗口末尾信号、C 近5日过热、
// D 信号日数据无效（缺量额单位）。返回全市场窗口交易日与个股 K 线索引。
func seedFourCandidateScenario(t *testing.T) (windowDays []string) {
	t.Helper()
	windowDays = bizDates("2026-01-05", 15)
	allDays := bizDates("2025-10-26", 80)
	iSignal := idxOf(allDays, "2026-01-08")  // 信号日（训练段）
	iConfirm := idxOf(allDays, "2026-01-09") // 确认日
	iExit := idxOf(allDays, "2026-01-12")    // 退出日
	iRun := iSignal - 5                      // C 过热起点前一根

	// A sh600000：信号日长上影冲高，次日反包触及触发价，第三日回落退出
	//（l 沿用推导值 min(o,c)×0.998：显式覆盖 low>open 会触发数据闸门 OHLC 异常）
	a := runnerBars("sh600000", allDays, 10, 0.1, []barOverride{
		{idx: iSignal, c: trendClose(iSignal-1) * 1.03, h: trendClose(iSignal-1) * 1.10},
		{idx: iConfirm, h: trendClose(iSignal-1) * 1.111, c: trendClose(iSignal-1) * 1.03 * 0.99},
		{idx: iExit, c: trendClose(iConfirm) * 0.9},
	}, nil)
	seedBarsRawAndAdjusted(t, a)

	// B sz000001：平滑趋势，信号在窗口最后一个交易日（确认数据越界）
	b := runnerBars("sz000001", allDays, 8, 0.05, nil, nil)
	seedBarsRawAndAdjusted(t, b)

	// C sh600519：近5日 +7%/日 过热（>20%）
	runs := []barOverride{}
	for k := 1; k <= 5; k++ {
		runs = append(runs, barOverride{idx: iRun + k, c: trendClose(iRun) * math.Pow(1.07, float64(k))})
	}
	c := runnerBars("sh600519", allDays, 10, 0.1, runs, nil)
	seedBarsRawAndAdjusted(t, c)

	// D sz000002：信号日 K 缺量/额单位（数据闸门判无效）
	d := runnerBars("sz000002", allDays, 5, 0.05, nil, map[string]bool{"2026-01-08": true})
	seedBarsRawAndAdjusted(t, d)

	seedBenchBars(t, windowDays)
	seedResearchRecord(t, "2026-01-08", []models.KhunterHunting{
		huntedRow("sh600000", "A股一", "2026-01-08", 74.5, 10.5),
		huntedRow("sh600519", "A股三", "2026-01-08", 80, 11.0),
		huntedRow("sz000002", "A股四", "2026-01-08", 70, 5.2),
	}, nil)
	seedResearchRecord(t, "2026-01-23", []models.KhunterHunting{
		huntedRow("sz000001", "A股二", "2026-01-23", 66, 8.0),
	}, nil)
	return windowDays
}

// trendClose 与 runnerBars(10, 0.1) 的推导收盘一致，用于构造覆盖价格。
func trendClose(idx int) float64 {
	return 10 + 0.1*float64(idx)
}

// ---------- 全链路回测：守恒、共享输入、变体差异 ----------

func TestRunFullBacktestConservesSamplesAndSharesInputs(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	seedFourCandidateScenario(t)

	res, err := RunFullBacktest(context.Background(), BacktestRequest{
		StartDate: "2026-01-05", EndDate: "2026-01-23",
		Codes:       []string{"sh600000", "sz000001", "sh600519", "sz000002"},
		InitialCash: 1e6,
	})
	if err != nil {
		t.Fatalf("全链路回测失败: %v", err)
	}
	if len(res.Variants) != 4 {
		t.Fatalf("应输出四组: %d", len(res.Variants))
	}
	if len(res.Splits) != 3 || res.Splits[2].Phase != PhaseSample {
		t.Fatalf("时间切分错误: %+v", res.Splits)
	}

	// 四组共享输入：代码、日期、成本哈希一致
	first := res.Variants[0].Manifest
	for _, v := range res.Variants[1:] {
		if v.Manifest.CostHash != first.CostHash || v.Manifest.StartDate != first.StartDate ||
			v.Manifest.EndDate != first.EndDate || strings.Join(v.Manifest.Codes, ",") != strings.Join(first.Codes, ",") {
			t.Fatalf("四组输入不一致: %+v vs %+v", v.Manifest, first)
		}
	}
	if len(res.Variants[0].Manifest.Codes) != 4 {
		t.Fatalf("清单应含 4 只: %+v", first.Codes)
	}

	// 按变体的预期样本结局（见报告：守恒式 信号=确认+过期+无效+不可成交+仍在窗口）
	// A 正常可成交；B 信号在最后一个交易日（末日入场不可成交）；C 过热被风险否决；
	// D 信号日缺量额单位（data_only/full 数据闸门判无效；risk 叠加风险过滤后同样无效）。
	// confirmed 为组合引擎实际成交数：baseline 无闸门 A/C/D 成交；data_only D 无效；
	// risk C 风险否决过期、D 数据无效；full A 确认成交、C 过期、D 无效、B 窗口未闭合。
	expect := map[BacktestVariant][5]int{ // confirmed, expired, invalid, untradeable, stillInWindow
		VariantBaseline: {3, 0, 0, 1, 0},
		VariantDataOnly: {2, 0, 1, 1, 0},
		VariantRiskOnly: {1, 1, 1, 1, 0},
		VariantFull:     {1, 1, 1, 0, 1},
	}
	for _, v := range res.Variants {
		want := expect[v.Variant]
		r := v.Report
		got := [5]int{r.ConfirmedCount, r.ExpiredCount, r.InvalidCount, r.UntradeableCount, r.StillInWindowCount}
		if got != want {
			t.Fatalf("%s 结局不符: got=%v want=%v (signal=%d trades=%+v)", v.Variant, got, want, r.SignalCount, r.Trades)
		}
		if r.SignalCount != 4 {
			t.Fatalf("%s 信号数=%d, want=4", v.Variant, r.SignalCount)
		}
		if r.SignalCount != r.ConfirmedCount+r.ExpiredCount+r.InvalidCount+r.UntradeableCount+r.StillInWindowCount {
			t.Fatalf("%s 样本计数不守恒: %+v", v.Variant, r)
		}
		// 样本外确认交易 < 100 → 明确标记样本不足
		if !v.SampleInsufficient {
			t.Fatalf("%s 样本外确认 %d 笔应标记样本不足", v.Variant, v.OOSConfirmed)
		}
		// 幸存者偏差说明必须写入报告
		joined := strings.Join(r.Limitations, "\n")
		if !strings.Contains(joined, "幸存者偏差") {
			t.Fatalf("%s 缺少幸存者偏差说明: %v", v.Variant, r.Limitations)
		}
		if v.Unfinished != (r.StillInWindowCount > 0) {
			t.Fatalf("%s 未完成标记与仍在窗口数不一致: %+v", v.Variant, r)
		}
	}
	// full 因末段信号确认数据越界而未完成；其余三组不受确认窗口影响
	if !res.Variants[3].Report.Unfinished {
		t.Fatal("full 末段信号确认数据越界应标记未完成")
	}
	if res.Variants[0].Report.Unfinished {
		t.Fatal("baseline 无仍在窗口记录不应标记未完成")
	}
	// 案例轨迹：应包含观察/确认/入场/退出阶段事件
	if len(res.CaseReplays) == 0 {
		t.Fatal("缺少案例回放轨迹")
	}
}

func TestRunFullBacktestSnapshotFlags(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	windowDays := bizDates("2026-01-05", 15)
	allDays := bizDates("2025-10-26", 80)
	seedBarsRawAndAdjusted(t, runnerBars("sh600036", allDays, 10, 0.1, nil, nil))
	seedBenchBars(t, windowDays)
	seedResearchRecord(t, "2026-01-08", []models.KhunterHunting{
		huntedRow("sh600036", "缺快照股", "2026-01-08", 75, 10),
	}, []string{"sh600036"})

	res, err := RunFullBacktest(context.Background(), BacktestRequest{
		StartDate: "2026-01-05", EndDate: "2026-01-23", Codes: []string{"sh600036"}, InitialCash: 1e6,
	})
	if err != nil {
		t.Fatalf("回测失败: %v", err)
	}
	byVariant := map[BacktestVariant]VariantBacktestResult{}
	for _, v := range res.Variants {
		byVariant[v.Variant] = v
		r := v.Report
		if r.SignalCount != 1 {
			t.Fatalf("%s 信号数=%d, want=1", v.Variant, r.SignalCount)
		}
		if r.SignalCount != r.ConfirmedCount+r.ExpiredCount+r.InvalidCount+r.UntradeableCount+r.StillInWindowCount {
			t.Fatalf("%s 样本计数不守恒: %+v", v.Variant, r)
		}
		if !strings.Contains(strings.Join(r.Limitations, "\n"), "sh600036") {
			t.Fatalf("%s 报告应写明缺快照标的: %v", v.Variant, r.Limitations)
		}
	}
	// 缺快照候选按研究记录原样回放（不得用最新快照补数），但必须标记可信度：
	// baseline = 近似基线；full = 完整五维不可回测
	if !byVariant[VariantBaseline].Manifest.Approximate {
		t.Fatal("baseline 缺快照应标记近似基线")
	}
	if !byVariant[VariantFull].Manifest.NotBacktestable {
		t.Fatal("full 缺历史快照应标记不可回测")
	}
	if len(byVariant[VariantFull].NotBacktestableCodes) != 1 || byVariant[VariantFull].NotBacktestableCodes[0] != "sh600036" {
		t.Fatalf("full 未列出不可回测标的: %+v", byVariant[VariantFull].NotBacktestableCodes)
	}
	if byVariant[VariantDataOnly].Manifest.NotBacktestable || byVariant[VariantDataOnly].Manifest.Approximate {
		t.Fatalf("data_only 不应继承近似/不可回测标记: %+v", byVariant[VariantDataOnly].Manifest)
	}
	// 近似基线仍按信号日收盘入场成交（不因标记而丢样本）
	if byVariant[VariantBaseline].Report.TradeCount != 1 {
		t.Fatalf("baseline 应有 1 笔成交: %+v", byVariant[VariantBaseline].Report)
	}
}

// ---------- 点入时点研究重放（integration：研究记录缺失时用生产研究流水线重算） ----------

func TestRunFullBacktestRegeneratesResearchPointInTime(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}, &models.AllStockInfo{}); err != nil {
		t.Fatal(err)
	}
	windowDays := bizDates("2026-01-05", 15)
	allDays := bizDates("2025-10-26", 80)
	iSignal := idxOf(allDays, "2026-01-08")

	// 仙人指路形态：T-3 冲高长上影放量，T 日反包确认
	bar := runnerBars("sh600000", allDays, 10, 0.1, []barOverride{
		{idx: iSignal - 3, o: trendClose(iSignal - 4), c: trendClose(iSignal-4) * 1.03, h: trendClose(iSignal-4) * 1.10, l: trendClose(iSignal-4) * 1.01},
		{idx: iSignal, c: trendClose(iSignal-4) * 1.086},
	}, nil)
	// 信号日成交量放大 2.2 倍
	for i := range bar {
		if i == iSignal-3 {
			bar[i].Volume = 22000000
			bar[i].Amount = bar[i].Close * 22000000
		}
	}
	seedBarsRawAndAdjusted(t, bar)
	seedBenchBars(t, windowDays)
	// 资金流 5 日主力净流入 > 1 亿、大单持续流入且小单流出（资金面满分，总分越过狩猎阈值）
	flows := []models.KhunterMoneyFlowDaily{}
	for _, d := range allDays[iSignal-6 : iSignal] {
		flows = append(flows, models.KhunterMoneyFlowDaily{
			Code: "sh600000", Date: d, MainNet: 3e7, LgNetRatio: 10, SmNetRatio: -2,
		})
	}
	if err := db.Dao.Create(&flows).Error; err != nil {
		t.Fatal(err)
	}
	stock := models.AllStockInfo{SECUCODE: "600000.SH", SECURITYNAMEABBR: "正常股"}
	if err := db.Dao.Create(&stock).Error; err != nil {
		t.Fatal(err)
	}

	res, err := RunFullBacktest(context.Background(), BacktestRequest{
		StartDate: "2026-01-05", EndDate: "2026-01-23", Codes: []string{"sh600000"}, InitialCash: 1e6,
	})
	if err != nil {
		t.Fatalf("回测失败: %v", err)
	}
	byVariant := map[BacktestVariant]VariantBacktestResult{}
	for _, v := range res.Variants {
		byVariant[v.Variant] = v
	}
	// 研究重放捕获信号：完整方案确认并成交；缺基本面/板块历史快照 → 近似/不可回测
	if byVariant[VariantFull].Report.SignalCount != 1 {
		t.Fatalf("研究重放未捕获候选: %+v", byVariant[VariantFull].Report)
	}
	if byVariant[VariantFull].Report.TradeCount != 1 {
		t.Fatalf("完整方案未确认成交: %+v", byVariant[VariantFull].Report)
	}
	if !byVariant[VariantFull].Manifest.NotBacktestable || len(byVariant[VariantFull].NotBacktestableCodes) != 1 {
		t.Fatalf("缺历史快照的完整五维结果应标不可回测: %+v", byVariant[VariantFull])
	}
	if !byVariant[VariantBaseline].Manifest.Approximate {
		t.Fatal("baseline 缺快照应标近似基线")
	}
	// 研究重放不得写生产表
	for _, m := range []any{&models.KhunterHunting{}, &models.KhunterObservation{}} {
		var n int64
		if err := db.Dao.Model(m).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("研究重放写入生产表 %T: %d 行", m, n)
		}
	}
	// 案例轨迹应含观察与确认事件
	found := map[string]bool{}
	for _, cr := range res.CaseReplays {
		for _, ev := range cr.Events {
			found[ev.Stage] = true
		}
	}
	if !found["observation"] || !found["confirm"] {
		t.Fatalf("案例轨迹缺少观察/确认事件: %v", found)
	}
}
