package khunter

import (
	"context"
	"time"

	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// SyncMoneyFlow 逐股拉取东财资金流历史并落库 khunter_money_flow_daily。
// 200ms 间隔限流；单股失败记录日志并跳过。返回成功落库的股票数。
func SyncMoneyFlow(ctx context.Context, codes []string) (int, error) {
	api := data.NewStockDataApi()
	repo := NewRepo()
	ok := 0
	for _, code := range codes {
		select {
		case <-ctx.Done():
			return ok, ctx.Err()
		default:
		}
		his := api.GetStockHistoryMoneyData(code)
		rows := make([]models.KhunterMoneyFlowDaily, 0, len(his))
		for _, h := range his {
			row := models.KhunterMoneyFlowDaily{Code: code, Date: h.Date}
			row.MainNet, _ = convertor.ToFloat(h.F62)
			row.MainRatio, _ = convertor.ToFloat(h.F184)
			row.SuperLgNet, _ = convertor.ToFloat(h.F66)
			row.LgNet, _ = convertor.ToFloat(h.F72)
			row.LgNetRatio, _ = convertor.ToFloat(h.F75)
			row.MdNet, _ = convertor.ToFloat(h.F78)
			row.SmNet, _ = convertor.ToFloat(h.F84)
			row.SmNetRatio, _ = convertor.ToFloat(h.F87)
			if row.Date != "" {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			logger.SugaredLogger.Warnf("khunter 资金流为空: %s", code)
		} else if err := repo.UpsertMoneyFlow(rows); err != nil {
			logger.SugaredLogger.Warnf("khunter 资金流落库失败 %s: %v", code, err)
		} else {
			ok++
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ok, nil
}
