package khunter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/data"
	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// FundamentalFetch 一次基本面取数结果：评分输入 + 所属报告期
type FundamentalFetch struct {
	Input        scorer.FundamentalInput
	ReportPeriod string // 报告期（REPORT_DATE 的日期部分，格式 2006-01-02；取不到为空）
}

func fetchFundamentalOnce(code string) FundamentalFetch {
	out := FundamentalFetch{}
	resp, err := data.NewStockDataApi().GetStockLatestFinance(code)
	if err != nil || resp == nil || resp.Result == nil || len(resp.Result.Data) == 0 {
		return out
	}
	m := resp.Result.Data[0]
	out.Input.Available = true
	if s, ok := m["REPORT_DATE"].(string); ok {
		out.ReportPeriod = strings.Split(s, " ")[0]
	}
	if v, err := convertor.ToFloat(m["PARENTNETPROFITTZ"]); err == nil {
		out.Input.NetProfitYoy, out.Input.HasYoy = v, true
	}
	if v, err := convertor.ToFloat(m["ROEJQ"]); err == nil {
		out.Input.ROE, out.Input.HasROE = v, true
	}
	// 经营现金流/营收 ≈ 每股经营现金流 / (营业总收入/总股本)
	ocf, err1 := convertor.ToFloat(m["MGJYXJJE"])
	income, err2 := convertor.ToFloat(m["TOTAL_OPERATEINCOME"])
	shares, err3 := convertor.ToFloat(m["TOTAL_SHARE"])
	if err1 == nil && err2 == nil && err3 == nil && shares > 0 && income > 0 {
		out.Input.OcfToIncome, out.Input.HasOcf = ocf/(income/shares), true
	}
	return out
}

// FetchFundamental 从东财 F10 最新主财务指标提取评分输入；失败返回 Available=false
func FetchFundamental(code string) scorer.FundamentalInput {
	return fetchFundamentalOnce(code).Input
}

// FetchFundamentalRetryDetail 带一次重试取基本面并附带报告期：流水线评分时段偶发网络失败
// 会导致基本面整维退化 50 分（生产实测），重试一次把瞬时失败降下来。
func FetchFundamentalRetryDetail(code string, retries int) FundamentalFetch {
	out := fetchFundamentalOnce(code)
	for i := 0; i < retries && !out.Input.Available; i++ {
		out = fetchFundamentalOnce(code)
	}
	return out
}

// FetchFundamentalRetry 带一次重试：只取评分输入（兼容旧入口）。
func FetchFundamentalRetry(code string, retries int) scorer.FundamentalInput {
	return FetchFundamentalRetryDetail(code, retries).Input
}

// FetchAndSnapshotFundamental 实时模式专用：取最新基本面先落时点快照再返回评分输入。
// 快照可用日不得早于取数时刻，否则历史研究会读到未来数据。落库失败只记日志，不阻断评分。
func FetchAndSnapshotFundamental(repo *Repo, code, availableDate string) scorer.FundamentalInput {
	f := FetchFundamentalRetryDetail(code, 1)
	payload, err := json.Marshal(f.Input)
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 基本面快照序列化失败 %s: %v", code, err)
		return f.Input
	}
	row := models.KhunterFundamentalSnapshot{
		Code: code, AvailableDate: availableDate, ReportPeriod: f.ReportPeriod,
		Payload: string(payload),
	}
	if err := repo.UpsertFundamentalSnapshot(&row); err != nil {
		logger.SugaredLogger.Warnf("khunter 基本面快照落库失败 %s@%s: %v", code, availableDate, err)
	}
	return f.Input
}

// loadFundamentalAsOf 研究模式读取时点基本面快照并解码；
// 缺快照（或快照记录当时取数失败）返回 ErrSnapshotUnavailable，禁止调用最新接口补缺。
func loadFundamentalAsOf(repo *Repo, code, asOfDate string) (scorer.FundamentalInput, error) {
	row, err := repo.GetFundamentalSnapshotAsOf(code, asOfDate)
	if err != nil {
		return scorer.FundamentalInput{}, err
	}
	var in scorer.FundamentalInput
	if err := json.Unmarshal([]byte(row.Payload), &in); err != nil {
		return scorer.FundamentalInput{}, fmt.Errorf("基本面快照解析失败 %s@%s: %w", code, row.AvailableDate, err)
	}
	if !in.Available {
		return scorer.FundamentalInput{}, fmt.Errorf("%w: %s@%s 快照记录当时取数失败", ErrSnapshotUnavailable, code, row.AvailableDate)
	}
	return in, nil
}
