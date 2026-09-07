// checker.go 实现分析前的数据完整性预检（方案 Step 3.3 / T5）。
// 在并行分析师 goroutine 启动前运行：数据非空、可解析、包含最新交易日、
// 完整率 >= 阈值；不通过时中止分析并给出明确原因，避免用残缺数据喂
// LLM 产出看似完整实则幻觉的报告，浪费 token 与用户时间。
package multi

import (
	"fmt"
	"strconv"
	"time"
)

const (
	// maxKlineStaleDays 最新 K 线允许的最大自然日龄。
	// 需覆盖春节/国庆最长约 11 天的休市间隙（如 2024 春节 2/8→2/19），取 12 天。
	maxKlineStaleDays = 12

	// minCompleteness 四类共享数据（K线/技术指标/财报/资金流历史）的最低可用率。
	minCompleteness = 0.5
)

// DataCheckItem 单项检查结果。
type DataCheckItem struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// DataCheckReport 预检报告。
type DataCheckReport struct {
	Items        []DataCheckItem `json:"items"`
	Passed       bool            `json:"passed"`
	Completeness float64         `json:"completeness"` // 0~1，四类核心数据的可用率
	FailReason   string          `json:"failReason,omitempty"`
}

// Summary 生成一行可读摘要，用于日志。
func (r *DataCheckReport) Summary() string {
	s := fmt.Sprintf("completeness=%.0f%% passed=%v", r.Completeness*100, r.Passed)
	for _, it := range r.Items {
		mark := "ok"
		if !it.OK {
			mark = "FAIL"
		}
		s += fmt.Sprintf(" [%s]%s(%s)", mark, it.Name, it.Detail)
	}
	return s
}

// CheckDataPack 对共享 DataPack 做完整性预检。
// pack 为 nil 时（自定义管线/部分测试，数据由分析师自行获取）直接放行。
//
// 阻断条件（硬失败）：
//   - 日 K 缺失或无一根可解析（价格为分析的最小前提）
//   - 最新 K 线超过 maxKlineStaleDays（数据源过期，多半是代码错误或长期停牌）
//
// 聚合条件（软失败）：四类核心数据可用率 < minCompleteness。
func CheckDataPack(pack *DataPack, now time.Time) *DataCheckReport {
	if pack == nil {
		return &DataCheckReport{Passed: true, Completeness: 1, FailReason: ""}
	}

	rep := &DataCheckReport{Passed: true}
	add := func(name string, ok bool, detail string) {
		rep.Items = append(rep.Items, DataCheckItem{Name: name, OK: ok, Detail: detail})
		if !ok && rep.FailReason == "" {
			rep.FailReason = detail
		}
	}

	// 1) 日 K 存在性
	klineOK := pack.KLineDaily != nil && len(*pack.KLineDaily) > 0
	if klineOK {
		add("kline_daily", true, fmt.Sprintf("%d bars", len(*pack.KLineDaily)))
	} else {
		add("kline_daily", false, "日K线数据缺失，无法进行任何维度分析")
		rep.Passed = false
		rep.Completeness = completenessOf(pack)
		return rep
	}

	// 2) K 线可解析性（Day 非空 + Close 为合法数字）
	var latest time.Time
	parseable := 0
	for _, k := range *pack.KLineDaily {
		if len(k.Day) < 10 {
			continue
		}
		if _, err := strconv.ParseFloat(k.Close, 64); err != nil {
			continue
		}
		parseable++
		if d, err := time.ParseInLocation("2006-01-02", k.Day[:10], time.Local); err == nil && d.After(latest) {
			latest = d
		}
	}
	if parseable == 0 {
		add("kline_parse", false, fmt.Sprintf("%d 根K线全部无法解析（日期/收盘价非法）", len(*pack.KLineDaily)))
		rep.Passed = false
		rep.Completeness = completenessOf(pack)
		return rep
	}
	add("kline_parse", true, fmt.Sprintf("%d/%d 可解析", parseable, len(*pack.KLineDaily)))

	// 3) 时效性：最新 K 线不能太旧（覆盖长假取宽松阈值）
	ageDays := int(now.Sub(latest).Hours() / 24)
	if ageDays > maxKlineStaleDays {
		add("kline_fresh", false, fmt.Sprintf(
			"K线数据过期：最新 %s，距今日 %d 天（阈值 %d 天），可能已长期停牌或数据源异常",
			latest.Format("2006-01-02"), ageDays, maxKlineStaleDays))
		rep.Passed = false
	} else {
		add("kline_fresh", true, fmt.Sprintf("latest=%s age=%dd", latest.Format("2006-01-02"), ageDays))
	}

	// 4) 四类核心数据聚合完整率
	rep.Completeness = completenessOf(pack)
	if rep.Completeness < minCompleteness {
		// 打破常量折叠：直接 int(常量表达式) 会在编译期因精度丢失报错
		minRatio := minCompleteness
		minBars := int(minRatio*4 + 0.5)
		add("completeness", false, fmt.Sprintf(
			"数据完整率 %.0f%% 低于阈值 %.0f%%（K线/技术指标/财报/资金流历史至少 %d 类可用）",
			rep.Completeness*100, minCompleteness*100, minBars))
		rep.Passed = false
	} else {
		add("completeness", true, fmt.Sprintf("%.0f%%", rep.Completeness*100))
	}

	return rep
}

// completenessOf 统计四类核心数据的可用率（0~1）。
func completenessOf(pack *DataPack) float64 {
	available := 0
	if pack.KLineDaily != nil && len(*pack.KLineDaily) > 0 {
		available++
	}
	if pack.TechnicalIndicators != nil {
		available++
	}
	if pack.FinancialReports != nil && len(*pack.FinancialReports) > 0 {
		available++
	}
	if len(pack.HistoryMoneyData) > 0 {
		available++
	}
	return float64(available) / 4
}
