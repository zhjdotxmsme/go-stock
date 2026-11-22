package khunter

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// KronosFactorResult 单只观察的 Kronos 因子结论。
type KronosFactorResult struct {
	ObservationID  uint
	Code           string
	FactorScore    float64 // 0~100，失败时为中性 50
	Weight         float64 // 实际使用权重（cfg.Weight，钳位到 MaxWeight）
	ForecastJSON   string  // 完整预测序列化（KronosPrediction），降级时为空
	DegradedReason string  // 非空表示降级（服务离线/预测失败/结果异常）
}

// KronosFactorScore 预测摘要 → 0~100 因子分，与 daily_pick_kronos.kronosFactorScore 同口径：
// 预期涨幅按 ±10% 截断映射到 0~100（0%→50），再按一致度（Confidence/100）向中性 50 收缩。
// 口径一致的目的是让新链路与历史复盘命中率统计可比。
// 与 daily_pick 唯一差异：nil/无 Summary 返回中性 50（daily_pick 返回 0），
// 因为 khunter 链路的空预测走降级语义（中性分 + DegradedReason），必须可区分于"强看空 0 分"。
func KronosFactorScore(pred *data.KronosPrediction) float64 {
	if pred == nil || pred.Summary == nil {
		return 50
	}
	chg := pred.Summary.ChangePct
	if chg > 10 {
		chg = 10
	} else if chg < -10 {
		chg = -10
	}
	base := (chg + 10) / 20 * 100
	conf := pred.Summary.Confidence / 100
	if conf < 0 {
		conf = 0
	} else if conf > 1 {
		conf = 1
	}
	return math.Round((base*conf+50*(1-conf))*10) / 10
}

// BatchPredictFunc 批量预测注入点：生产传 data.KronosBatchPredict。
type BatchPredictFunc func(ctx context.Context, apiCodes []string, predLen int) (map[string]*data.KronosPrediction, map[string]string, error)

// ApplyKronosFactor 对观察候选按评分降序取 topN，用一次 /predict_batch 批量预测并折算因子分。
// 单只失败不中断批次；服务离线整体降级为中性分且返回 nil error——调用方（流水线）不得被预测故障阻塞。
// 结果按 (Code, SignalDate) 关联观察：SaveObservations 是逐条 upsert，冲突行不回填自增 ID，
// ObservationID 仅供同进程新插入的观察参考，落库以 code+signal_date 为准。
func ApplyKronosFactor(ctx context.Context, observations []models.KhunterObservation,
	cfg KronosFactorConfig, predict BatchPredictFunc,
) ([]KronosFactorResult, error) {
	if !cfg.Enable || predict == nil || len(observations) == 0 {
		return nil, nil
	}
	w := cfg.Weight
	if w > cfg.MaxWeight {
		w = cfg.MaxWeight
	}
	byScore := append([]models.KhunterObservation(nil), observations...)
	sort.SliceStable(byScore, func(i, j int) bool { return byScore[i].Score > byScore[j].Score })
	if len(byScore) > cfg.TopN {
		byScore = byScore[:cfg.TopN]
	}

	// 代码规范化 + 反查表：apiCode → 观察下标
	codes := make([]string, 0, len(byScore))
	idxByCode := make(map[string]int, len(byScore))
	for i, o := range byScore {
		api := data.NormalizeKronosCode(o.Code)
		if api == "" {
			continue
		}
		codes = append(codes, api)
		idxByCode[api] = i
	}
	out := make([]KronosFactorResult, 0, len(byScore))
	if len(codes) == 0 {
		return out, nil
	}

	preds, errs, err := predict(ctx, codes, cfg.PredLen)
	for _, o := range byScore {
		res := KronosFactorResult{ObservationID: o.ID, Code: o.Code, FactorScore: 50, Weight: w}
		api := data.NormalizeKronosCode(o.Code)
		switch {
		case err != nil:
			res.DegradedReason = err.Error()
		case errs[api] != "":
			res.DegradedReason = errs[api]
		case preds[api] == nil || preds[api].Summary == nil:
			res.DegradedReason = "预测结果缺 summary"
		default:
			res.FactorScore = KronosFactorScore(preds[api])
			if b, mErr := json.Marshal(preds[api]); mErr == nil {
				res.ForecastJSON = string(b)
			}
		}
		out = append(out, res)
	}
	return out, nil
}

// buildObsByCode 观察按代码索引（goroutine 内按 code 关联 signal_date）。
func buildObsByCode(rows []models.KhunterObservation) map[string]models.KhunterObservation {
	out := make(map[string]models.KhunterObservation, len(rows))
	for _, o := range rows {
		out[o.Code] = o
	}
	return out
}
