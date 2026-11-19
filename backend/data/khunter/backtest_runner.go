// 全链路四组对照回测（Task 10）。复用 Task 1～9 生产规则：
// 研究流水线（runResearchPipeline 点入时点重放）、数据闸门（EvaluateDataGate）、
// 风险过滤（BuildRiskMetrics/EvaluateRisk/AttachSignalRisk）、确认引擎单日规则
// （ConfirmationWindowState/EvaluateConfirmationDay/RawSignalLevel/RiskBudgetShares）、
// 退出状态机（EvaluateExitDay，经组合回测引擎回放）与组合引擎（RunPortfolioBacktest）。
// 不复制任何简化形态逻辑，也不把旧信号级回测（backtest_adapter.go）的结果混入四组对照。
//
// 四组共享同一份输入清单、日期与成本假设（BacktestInputManifest 是显式边界）：
//
//	baseline  信号日收盘入场、研究支撑位失效，无数据闸门/风险过滤/确认窗口；
//	data_only 只启用数据闸门（Task 3）；
//	risk      数据闸门 + 风险过滤（Task 3+6，消融递进在 data_only 之上增加风险层）；
//	full      数据闸门 + 风险过滤 + T+1~T+3 确认入场（Task 7）+ 退出引擎（Task 8）。
//
// 信号/评分输入全部来自时点研究记录；记录缺失时用生产研究流水线重放（只写研究记录表）。
package khunter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
)

// BacktestVariant 四组对照的变体标识。
type BacktestVariant string

const (
	VariantBaseline BacktestVariant = "baseline"
	VariantDataOnly BacktestVariant = "data_only"
	VariantRiskOnly BacktestVariant = "risk"
	VariantFull     BacktestVariant = "full"
)

// DefaultCaseReplayCodes 案例回放五只（设计文档 §3 样本复核）；
// 中稀有色固定为 600259.SH。
var DefaultCaseReplayCodes = []string{"688322.SH", "300780.SZ", "600259.SH", "300829.SZ", "300969.SZ"}

// 时间切分阶段与默认参数。
const (
	PhaseTrain    = "train"         // 训练区间（前 60%）
	PhaseValidate = "validate"      // 验证区间（中 20%）
	PhaseSample   = "out_of_sample" // 样本外区间（后 20%）

	// DefaultWarmupTradingDays 预热向前保留的交易日数（策略最长窗口 120 根）。
	DefaultWarmupTradingDays = 120
	// DefaultTrailingTradingDays 信号终点之后保留的退出数据交易日数
	//（确认窗口 3 日 + 最大持有 5 日）。
	DefaultTrailingTradingDays = 8

	// OOSMinConfirmedTrades 样本外确认交易低于该值时标记样本不足（设计 §7.2）。
	OOSMinConfirmedTrades = 100
)

// BacktestRequest 全链路回测请求。Codes 为空表示本地全市场可研究清单
// （models.AllStockInfo，而非当前狩猎池）。
type BacktestRequest struct {
	StartDate   string
	EndDate     string
	Codes       []string
	InitialCash float64
	Variants    []BacktestVariant
	ConfigPath  string
}

// BacktestInputManifest 四组共享输入的显式边界：清单、日期、成本哈希与初始资金
// 对四组完全一致；只允许变体策略开关（UseDataFix/UseRiskFilter/UseConfirmExit）
// 与可信度标记（Approximate/NotBacktestable）不同。
type BacktestInputManifest struct {
	Codes       []string `json:"codes"` // nil = 全市场
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	CostHash    string   `json:"costHash"`
	InitialCash float64  `json:"initialCash"`
	Universe    string   `json:"universe"` // explicit / all_market

	UseDataFix     bool `json:"useDataFix"`     // Task 3 数据闸门
	UseRiskFilter  bool `json:"useRiskFilter"`  // Task 6 风险过滤
	UseConfirmExit bool `json:"useConfirmExit"` // Task 7 确认入场 + Task 8 退出（退出始终经组合引擎回放）

	Approximate     bool `json:"approximate"`     // 缺时点快照时的近似基线标记
	NotBacktestable bool `json:"notBacktestable"` // 缺历史快照时完整五维结果不可回测
}

// NormalizeBacktestRequest 归一化请求：变体缺省为全部四组、初始资金回退默认、
// 校验日期格式与先后、股票代码归一为内部格式并去重（nil 保持全市场语义）。
func NormalizeBacktestRequest(req BacktestRequest) (BacktestRequest, error) {
	out := req
	if _, err := time.Parse("2006-01-02", req.StartDate); err != nil {
		return out, fmt.Errorf("开始日期无效 %q: %w", req.StartDate, err)
	}
	if _, err := time.Parse("2006-01-02", req.EndDate); err != nil {
		return out, fmt.Errorf("结束日期无效 %q: %w", req.EndDate, err)
	}
	if req.StartDate > req.EndDate {
		return out, fmt.Errorf("开始日期 %s 晚于结束日期 %s", req.StartDate, req.EndDate)
	}
	if len(req.Variants) == 0 {
		out.Variants = []BacktestVariant{VariantBaseline, VariantDataOnly, VariantRiskOnly, VariantFull}
	}
	known := map[BacktestVariant]bool{
		VariantBaseline: true, VariantDataOnly: true, VariantRiskOnly: true, VariantFull: true,
	}
	seen := map[BacktestVariant]bool{}
	variants := make([]BacktestVariant, 0, len(out.Variants))
	for _, v := range out.Variants {
		if !known[v] {
			return out, fmt.Errorf("未知回测变体 %q", v)
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		variants = append(variants, v)
	}
	out.Variants = variants
	if out.InitialCash <= 0 {
		out.InitialCash = DefaultRiskControlConfig().InitialCash
	}
	if req.Codes != nil {
		codes := make([]string, 0, len(req.Codes))
		dup := map[string]bool{}
		for _, c := range req.Codes {
			c = stockcode.Normalize(strings.TrimSpace(c))
			if c == "" || dup[c] {
				continue
			}
			dup[c] = true
			codes = append(codes, c)
		}
		out.Codes = codes
	}
	return out, nil
}

// BuildVariantInput 由共享清单生成变体输入：深拷贝 Codes，只附加策略开关，
// 不改变股票、日期或成本哈希——四组对照的输入一致性边界。
func BuildVariantInput(variant BacktestVariant, manifest BacktestInputManifest) BacktestInputManifest {
	out := manifest
	out.Codes = append([]string(nil), manifest.Codes...)
	switch variant {
	case VariantDataOnly:
		out.UseDataFix = true
	case VariantRiskOnly:
		// 消融递进：risk = 数据修复 + 风险过滤（在 data_only 之上增加风险层）
		out.UseDataFix = true
		out.UseRiskFilter = true
	case VariantFull:
		out.UseDataFix = true
		out.UseRiskFilter = true
		out.UseConfirmExit = true
	case VariantBaseline:
		// 基线：信号日收盘入场、支撑位失效，不启用任何新增层
	}
	return out
}

// CostConfigHash 成本假设指纹（费率/滑点任一变化即变化），四组共享输入的一致性凭证。
func CostConfigHash(cost CostConfig) string {
	blob, _ := json.Marshal(cost)
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:8])
}

// SplitRange 时间切分后的单一区间。WarmupStart 向前（更早）保留指标预热数据，
// ExitDataEnd 向后（更晚）保留确认与退出数据；只在 [SignalStart, SignalEnd] 内生成信号。
type SplitRange struct {
	Phase       string `json:"phase"`       // train / validate / out_of_sample
	WarmupStart string `json:"warmupStart"` // 预热起点（信号起点前 warmup 个交易日）
	SignalStart string `json:"signalStart"`
	SignalEnd   string `json:"signalEnd"`
	ExitDataEnd string `json:"exitDataEnd"` // 退出数据终点（信号终点后 trailing 个交易日）
}

// SplitTradingDays 按 60%/20%/20% 时间顺序切分信号日（禁止随机打乱）。
// warmup 向前扩展预热窗口、trailing 向后扩展确认/退出数据窗口；不足时在序列边界截断。
// 最后一段的 ExitDataEnd 恒为最后一个信号日，其末端信号的确认窗口将越界（仍在窗口）。
func SplitTradingDays(days []string, warmup int, trailing int) []SplitRange {
	n := len(days)
	if n == 0 {
		return nil
	}
	if warmup < 0 {
		warmup = 0
	}
	if trailing < 0 {
		trailing = 0
	}
	phaseOf := []string{PhaseTrain, PhaseValidate, PhaseSample}
	bounds := make([][2]int, 1, 3)
	if n < 3 {
		// 不足以三段切分：整段作为一个训练区间
		bounds[0] = [2]int{0, n}
	} else {
		first := n * 60 / 100
		if first < 1 {
			first = 1
		}
		second := n * 20 / 100
		if second < 1 {
			second = 1
		}
		bounds = bounds[:0]
		bounds = append(bounds, [2]int{0, first}, [2]int{first, first + second}, [2]int{first + second, n})
	}
	splits := make([]SplitRange, 0, len(bounds))
	for i, b := range bounds {
		start, end := b[0], b[1]
		if end <= start {
			continue
		}
		w := start - warmup
		if w < 0 {
			w = 0
		}
		t := end - 1 + trailing
		if t > n-1 {
			t = n - 1
		}
		splits = append(splits, SplitRange{
			Phase:       phaseOf[i],
			WarmupStart: days[w],
			SignalStart: days[start],
			SignalEnd:   days[end-1],
			ExitDataEnd: days[t],
		})
	}
	return splits
}

// ---------- 结果结构 ----------

// SplitBacktestResult 单区间（训练/验证/样本外）回放结果。
type SplitBacktestResult struct {
	Split  SplitRange      `json:"split"`
	Report *BacktestReport `json:"report"`
}

// VariantBacktestResult 单变体（一组对照）结果。
type VariantBacktestResult struct {
	Variant              BacktestVariant       `json:"variant"`
	Manifest             BacktestInputManifest `json:"manifest"`
	Report               *BacktestReport       `json:"report"` // 全窗口合并报告
	Splits               []SplitBacktestResult `json:"splits"`
	OOSConfirmed         int                   `json:"oosConfirmed"`
	SampleInsufficient   bool                  `json:"sampleInsufficient"`
	Stable               bool                  `json:"stable"`
	StableNote           string                `json:"stableNote"`
	Unfinished           bool                  `json:"unfinished"`
	NotBacktestableCodes []string              `json:"notBacktestableCodes,omitempty"`

	// events 案例回放轨迹累积器：RunFullBacktest 结束后统一抽取进 CaseReplays，
	// 不参与 JSON 序列化（避免与 CaseReplays 重复输出）。
	events []ReplayEvent
}

// ReplayEvent 案例回放的逐日轨迹事件。
type ReplayEvent struct {
	Date         string  `json:"date"`
	Variant      string  `json:"variant"`
	Code         string  `json:"code"`
	Stage        string  `json:"stage"` // observation/data/risk/confirm/entry/protect/exit/invalid/expired/untradeable/still_in_window
	Detail       string  `json:"detail,omitempty"`
	Price        float64 `json:"price,omitempty"`
	ProtectPrice float64 `json:"protectPrice,omitempty"`
	Shares       int     `json:"shares,omitempty"`
}

const (
	stageObservation   = "observation"
	stageData          = "data"
	stageRisk          = "risk"
	stageConfirm       = "confirm"
	stageEntry         = "entry"
	stageProtect       = "protect"
	stageExit          = "exit"
	stageInvalid       = "invalid"
	stageExpired       = "expired"
	stageUntradeable   = "untradeable"
	stageStillInWindow = "still_in_window"
)

// CaseReplay 单只案例的完整轨迹。
type CaseReplay struct {
	Code   string        `json:"code"`
	Events []ReplayEvent `json:"events"`
}

// FullBacktestResult 全链路四组对照回测结果（CLI JSON 输出的顶层结构）。
type FullBacktestResult struct {
	StartDate   string                  `json:"startDate"`
	EndDate     string                  `json:"endDate"`
	AllMarket   bool                    `json:"allMarket"`
	Codes       []string                `json:"codes,omitempty"`
	CostHash    string                  `json:"costHash"`
	Config      RiskControlConfig       `json:"config"`
	Splits      []SplitRange            `json:"splits"`
	Variants    []VariantBacktestResult `json:"variants"`
	CaseReplays []CaseReplay            `json:"caseReplays"`
	Limitations []string                `json:"limitations"`
}

// ---------- 内部：逐日缓存 ----------

// researchCache 时点研究记录缓存：记录缺失时用生产研究流水线重放该交易日
// （只写 KhunterResearchRecord，不碰生产表），跨变体与跨区间复用。
type researchCache struct {
	ctx    context.Context
	repo   *Repo
	byDate map[string]*ResearchRecordResult
}

func newResearchCache(ctx context.Context, repo *Repo) *researchCache {
	return &researchCache{ctx: ctx, repo: repo, byDate: map[string]*ResearchRecordResult{}}
}

func (c *researchCache) forDate(tradeDate string) (*ResearchRecordResult, error) {
	if p, ok := c.byDate[tradeDate]; ok {
		return p, nil
	}
	payload := c.loadStored(tradeDate)
	if payload == nil {
		now, _ := time.Parse("2006-01-02", tradeDate)
		if _, err := runResearchPipeline(c.ctx, c.repo, tradeDate, now); err != nil {
			return nil, fmt.Errorf("研究重放失败 %s: %w", tradeDate, err)
		}
		payload = c.loadStored(tradeDate)
		if payload == nil {
			return nil, fmt.Errorf("研究记录重放后仍缺失 %s", tradeDate)
		}
	}
	c.byDate[tradeDate] = payload
	return payload, nil
}

func (c *researchCache) loadStored(tradeDate string) *ResearchRecordResult {
	row, err := c.repo.GetResearchRecord(tradeDate)
	if err != nil || row == nil {
		return nil
	}
	var p ResearchRecordResult
	if json.Unmarshal([]byte(row.ResultJSON), &p) != nil || p.Result == nil {
		return nil
	}
	return &p
}

// riskLevelCache 当日市场风险档缓存（生产 marketRisk，点入时点：只读 ≤ 当日的 sh000300）。
type riskLevelCache struct {
	ctx    context.Context
	byDate map[string]risk.Level
}

func (c *riskLevelCache) forDate(tradeDate string) risk.Level {
	if lv, ok := c.byDate[tradeDate]; ok {
		return lv
	}
	now, _ := time.Parse("2006-01-02", tradeDate)
	lv, _ := marketRisk(c.ctx, tradeDate, now)
	c.byDate[tradeDate] = lv
	return lv
}

// barCache 本地日 K 缓存（候选股按需加载，超上限整体清空防止全市场常驻内存）。
type barCache struct {
	ctx      context.Context
	start    string // 全窗口起点（含预热）
	end      string
	raw      map[string][]models.KLineBar
	scan     map[string][]models.KLineBar // key: code|asOf → prefer-adjusted ≤ asOf
	order    []string
	maxCodes int
}

func newBarCache(ctx context.Context, start, end string) *barCache {
	return &barCache{
		ctx: ctx, start: start, end: end,
		raw: map[string][]models.KLineBar{}, scan: map[string][]models.KLineBar{},
		maxCodes: 2048,
	}
}

func (c *barCache) key(code, asOf string) string { return code + "|" + asOf }

func (c *barCache) evictIfNeeded(code string) {
	if _, ok := c.raw[code]; ok {
		return
	}
	if len(c.order) >= c.maxCodes {
		c.raw = map[string][]models.KLineBar{}
		c.scan = map[string][]models.KLineBar{}
		c.order = c.order[:0]
	}
	c.order = append(c.order, code)
}

// rawBars 不复权日 K（[start-400 自然日, end]，升序）。
func (c *barCache) rawBars(code string) ([]models.KLineBar, error) {
	if bars, ok := c.raw[code]; ok {
		return bars, nil
	}
	c.evictIfNeeded(code)
	start := daysBefore(c.start, 400)
	if start == "" {
		start = c.start
	}
	bars, err := datasource.NewKLineStore().QueryKLines(c.ctx, code, "day", start, c.end, false)
	if err != nil {
		return nil, err
	}
	c.raw[code] = bars
	return bars, nil
}

// scanBars 研究扫描口径（prefer-adjusted，≤ asOf，升序）。
func (c *barCache) scanBars(code, asOf string) ([]models.KLineBar, error) {
	k := c.key(code, asOf)
	if bars, ok := c.scan[k]; ok {
		return bars, nil
	}
	start := daysBefore(asOf, 400)
	if start == "" {
		start = asOf
	}
	bars, err := datasource.NewKLineStore().QueryKLinesPreferAdjusted(c.ctx, code, "day", start, asOf)
	if err != nil {
		return nil, err
	}
	c.scan[k] = bars
	return bars, nil
}

func (c *barCache) rawBarAt(code, date string) (models.KLineBar, bool) {
	bars, err := c.rawBars(code)
	if err != nil {
		return models.KLineBar{}, false
	}
	return findBarByDate(bars, date)
}

func (c *barCache) adjustedBarAt(code, date string) (models.KLineBar, bool) {
	bars, err := datasource.NewKLineStore().QueryKLines(c.ctx, code, "day", date, date, true)
	if err != nil || len(bars) == 0 {
		return models.KLineBar{}, false
	}
	return bars[0], true
}

// windowBars 不复权日 K 切片（[start, end]，供组合回测引擎）。
func (c *barCache) windowBars(code, start, end string) []models.KLineBar {
	bars, err := c.rawBars(code)
	if err != nil {
		return nil
	}
	out := make([]models.KLineBar, 0, len(bars))
	for _, b := range bars {
		if b.TradeDate >= start && b.TradeDate <= end {
			out = append(out, b)
		}
	}
	return out
}

// ---------- 内部：候选准备 ----------

// preparedCandidate 单个候选信号的全部点入时点输入（跨变体共享）。
type preparedCandidate struct {
	code            string
	name            string
	signalDate      string
	score           float64
	support         float64 // 研究记录支撑位（缺失回退生产 SupportPrice）
	obs             models.KhunterObservation
	gate            DataDecision
	riskSignal      RiskDecision // 信号日风险结论（生产 AttachSignalRisk）
	snapshotMissing bool

	rawHigh       float64 // 不复权触发价（确认组；转换失败为 0）
	rawLow        float64
	rawATR        float64
	rawEntry      float64 // 信号日不复权收盘（非确认组入场价）
	rawSignalHigh float64 // 信号日不复权最高（非确认组突破失败判定）
	levelsOK      bool

	confirmIdx     []int // 确认日在全局交易日序列的下标（T+1..T+3 全序列，含越界）
	windowCloseIdx int   // 确认窗口最后确认日下标
}

type runner struct {
	ctx         context.Context
	repo        *Repo
	cfg         RiskControlConfig
	days        []string
	dayIdx      map[string]int
	research    *researchCache
	levels      *riskLevelCache
	bars        *barCache
	bench       []models.KLineBar
	universeSet map[string]bool // nil = 全市场
	caseTraces  []caseEntry
}

type caseEntry struct {
	entry      PortfolioEntry
	exitEndIdx int
}

// prepareCandidate 加载并准备候选（结果按 (code, signalDate) 在调用方缓存）。
func (r *runner) prepareCandidate(hunted models.KhunterHunting, tradeDate string, snapshotMissing bool, level risk.Level) (*preparedCandidate, error) {
	cand := &preparedCandidate{
		code:            hunted.Code,
		name:            hunted.Name,
		signalDate:      tradeDate,
		score:           hunted.EnterScore,
		support:         hunted.SupportPrice,
		snapshotMissing: snapshotMissing,
	}
	scan, err := r.bars.scanBars(cand.code, tradeDate)
	if err != nil || len(scan) == 0 {
		return cand, fmt.Errorf("候选 %s 信号日无本地K线", cand.code)
	}
	gate := EvaluateDataGate(scan, tradeDate, true)
	cand.gate = gate
	// 生产观察构造：触发价=信号日最高、低点、ATR14（前复权口径，确认时转换）
	score := models.KhunterScore{Code: cand.code, Name: cand.name, ScoreDate: tradeDate, Total: cand.score}
	cand.obs = BuildObservation(score, scan, gate, tradeDate)
	// 生产信号日风险画像（只记录，不否决——入场日恢复仍可确认）
	cand.riskSignal = AttachSignalRisk(&cand.obs, scan, gate, level, r.cfg)

	if cand.support <= 0 {
		cand.support = SupportPrice(scan)
	}

	// 不复权信号价转换（与生产确认引擎同一口径 RawSignalLevel）
	adjBar, okA := r.bars.adjustedBarAt(cand.code, tradeDate)
	rawBar, okR := r.bars.rawBarAt(cand.code, tradeDate)
	if okA && okR {
		high, errH := RawSignalLevel(cand.obs.TriggerPrice, adjBar.Close, rawBar.Close)
		low, errL := RawSignalLevel(cand.obs.SignalLow, adjBar.Close, rawBar.Close)
		atr, errATR := RawSignalLevel(cand.obs.ATR14, adjBar.Close, rawBar.Close)
		if errH == nil && errL == nil && errATR == nil {
			cand.rawHigh, cand.rawLow, cand.rawATR = high, low, atr
			cand.levelsOK = true
		}
	}
	if okR {
		cand.rawEntry = rawBar.Close
		cand.rawSignalHigh = rawBar.High
	}

	// 确认窗口：信号日后第 1..ConfirmTradingDays 个交易日（生产 ConfirmationWindowState 语义）
	signalIdx := r.dayIdx[tradeDate]
	confirmWindow := r.cfg.ConfirmTradingDays
	if confirmWindow <= 0 {
		confirmWindow = 3
	}
	for off := 1; off <= confirmWindow; off++ {
		if idx := signalIdx + off; idx < len(r.days) {
			cand.confirmIdx = append(cand.confirmIdx, idx)
		}
	}
	cand.windowCloseIdx = signalIdx + confirmWindow
	return cand, nil
}

// ---------- 内部：候选 → 变体结局 ----------

// allowDataDecision 非数据修复组的替代数据结论：旧口径容忍缺失量额单位，
// 数据闸门不参与否决（生产 EvaluateRisk 的 DataDecision 输入按全放行处理）。
func allowDataDecision() DataDecision {
	return DataDecision{AllowObservation: true, AllowConfirm: true, AllowBacktest: true, Status: "valid"}
}

type candidateOutcome struct {
	entry   *PortfolioEntry
	outcome string // entry / invalid / untradeable / expired / still_window
}

func (r *runner) processCandidate(cc candidateCtx, cand *preparedCandidate) candidateOutcome {
	rep := cc.report
	base := ReplayEvent{Variant: string(cc.variant), Code: cand.code, Date: cand.signalDate}

	// 观察事件（信号日候选进入链路）
	obsDetail := fmt.Sprintf("评分 %.1f 数据状态 %s 支撑位 %.2f", cand.score, cand.gate.Status, cand.support)
	if cand.snapshotMissing {
		obsDetail += "（缺时点快照，五维结果不可回测）"
	}
	cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageObservation, Detail: obsDetail, Price: cand.score})

	if cand.snapshotMissing {
		// 缺时点快照：按研究记录原样回放（禁止调最新接口补数），只标记可信度
		cc.vr.NotBacktestableCodes = appendUnique(cc.vr.NotBacktestableCodes, cand.code)
		if cc.variant == VariantBaseline {
			cc.vr.Manifest.Approximate = true
		}
		if cc.variant == VariantFull {
			cc.vr.Manifest.NotBacktestable = true
		}
	}

	// 数据闸门（data_only / full）
	if cc.vcfg.UseDataFix {
		if cand.gate.Status == "invalid" || !cand.gate.AllowObservation {
			reason := strings.Join(cand.gate.Reasons, "；")
			rep.AddInvalid("数据闸门不放行: " + reason)
			cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageData, Detail: "不放行确认: " + reason})
			return candidateOutcome{outcome: "invalid"}
		}
	}

	dataForRisk := cand.gate
	if !cc.vcfg.UseDataFix {
		dataForRisk = allowDataDecision()
	}

	if !cc.vcfg.UseConfirmExit {
		return r.processImmediateEntry(cc, cand, dataForRisk)
	}
	return r.processConfirmedEntry(cc, cand, dataForRisk)
}

// processImmediateEntry 非确认组：信号日收盘入场、研究支撑位失效
// （当前基线口径；退出仍由 Task 8 状态机经组合引擎回放）。
func (r *runner) processImmediateEntry(cc candidateCtx, cand *preparedCandidate, dataForRisk DataDecision) candidateOutcome {
	rep := cc.report
	base := ReplayEvent{Variant: string(cc.variant), Code: cand.code, Date: cand.signalDate}

	if cc.vcfg.UseRiskFilter {
		decision := r.evaluateRiskAt(cand.code, cand.signalDate, dataForRisk)
		cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageRisk, Detail: riskDetail(decision)})
		if !decision.AllowConfirm {
			rep.AddExpired()
			cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageExpired, Detail: "风险过滤否决: " + strings.Join(decision.Reasons, "；")})
			return candidateOutcome{outcome: "expired"}
		}
	}

	entry := cand.rawEntry
	stop := cand.support
	if entry <= 0 || stop <= 0 || stop >= entry {
		rep.AddInvalid("失效价或风险距离无效")
		cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageInvalid, Detail: "入场价或支撑位无效"})
		return candidateOutcome{outcome: "invalid"}
	}
	limit := 1.0
	if cc.vcfg.UseRiskFilter {
		limit = r.levels.forDate(cand.signalDate).PositionLimit
	}
	shares := RiskBudgetShares(r.cfg.InitialCash, entry, stop, limit, 0, r.cfg)
	if shares < 100 {
		rep.AddUntradeable("风险预算不足一手")
		cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageUntradeable, Detail: "风险预算不足一手"})
		return candidateOutcome{outcome: "untradeable"}
	}
	*cc.seq++
	e := PortfolioEntry{
		Code: cand.code, SignalDate: cand.signalDate, ConfirmDate: cand.signalDate,
		EntryPrice: entry, StopPrice: stop, SignalHigh: cand.rawSignalHigh,
		Shares: shares, ConfirmSeq: *cc.seq,
	}
	cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageEntry, Detail: "信号日收盘入场（基线口径）", Price: entry, ProtectPrice: stop, Shares: shares})
	return candidateOutcome{entry: &e, outcome: "entry"}
}

// processConfirmedEntry 确认组：T+1~T+3 内逐日按生产确认规则评估（数据闸门、
// 风险过滤、一字板、跳空、触发价、ATR 失效价、风险预算股数）。
func (r *runner) processConfirmedEntry(cc candidateCtx, cand *preparedCandidate, dataForRisk DataDecision) candidateOutcome {
	rep := cc.report
	base := ReplayEvent{Variant: string(cc.variant), Code: cand.code, Date: cand.signalDate}

	// 信号日风险只记录不否决（生产：信号日失败、入场日恢复仍可确认）
	if cc.vcfg.UseRiskFilter {
		cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageRisk, Detail: riskDetail(cand.riskSignal)})
	}

	lastReason := ""
	for _, ci := range cand.confirmIdx {
		if ci > cc.exitEndIdx {
			break // 确认日数据越界：不再尝试
		}
		confirmDay := r.days[ci]
		level := r.levels.forDate(confirmDay)
		if cc.vcfg.UseRiskFilter && level.PositionLimit <= 0 {
			lastReason = "市场风险档禁止新增"
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageRisk, Detail: lastReason})
			continue
		}
		scan, err := r.bars.scanBars(cand.code, confirmDay)
		if err != nil || len(scan) == 0 {
			lastReason = "确认日无本地K线"
			continue
		}
		dayGate := EvaluateDataGate(scan, confirmDay, true)
		if cc.vcfg.UseDataFix && !dayGate.AllowConfirm {
			lastReason = strings.Join(dayGate.Reasons, "；")
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageData, Detail: "数据闸门不放行确认: " + lastReason})
			continue
		}
		dayData := dayGate
		if !cc.vcfg.UseDataFix {
			dayData = allowDataDecision()
		}
		if cc.vcfg.UseRiskFilter {
			decision := r.evaluateRiskAt(cand.code, confirmDay, dayData)
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageRisk, Detail: riskDetail(decision)})
			if !decision.AllowConfirm {
				lastReason = strings.Join(decision.Reasons, "；")
				continue
			}
		}
		if !cand.levelsOK {
			lastReason = "不复权信号价转换失败"
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageConfirm, Detail: lastReason})
			continue
		}
		confirmBar, ok := r.bars.rawBarAt(cand.code, confirmDay)
		if !ok {
			lastReason = "确认日无本地K线"
			continue
		}
		limit, limitOK := dailyPriceLimit(cand.code, confirmBar)
		if !limitOK {
			// 与实盘确认链路同一规则：缺前收无法判定涨跌停，禁止确认（防回测偏乐观）
			lastReason = "缺前一交易日收盘价，无法判定涨跌停，禁止确认"
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageConfirm, Detail: lastReason})
			continue
		}
		cd := EvaluateConfirmationDay(cand.rawHigh, cand.rawLow, cand.rawATR, confirmBar, limit, RiskDecision{AllowConfirm: true}, r.cfg)
		if !cd.Confirmed {
			lastReason = cd.Reason
			if lastReason == "" {
				lastReason = "未触及确认价"
			}
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageConfirm, Detail: "未确认: " + lastReason})
			continue
		}
		posLimit := 1.0
		if cc.vcfg.UseRiskFilter {
			posLimit = level.PositionLimit
		}
		shares := RiskBudgetShares(r.cfg.InitialCash, cd.EntryPrice, cd.StopPrice, posLimit, 0, r.cfg)
		if shares < 100 {
			lastReason = "不足一手"
			cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageConfirm, Detail: "已确认但" + lastReason})
			continue
		}
		*cc.seq++
		e := PortfolioEntry{
			Code: cand.code, SignalDate: cand.signalDate, ConfirmDate: confirmDay,
			EntryPrice: cd.EntryPrice, StopPrice: cd.StopPrice, SignalHigh: cand.rawHigh,
			Shares: shares, ConfirmSeq: *cc.seq,
		}
		cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageConfirm, Detail: "确认成立", Price: cd.EntryPrice})
		cc.emit(ReplayEvent{Date: confirmDay, Variant: base.Variant, Code: base.Code, Stage: stageEntry, Detail: "确认入场", Price: cd.EntryPrice, ProtectPrice: cd.StopPrice, Shares: shares})
		return candidateOutcome{entry: &e, outcome: "entry"}
	}

	// 窗口内未确认：确认窗口在回测数据内闭合 → 过期；否则仍在窗口（未完成）
	if cand.windowCloseIdx > cc.exitEndIdx || cand.windowCloseIdx >= len(r.days) {
		rep.AddStillInWindow("确认窗口未在回测数据内闭合（信号 " + cand.signalDate + "）")
		cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageStillInWindow, Detail: "确认窗口越界，运行未完成"})
		return candidateOutcome{outcome: "still_window"}
	}
	rep.AddExpired()
	cc.emit(ReplayEvent{Date: base.Date, Variant: base.Variant, Code: base.Code, Stage: stageExpired, Detail: "确认窗口内未确认: " + lastReason})
	return candidateOutcome{outcome: "expired"}
}

// evaluateRiskAt 生产风险过滤（Task 6）：当日 K 线风险画像 + 阈值 + 市场风险档。
func (r *runner) evaluateRiskAt(code, date string, data DataDecision) RiskDecision {
	scan, err := r.bars.scanBars(code, date)
	if err != nil {
		return RiskDecision{AllowConfirm: false, Reasons: []string{err.Error()}}
	}
	metrics, merr := BuildRiskMetrics(scan, r.cfg)
	if merr != nil {
		return RiskDecision{AllowConfirm: false, Reasons: []string{merr.Error()}}
	}
	return EvaluateRisk(metrics, data, r.levels.forDate(date), r.cfg)
}

func riskDetail(d RiskDecision) string {
	if d.AllowConfirm {
		return "风险过滤通过"
	}
	return "风险否决: " + strings.Join(d.Reasons, "；")
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// candidateCtx 单个候选处理所需的上下文。
type candidateCtx struct {
	variant    BacktestVariant
	vcfg       BacktestInputManifest
	split      SplitRange
	exitEndIdx int
	report     *BacktestReport
	vr         *VariantBacktestResult
	seq        *int

	// 事件保留过滤器：全市场运行时只保留案例代码的轨迹（避免 O(候选×变体)
	// 事件驻留内存后被丢弃）；显式小清单运行保留全部代码轨迹。
	keepEvent    bool
	eventCaseSet map[string]bool
}

func (cc *candidateCtx) emit(ev ReplayEvent) {
	if !cc.keepEvent && !cc.eventCaseSet[ev.Code] {
		return
	}
	cc.vr.events = append(cc.vr.events, ev)
}

// ---------- 内部：变体与区间装配 ----------

// runVariantSplit 回放单一区间：信号日逐日取时点研究记录 → 候选 → 变体规则 →
// 组合引擎（生产成本/滑点/涨跌停/T+1/敞口/强平）。
func (r *runner) runVariantSplit(variant BacktestVariant, vcfg BacktestInputManifest, split SplitRange, vr *VariantBacktestResult) (SplitBacktestResult, error) {
	sigStartIdx := r.dayIdx[split.SignalStart]
	sigEndIdx := r.dayIdx[split.SignalEnd]
	exitEndIdx := r.dayIdx[split.ExitDataEnd]

	// 候选缓存：同一 (信号日, 代码) 跨变体共享准备结果
	candCache := map[string]*preparedCandidate{}
	report := NewBacktestReport(0)
	seq := 0
	entries := make([]PortfolioEntry, 0, 8)
	candCount := 0

	// 案例轨迹过滤集合：全市场运行只保留案例代码事件，显式清单保留全部
	caseSet := map[string]bool{}
	for _, c := range DefaultCaseReplayCodes {
		caseSet[stockcode.Normalize(c)] = true
	}

	for idx := sigStartIdx; idx <= sigEndIdx; idx++ {
		date := r.days[idx]
		payload, err := r.research.forDate(date)
		if err != nil {
			return SplitBacktestResult{}, err
		}
		notBT := map[string]bool{}
		for _, code := range payload.NotBacktestable {
			notBT[code] = true
		}
		level := r.levels.forDate(date)
		for _, hunted := range payload.Hunted {
			if r.universeSet != nil && !r.universeSet[hunted.Code] {
				continue
			}
			cc := candidateCtx{
				variant: variant, vcfg: vcfg, split: split, exitEndIdx: exitEndIdx,
				report: report, vr: vr, seq: &seq,
				keepEvent: r.universeSet != nil, eventCaseSet: caseSet,
			}
			key := hunted.Code + "|" + date
			cand, ok := candCache[key]
			if !ok {
				cand, err = r.prepareCandidate(hunted, date, notBT[hunted.Code], level)
				if err != nil {
					// 无本地 K 线的候选记为无效，不静默丢弃
					report.AddInvalid(err.Error())
					cc.emit(ReplayEvent{Date: date, Variant: string(variant), Code: hunted.Code, Stage: stageInvalid, Detail: err.Error()})
					candCount++
					continue
				}
				candCache[key] = cand
			}
			candCount++
			out := r.processCandidate(cc, cand)
			if out.entry != nil {
				entries = append(entries, *out.entry)
			}
		}
	}
	report.SignalCount = candCount

	// 组合回放窗口：[SignalStart, ExitDataEnd]（信号只来自区间内；退出数据向后保留）
	windowDays := append([]string(nil), r.days[sigStartIdx:exitEndIdx+1]...)
	codeBars := map[string][]models.KLineBar{}
	for _, e := range entries {
		if _, ok := codeBars[e.Code]; !ok {
			codeBars[e.Code] = r.bars.windowBars(e.Code, split.SignalStart, split.ExitDataEnd)
		}
	}
	var posLimits map[string]float64
	defaultLimit := 1.0
	if vcfg.UseRiskFilter {
		posLimits = map[string]float64{}
		for _, d := range windowDays {
			posLimits[d] = r.levels.forDate(d).PositionLimit
		}
	}
	engine := RunPortfolioBacktest(PortfolioBacktestInput{
		TradeDates:           windowDays,
		BenchmarkBars:        r.benchBetween(split.SignalStart, split.ExitDataEnd),
		CodeBars:             codeBars,
		Entries:              entries,
		PositionLimits:       posLimits,
		DefaultPositionLimit: defaultLimit,
		Cfg:                  r.cfg,
	})
	mergeEngineReport(report, engine)
	if variant == VariantFull {
		for _, e := range entries {
			r.caseTraces = append(r.caseTraces, caseEntry{entry: e, exitEndIdx: exitEndIdx})
		}
	}
	return SplitBacktestResult{Split: split, Report: report}, nil
}

func mergeEngineReport(dst *BacktestReport, src *BacktestReport) {
	dst.ConfirmedCount += src.ConfirmedCount
	dst.InvalidCount += src.InvalidCount
	dst.InvalidReasons = append(dst.InvalidReasons, src.InvalidReasons...)
	dst.UntradeableCount += src.UntradeableCount
	dst.UntradeableReasons = append(dst.UntradeableReasons, src.UntradeableReasons...)
	dst.ExpiredCount += src.ExpiredCount
	dst.TradeCount += src.TradeCount
	dst.Trades = append(dst.Trades, src.Trades...)
	dst.PortfolioEquity = append(dst.PortfolioEquity, src.PortfolioEquity...)
	dst.Limitations = append(dst.Limitations, src.Limitations...)
	dst.refreshMetrics()
}

func (r *runner) benchBetween(start, end string) []models.KLineBar {
	out := make([]models.KLineBar, 0, len(r.bench))
	for _, b := range r.bench {
		if b.TradeDate >= start && b.TradeDate <= end {
			out = append(out, b)
		}
	}
	return out
}

func (r *runner) runVariant(variant BacktestVariant, vcfg BacktestInputManifest, splits []SplitRange, sharedLimitations []string) (VariantBacktestResult, error) {
	vr := VariantBacktestResult{Variant: variant, Manifest: vcfg, Stable: true}
	merged := &BacktestReport{}
	for _, split := range splits {
		sr, err := r.runVariantSplit(variant, vcfg, split, &vr)
		if err != nil {
			return vr, err
		}
		vr.Splits = append(vr.Splits, sr)
		mergeReports(merged, sr.Report)
	}
	finalizeMergedReport(merged, vr.Splits)
	merged.SignalCount = 0
	for _, sr := range vr.Splits {
		merged.SignalCount += sr.Report.SignalCount
	}
	if first, last, ok := benchmarkRange(r.bench, r.days[0], r.days[len(r.days)-1]); ok {
		merged.SetBenchmarkReturn(last/first - 1)
	}
	merged.Limitations = append(merged.Limitations, sharedLimitations...)
	if len(vr.NotBacktestableCodes) > 0 {
		merged.Limitations = append(merged.Limitations,
			"缺时点快照标的（按研究记录原样回放，禁止用最新数据替代）: "+strings.Join(vr.NotBacktestableCodes, "、"))
		if variant == VariantBaseline {
			merged.Limitations = append(merged.Limitations, "近似基线：缺时点快照，基线结果为近似重建（Approximate）")
		}
		if variant == VariantFull {
			merged.Limitations = append(merged.Limitations, "完整五维结果不可回测：缺基本面/板块历史时点快照（NotBacktestable），结论不得用于替换默认策略")
		}
	}
	vr.Report = merged
	vr.Unfinished = merged.StillInWindowCount > 0 || merged.Unfinished
	if n := len(vr.Splits); n > 0 && vr.Splits[n-1].Split.Phase == PhaseSample {
		vr.OOSConfirmed = vr.Splits[n-1].Report.ConfirmedCount
	}
	vr.SampleInsufficient = vr.OOSConfirmed < OOSMinConfirmedTrades
	return vr, nil
}

// finalizeMergedReport 各时间段独立以初始资金重放（段间现金重启）：直接对拼接权益
// 序列求回撤会把上一段期末盈利当作下一段的回撤（如 +5% 段接平段虚增 ~4.8% 回撤）。
// 合并口径的组合最大回撤取各时间段（训练/验证/样本外）回撤的最大值（最差段），
// 不从拼接序列推导，并注明口径；PortfolioEquity 拼接保留，仅作展示序列。
func finalizeMergedReport(merged *BacktestReport, splits []SplitBacktestResult) {
	worst := 0.0
	for _, sr := range splits {
		if sr.Report.Metrics.PortfolioMaxDD > worst {
			worst = sr.Report.Metrics.PortfolioMaxDD
		}
	}
	merged.Metrics.PortfolioMaxDD = worst
	merged.AddLimitation("组合最大回撤取各时间段（训练/验证/样本外）回撤的最大值；各时间段独立以初始资金重放，不拼接权益曲线")
}

func mergeReports(dst, src *BacktestReport) {
	dst.ConfirmedCount += src.ConfirmedCount
	dst.InvalidCount += src.InvalidCount
	dst.InvalidReasons = append(dst.InvalidReasons, src.InvalidReasons...)
	dst.UntradeableCount += src.UntradeableCount
	dst.UntradeableReasons = append(dst.UntradeableReasons, src.UntradeableReasons...)
	dst.ExpiredCount += src.ExpiredCount
	dst.StillInWindowCount += src.StillInWindowCount
	dst.StillInWindowReasons = append(dst.StillInWindowReasons, src.StillInWindowReasons...)
	if src.Unfinished {
		dst.Unfinished = true
	}
	dst.TradeCount += src.TradeCount
	dst.Trades = append(dst.Trades, src.Trades...)
	dst.PortfolioEquity = append(dst.PortfolioEquity, src.PortfolioEquity...)
	dst.Limitations = append(dst.Limitations, src.Limitations...)
	dst.refreshMetrics()
}

func benchmarkRange(bars []models.KLineBar, start, end string) (first, last float64, ok bool) {
	for _, b := range bars {
		if b.TradeDate < start || b.TradeDate > end || b.Close <= 0 {
			continue
		}
		if first == 0 {
			first = b.Close
		}
		last = b.Close
		ok = true
	}
	return first, last, ok
}

// computeStability 结论方向稳定性：各市场阶段（训练/验证/样本外）中，
// 变体相对基线组的平均净收益方向（baseline 相对 0）至少两个阶段不一致 → Stable=false。
func computeStability(variants []VariantBacktestResult) {
	basePhases := map[string]float64{}
	var baseline *VariantBacktestResult
	for i := range variants {
		if variants[i].Variant == VariantBaseline {
			baseline = &variants[i]
			break
		}
	}
	if baseline != nil {
		for _, sb := range baseline.Splits {
			basePhases[sb.Split.Phase] = sb.Report.Metrics.AvgNetReturn
		}
	}
	for i := range variants {
		v := &variants[i]
		dirs := make([]int, 0, 3)
		phases := make([]string, 0, 3)
		for _, sb := range v.Splits {
			hasData := sb.Report.TradeCount > 0
			baseHasData := baseline != nil && baseline != v && basePhaseHasTrades(baseline, sb.Split.Phase)
			if v == baseline {
				baseHasData = hasData
			}
			if !hasData || !baseHasData {
				continue // 该阶段无可比成交，方向未定义
			}
			diff := sb.Report.Metrics.AvgNetReturn - basePhases[sb.Split.Phase]
			if v == baseline {
				diff = sb.Report.Metrics.AvgNetReturn
			}
			switch {
			case diff > 1e-12:
				dirs = append(dirs, 1)
			case diff < -1e-12:
				dirs = append(dirs, -1)
			default:
				dirs = append(dirs, 0)
			}
			phases = append(phases, sb.Split.Phase)
		}
		stable, note := true, ""
		if len(dirs) < 2 {
			note = "可比市场阶段不足两个，结论稳定性未定（Stable 保持 true，不作为通过依据）"
		} else {
			for _, d := range dirs[1:] {
				if d != dirs[0] {
					stable = false
					break
				}
			}
			if !stable {
				note = "不同市场阶段结论方向不一致（" + strings.Join(phases, "/") + "），Stable=false"
			} else {
				note = "可比阶段结论方向一致"
			}
		}
		v.Stable = stable
		v.StableNote = note
	}
}

func basePhaseHasTrades(baseline *VariantBacktestResult, phase string) bool {
	for _, sb := range baseline.Splits {
		if sb.Split.Phase == phase {
			return sb.Report.TradeCount > 0
		}
	}
	return false
}

// ---------- 案例回放轨迹 ----------

// buildCaseReplays 案例轨迹：优先回放五只案例代码（DefaultCaseReplayCodes）；
// 全市场运行下若无案例命中则不输出（避免全市场轨迹）；显式小清单运行回放全部代码。
// 保护价/退出轨迹由退出状态机（EvaluateExitDay）逐日重放生成，与组合引擎同一输入口径。
func (r *runner) buildCaseReplays(eventsByVariant map[BacktestVariant][]ReplayEvent) []CaseReplay {
	all := make([]ReplayEvent, 0, 256)
	for _, evs := range eventsByVariant {
		all = append(all, evs...)
	}
	caseSet := map[string]bool{}
	for _, c := range DefaultCaseReplayCodes {
		caseSet[stockcode.Normalize(c)] = true
	}
	codes := map[string]bool{}
	caseHit := false
	for _, ev := range all {
		codes[ev.Code] = true
		if caseSet[ev.Code] {
			caseHit = true
		}
	}
	selected := make([]string, 0, 8)
	if caseHit {
		for c := range codes {
			if caseSet[c] {
				selected = append(selected, c)
			}
		}
	} else if r.universeSet != nil {
		for c := range codes {
			selected = append(selected, c)
		}
	}
	if len(selected) == 0 {
		return nil
	}
	sort.Strings(selected)

	replays := make([]CaseReplay, 0, len(selected))
	for _, code := range selected {
		evs := make([]ReplayEvent, 0, 64)
		for _, ev := range all {
			if ev.Code == code {
				evs = append(evs, ev)
			}
		}
		for _, ct := range r.caseTraces {
			if ct.entry.Code != code {
				continue
			}
			evs = append(evs, r.exitTrace(ct)...)
		}
		sort.SliceStable(evs, func(i, j int) bool {
			if evs[i].Date != evs[j].Date {
				return evs[i].Date < evs[j].Date
			}
			return stageRank(evs[i].Stage) < stageRank(evs[j].Stage)
		})
		replays = append(replays, CaseReplay{Code: code, Events: evs})
	}
	return replays
}

func stageRank(stage string) int {
	switch stage {
	case stageObservation:
		return 0
	case stageData:
		return 1
	case stageRisk:
		return 2
	case stageConfirm:
		return 3
	case stageEntry:
		return 4
	case stageProtect:
		return 5
	case stageExit:
		return 6
	default:
		return 7
	}
}

// exitTrace 保护价/退出轨迹：按组合引擎同一顺序重放退出状态机（先退出后入场、
// T+1 只记录待退出、一字跌停不可成交、保护位抬升）。价格为原始口径，
// 引擎成交价另含滑点（成交记录见 report.Trades）。
func (r *runner) exitTrace(ct caseEntry) []ReplayEvent {
	e := ct.entry
	entryIdx := r.dayIdx[e.ConfirmDate]
	if entryIdx < 0 {
		return nil
	}
	p := models.KhunterHunting{
		Code: e.Code, EntryPrice: e.EntryPrice, ProtectPrice: e.StopPrice,
		InitialRisk: e.EntryPrice - e.StopPrice,
	}
	evs := make([]ReplayEvent, 0, 8)
	for idx := entryIdx; idx <= ct.exitEndIdx && idx < len(r.days); idx++ {
		date := r.days[idx]
		bar, ok := r.bars.rawBarAt(e.Code, date)
		if !ok {
			continue
		}
		exitLimit, _ := dailyPriceLimit(e.Code, bar) // 退出降级口径
		decision := EvaluateExitDay(p, e.SignalHigh, bar, exitLimit,
			idx == entryIdx, idx-entryIdx, r.cfg)
		evs = append(evs, ReplayEvent{
			Date: date, Variant: string(VariantFull), Code: e.Code, Stage: stageProtect,
			Detail: "保护位跟踪", ProtectPrice: decision.NewProtectPrice, Price: bar.Close,
			Shares: e.Shares,
		})
		p.ProtectPrice = decision.NewProtectPrice
		if decision.Exit {
			evs = append(evs, ReplayEvent{
				Date: date, Variant: string(VariantFull), Code: e.Code, Stage: stageExit,
				Detail: decision.Reason, Price: decision.ExitPrice, Shares: e.Shares,
			})
			break
		}
	}
	return evs
}

// ---------- 入口 ----------

// RunFullBacktest 全链路四组对照回测入口。需要 db.Dao 已初始化（CLI 调 db.Init 后
// EnsureMigrate；测试直接注入）。信号/评分只读时点研究记录，缺失时用生产研究流水线重放；
// 绝不调用最新行情/快照接口，也绝不写当前狩猎池。
func RunFullBacktest(ctx context.Context, req BacktestRequest) (*FullBacktestResult, error) {
	norm, err := NormalizeBacktestRequest(req)
	if err != nil {
		return nil, err
	}
	cfg := LoadRiskControlConfig(backtestConfigPath(norm.ConfigPath))
	if norm.InitialCash > 0 {
		cfg.InitialCash = norm.InitialCash
	}
	repo := NewRepo()
	allMarket := len(norm.Codes) == 0
	var universeSet map[string]bool
	if !allMarket {
		universeSet = map[string]bool{}
		for _, c := range norm.Codes {
			universeSet[c] = true
		}
	}

	days, err := repo.GetTradeDates(ctx, norm.StartDate, norm.EndDate)
	if err != nil {
		return nil, fmt.Errorf("读取交易日失败: %w", err)
	}
	if len(days) == 0 {
		if bench, berr := LoadPortfolioBenchmarkBars(ctx, norm.StartDate, norm.EndDate); berr == nil {
			for _, b := range bench {
				days = append(days, b.TradeDate)
			}
		}
	}
	if len(days) == 0 {
		return nil, errors.New("回测区间内无本地交易日数据（需要 sh000300 或市场日 K）")
	}
	dayIdx := make(map[string]int, len(days))
	for i, d := range days {
		dayIdx[d] = i
	}
	splits := SplitTradingDays(days, DefaultWarmupTradingDays, DefaultTrailingTradingDays)

	bench, err := LoadPortfolioBenchmarkBars(ctx, days[0], days[len(days)-1])
	if err != nil {
		bench = nil
	}

	universe := "all_market"
	if !allMarket {
		universe = "explicit"
	}
	shared := BacktestInputManifest{
		Codes:       append([]string(nil), norm.Codes...),
		StartDate:   norm.StartDate,
		EndDate:     norm.EndDate,
		CostHash:    CostConfigHash(cfg.CostConfig),
		InitialCash: cfg.InitialCash,
		Universe:    universe,
	}

	sharedLimitations := []string{
		"幸存者偏差：股票清单取自当前本地清单快照，已退市标的无法回补，历史结果可能偏乐观",
	}
	if allMarket {
		sharedLimitations = append(sharedLimitations,
			"回测股票池为本地全市场可研究清单（models.AllStockInfo），不使用当前狩猎池")
	} else {
		sharedLimitations = append(sharedLimitations,
			"回测股票池为显式指定代码，不代表全市场（存在选择偏差）")
	}
	sharedLimitations = append(sharedLimitations,
		"变体差异仅在数据闸门/风险过滤/确认入场三层；四组退出均由同一退出状态机经组合回测引擎回放（成本、滑点、涨跌停、T+1 口径一致）",
		"风险预算股数按初始资金计算（与生产盘中确认一致），实际占用与敞口上限由组合回测引擎逐日复核",
	)

	r := &runner{
		ctx: ctx, repo: repo, cfg: cfg, days: days, dayIdx: dayIdx,
		research: newResearchCache(ctx, repo),
		levels:   &riskLevelCache{ctx: ctx, byDate: map[string]risk.Level{}},
		bars:     newBarCache(ctx, daysBefore(days[0], 400), days[len(days)-1]),
		bench:    bench, universeSet: universeSet,
	}
	if r.bars.start == "" {
		r.bars.start = days[0]
	}

	result := &FullBacktestResult{
		StartDate: norm.StartDate, EndDate: norm.EndDate, AllMarket: allMarket,
		Codes: shared.Codes, CostHash: shared.CostHash, Config: cfg, Splits: splits,
		Limitations: sharedLimitations,
	}
	eventsByVariant := map[BacktestVariant][]ReplayEvent{}
	for _, variant := range norm.Variants {
		vcfg := BuildVariantInput(variant, shared)
		vr, err := r.runVariant(variant, vcfg, splits, sharedLimitations)
		if err != nil {
			return nil, fmt.Errorf("变体 %s 回放失败: %w", variant, err)
		}
		eventsByVariant[variant] = vr.events
		vr.events = nil
		result.Variants = append(result.Variants, vr)
	}
	computeStability(result.Variants)
	result.CaseReplays = r.buildCaseReplays(eventsByVariant)
	return result, nil
}

func backtestConfigPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	return resolveRiskControlConfigPath()
}
