package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/mathutil"
)

// @Author spark
// @Date 2025/6/28 21:02
// @Desc
// -----------------------------------------------------------------------------------
type SearchStockApi struct {
	words string
}

func NewSearchStockApi(words string) *SearchStockApi {
	return &SearchStockApi{words: words}
}
func (s SearchStockApi) SearchStock(pageSize int) map[string]any {
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}

	url := "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	logger.SugaredLogger.Infof("SearchStock 东财 API 原始响应: %s", string(resp.Body()))

	code, _ := respMap["code"].(float64)
	if code != 100 {
		logger.SugaredLogger.Warnf("SearchStock 东财 API 返回 code=%.0f，切换至本地数据库搜索", code)
		return s.searchStockLocalFallback()
	}
	return respMap
}

func (s SearchStockApi) searchStockLocalFallback() map[string]any {
	stockList := NewStockDataApi().GetStockList(s.words)
	if len(stockList) == 0 {
		return map[string]any{
			"code":    -1,
			"message": fmt.Sprintf("未找到与「%s」相关的股票，请尝试其他关键词", s.words),
		}
	}

	var dataList []map[string]any
	for _, item := range stockList {
		marketShort := item.Market
		code := item.TsCode
		if marketShort == "" || strings.EqualFold(marketShort, "SSE") || strings.EqualFold(marketShort, "SZSE") {
			if len(code) >= 2 {
				prefix := code[:2]
				switch prefix {
				case "sh", "SH":
					marketShort = "SH"
				case "sz", "SZ":
					marketShort = "SZ"
				case "bj", "BJ":
					marketShort = "BJ"
				case "hk", "HK":
					marketShort = "HK"
				}
			}
		}
		// 美股: gb_aapl → code=AAPL, market=US
		if strings.HasPrefix(strings.ToLower(code), "gb_") {
			code = strings.ToUpper(code[3:])
			marketShort = "US"
		}

		// 移除 Tushare 格式后缀 .SH/.SZ/.BJ → 仅保留纯代码 (前端 toEastMoneyCode 会重新拼接)
		code = strings.TrimSuffix(code, "."+marketShort)

		dataList = append(dataList, map[string]any{
			"SECURITY_CODE":       code,
			"SECURITY_SHORT_NAME": item.Name,
			"MARKET_SHORT_NAME":   marketShort,
		})
	}

	return map[string]any{
		"code": 100,
		"data": map[string]any{
			"traceInfo": map[string]any{
				"showText": fmt.Sprintf("本地数据库搜索: %s（共%v只）", s.words, len(stockList)),
			},
			"result": map[string]any{
				"columns": []map[string]any{
					{
						"title":    "股票代码",
						"key":      "SECURITY_CODE",
						"minWidth": 120,
					},
					{
						"title":    "股票名称",
						"key":      "SECURITY_SHORT_NAME",
						"minWidth": 150,
					},
					{
						"title":    "所属市场",
						"key":      "MARKET_SHORT_NAME",
						"minWidth": 80,
					},
				},
				"dataList": dataList,
			},
		},
	}
}

func (s SearchStockApi) SearchBk(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/bkc/v3/pw/search-code"
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

func (s SearchStockApi) SearchETF(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/etf/v3/pw/search-code"
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchETF-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

func (s SearchStockApi) HotStrategy() map[string]any {
	url := fmt.Sprintf("https://np-ipick.eastmoney.com/recommend/stock/heat/ranking?count=20&trace=%d&client=web&biz=web_smart_tag", time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-ipick.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotStrategy-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	return respMap
}

func (s SearchStockApi) HotStrategyTable() string {
	markdownTable := ""
	res := s.HotStrategy()
	bytes, _ := json.Marshal(res)
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable = util.MarkdownTableWithTitle("当前热门选股策略", strategy.Data)
	return markdownTable
}

func (s SearchStockApi) StrategySquare() map[string]any {
	//https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword=
	url := "https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword="
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "backtest.10jqka.com.cn").
		SetHeader("Origin", "https://backtest.10jqka.com.cn").
		SetHeader("Referer", "https://backtest.10jqka.com.cn/strategysquare/list").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("StrategySquare-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}
