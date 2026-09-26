package khunter

import (
	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/data"
	"go-stock/backend/data/khunter/scorer"
)

// FetchFundamental 从东财 F10 最新主财务指标提取评分输入；失败返回 Available=false
func FetchFundamental(code string) scorer.FundamentalInput {
	in := scorer.FundamentalInput{}
	resp, err := data.NewStockDataApi().GetStockLatestFinance(code)
	if err != nil || resp == nil || resp.Result == nil || len(resp.Result.Data) == 0 {
		return in
	}
	m := resp.Result.Data[0]
	in.Available = true
	if v, err := convertor.ToFloat(m["PARENTNETPROFITTZ"]); err == nil {
		in.NetProfitYoy, in.HasYoy = v, true
	}
	if v, err := convertor.ToFloat(m["ROEJQ"]); err == nil {
		in.ROE, in.HasROE = v, true
	}
	// 经营现金流/营收 ≈ 每股经营现金流 / (营业总收入/总股本)
	ocf, err1 := convertor.ToFloat(m["MGJYXJJE"])
	income, err2 := convertor.ToFloat(m["TOTAL_OPERATEINCOME"])
	shares, err3 := convertor.ToFloat(m["TOTAL_SHARE"])
	if err1 == nil && err2 == nil && err3 == nil && shares > 0 && income > 0 {
		in.OcfToIncome, in.HasOcf = ocf/(income/shares), true
	}
	return in
}
