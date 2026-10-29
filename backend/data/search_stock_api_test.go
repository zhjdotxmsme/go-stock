package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/random"
)

func TestGenerateRequestId(t *testing.T) {
	id := generateRequestId()
	if len(id) != 45 {
		t.Fatalf("requestId 长度应为 45（32随机+13毫秒时间戳），实际 %d: %s", len(id), id)
	}
	for i, c := range id {
		if i < 32 {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
				t.Fatalf("requestId 前 32 位应全为字母数字，第 %d 位为 %q", i, c)
			}
		} else if c < '0' || c > '9' {
			t.Fatalf("requestId 后 13 位应为毫秒时间戳数字，第 %d 位为 %q", i, c)
		}
	}
	// 尾部时间戳与当前时间偏差应在 5 秒内
	ts, err := strconv.ParseInt(id[32:], 10, 64)
	if err != nil {
		t.Fatalf("requestId 尾部时间戳解析失败: %v", err)
	}
	if diff := time.Now().UnixMilli() - ts; diff < -5000 || diff > 5000 {
		t.Fatalf("requestId 时间戳与当前时间偏差 %dms，超出 ±5s", diff)
	}
	// 两次生成应不同（随机性）
	if generateRequestId() == id {
		t.Fatal("两次生成的 requestId 不应相同")
	}
}

func TestSearchStock(t *testing.T) {
	db.Init("../../data/stock.db")

	e := convertor.ToString(math.Floor(float64(9*random.RandFloat(0, 1, 12) + 1)))
	for i := 0; i < 19; i++ {
		e += convertor.ToString(math.Floor(float64(9 * random.RandFloat(0, 1, 12))))
	}
	logger.SugaredLogger.Infof("e:%s", e)

	//res := NewSearchStockApi("量比大于2，基本面优秀，2025年三季报已披露，主力连续3日净流入，非创业板非科创板非ST").SearchStock(20)
	//res := NewSearchStockApi("今日涨幅前5的概念板块").SearchBk(50)
	res := NewSearchStockApi("今日涨幅前15的ETF").SearchETF(50)

	logger.SugaredLogger.Infof("res:%+v", res)
	data := res["data"].(map[string]any)
	result := data["result"].(map[string]any)
	dataList := result["dataList"].([]any)
	columns := result["columns"].([]any)
	headers := map[string]string{}
	for _, v := range columns {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		//logger.SugaredLogger.Infof("key:%s title:%s dateMsg:%s unit:%s", d["key"], d["title"], d["dateMsg"], d["unit"])
		title := convertor.ToString(d["title"])
		if convertor.ToString(d["dateMsg"]) != "" {
			title = title + "[" + convertor.ToString(d["dateMsg"]) + "]"
		}
		if convertor.ToString(d["unit"]) != "" {
			title = title + "(" + convertor.ToString(d["unit"]) + ")"
		}
		headers[d["key"].(string)] = title
	}
	table := &[]map[string]any{}
	for _, v := range dataList {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		tmp := map[string]any{}
		for key, title := range headers {
			//logger.SugaredLogger.Infof("%s:%s", title, convertor.ToString(d[key]))
			tmp[title] = convertor.ToString(d[key])
		}
		*table = append(*table, tmp)
		//logger.SugaredLogger.Infof("--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	}
	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	logger.SugaredLogger.Infof("markdownTable=\n%s", markdownTable)
}

func TestGetStockFinancialInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockFinancialInfo("300390.SZ")
	MD := util.MarkdownTableWithTitle("300390.SZ股票财报信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}
func TestGetStockHolderNum(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockHolderNum("300390.SZ")
	MD := util.MarkdownTableWithTitle("股票股东人数信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}

func TestSearchStockApi_HotStrategy(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").HotStrategy()
	bytes, err := json.Marshal(res)
	if err != nil {
		return
	}
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable := util.MarkdownTable(strategy.Data)
	logger.SugaredLogger.Infof("res:%s", markdownTable)
	//dataList := res["data"].([]any)
	//for _, v := range dataList {
	//	d := v.(map[string]any)
	//	logger.SugaredLogger.Infof("v:%+v", d)
	//}
}
func TestSearchStockApi_HotStrategyTable(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").StrategySquare()
	logger.SugaredLogger.Infof("res:%+v", res)
}
