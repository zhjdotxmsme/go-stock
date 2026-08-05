package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 本文件为应用启动生命周期相关逻辑，从 app.go 纯搬运拆分。

// registerStartupCronTasks 启动时注册各类内置定时任务
// （价格/新闻/基金净值/AI 推荐与成本价监控/市场统计/资金流向/自选股自动分析/daily_pick 等），
// 由 domReady 在基础数据初始化后调用。
func (a *App) registerStartupCronTasks() {
	// Add your action here
	//定时更新数据
	config := data.GetSettingConfig()
	go func() {
		go data.NewMarketNewsApi().TelegraphList(30)
		go data.NewMarketNewsApi().GetSinaNews(30)
		go data.NewMarketNewsApi().TradingViewNews()

		interval := config.RefreshInterval
		if interval <= 0 {
			interval = 1
		}
		//ticker := time.NewTicker(time.Second * time.Duration(interval))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorStockPrices(a)
		//}
		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
			MonitorStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorStockPrices", id)
		}
		entryID, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			//news := data.NewMarketNewsApi().GetNewTelegraph(30)
			news := data.NewMarketNewsApi().TelegraphList(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newTelegraph", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("GetNewTelegraph", entryID)
		}

		entryIDSina, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().GetSinaNews(30)
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "newSinaNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("newSinaNews", entryIDSina)
		}

		entryIDTradingViewNews, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", interval+10), func() {
			news := data.NewMarketNewsApi().TradingViewNews()
			if data.GetSettingConfig().EnablePushNews {
				go a.NewsPush(news)
			}
			go runtime.EventsEmit(a.ctx, "tradingViewNews", news)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("tradingViewNews", entryIDTradingViewNews)
		}
	}()

	//刷新基金净值信息
	go func() {
		//ticker := time.NewTicker(time.Second * time.Duration(60))
		//defer ticker.Stop()
		//for range ticker.C {
		//	MonitorFundPrices(a)
		//}
		if config.EnableFund {
			id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
				MonitorFundPrices(a)
			})
			if err != nil {
				logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
			} else {
				a.setCronEntry("MonitorFundPrices", id)
			}
		}

		// AI 推荐股票价格监控定时器
		idAiStock, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorAiRecommendStockPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorAiRecommendStockPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorAiRecommendStockPrices", idAiStock)
		}

		// 自选股成本价监控定时器
		idCostPrice, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			MonitorFollowedStockCostPrices(a)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc MonitorFollowedStockCostPrices error:%s", err.Error())
		} else {
			a.setCronEntry("MonitorFollowedStockCostPrices", idCostPrice)
		}

	}()

	if config.EnableNews {
		//go func() {
		//	ticker := time.NewTicker(time.Second * time.Duration(60))
		//	defer ticker.Stop()
		//	for range ticker.C {
		//		telegraph := refreshTelegraphList()
		//		if telegraph != nil {
		//			go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
		//		}
		//	}
		//
		//}()

		id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", 60), func() {
			telegraph := refreshTelegraphList()
			if telegraph != nil {
				go runtime.EventsEmit(a.ctx, "telegraph", telegraph)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc error:%s", err.Error())
		} else {
			a.setCronEntry("refreshTelegraphList", id)
		}

		go runtime.EventsEmit(a.ctx, "telegraph", refreshTelegraphList())
	}
	go MonitorStockPrices(a)
	if config.EnableFund {
		go MonitorFundPrices(a)
		go data.NewFundApi().AllFund()
	}
	// AI 推荐股票价格监控
	go MonitorAiRecommendStockPrices(a)
	// 自选股成本价监控
	go MonitorFollowedStockCostPrices(a)
	// 市场统计数据采集（交易日每5分钟）
	go func() {
		a.FetchAndSaveMarketStatistic()
		idMarketStat, err := a.cron.AddFunc("0 */5 9-15 * * 1-5", func() {
			a.FetchAndSaveMarketStatistic()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc FetchAndSaveMarketStatistic error:%s", err.Error())
		} else {
			a.setCronEntry("FetchAndSaveMarketStatistic", idMarketStat)
		}
	}()
	// 板块资金流向数据采集（交易日每60秒）
	go func() {
		data.NewBKFundFlowApi().FetchAndSave()
		idBKFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewBKFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc BKFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("BKFundFlowFetchAndSave", idBKFundFlow)
		}
	}()
	// 概念资金流向数据采集（交易日每60秒）
	go func() {
		data.NewConceptFundFlowApi().FetchAndSave()
		idConceptFundFlow, err := a.cron.AddFunc("@every 60s", func() {
			if a.IsTradingTime() {
				data.NewConceptFundFlowApi().FetchAndSave()
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("AddFunc ConceptFundFlowFetchAndSave error:%s", err.Error())
		} else {
			a.setCronEntry("ConceptFundFlowFetchAndSave", idConceptFundFlow)
		}
	}()
	// 启动时同步一次 all_stock_info（用于 daily_pick 和增强搜索），
	// 并在交易时段定时刷新。
	go func() {
		time.Sleep(10 * time.Second) // 等 DB 和基础数据初始化完成
		logger.SugaredLogger.Info("syncAllStockInfo: initial sync on startup")
		syncAllStockInfo(a.ctx)
	}()
	a.cron.AddFunc("0 30 10,14,20 * * 1-5", func() {
		logger.SugaredLogger.Info("syncAllStockInfo: cron sync")
		syncAllStockInfo(a.ctx)
	})

	//检查谷歌浏览器
	//go func() {
	//	f := checkChromeOnWindows()
	//	if !f {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "谷歌浏览器未安装,ai分析功能可能无法使用")
	//	}
	//}()

	//检查Edge浏览器
	//go func() {
	//	path, e := checkEdgeOnWindows()
	//	if !e {
	//		go runtime.EventsEmit(a.ctx, "warnMsg", "Edge浏览器未安装,ai分析功能可能无法使用")
	//	} else {
	//		logger.SugaredLogger.Infof("Edge浏览器已安装，路径为: %s", path)
	//	}
	//}()
	followList := data.NewStockDataApi().GetFollowList(0)
	for _, follow := range *followList {
		if follow.Cron == nil || *follow.Cron == "" {
			continue
		}
		entryID, err := a.cron.AddFunc(*follow.Cron, a.AddCronTask(follow))
		if err != nil {
			logger.SugaredLogger.Errorf("添加自动分析任务失败:%s cron=%s entryID:%v", follow.Name, *follow.Cron, entryID)
			continue
		}
		a.setCronEntry(follow.StockCode, entryID)
	}
	//logger.SugaredLogger.Infof("domReady-cronEntrys:%+v", a.cronEntrys)

	// ---- Daily Pick cron tasks ----
	// Auto-run daily pick after market close (15:30 weekdays)
	dailyPickSvc := data.NewDailyPickService()
	pickEntryID, err := a.cron.AddFunc("0 30 15 * * 1-5", func() {
		today := time.Now().Format("2006-01-02")
		logger.SugaredLogger.Infof("daily_pick: auto-run for %s", today)
		if _, err := dailyPickSvc.RunDailyPick(today, 5); err != nil {
			logger.SugaredLogger.Errorf("daily_pick: auto-run failed: %v", err)
		}
	})
	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: add auto-run cron failed: %v", err)
	} else {
		a.setCronEntry("dailyPickAutoRun", pickEntryID)
	}

	// Auto-review picks before market open (09:15 weekdays)
	reviewEntryID, err := a.cron.AddFunc("0 15 9 * * 1-5", func() {
		today := time.Now().Format("2006-01-02")
		logger.SugaredLogger.Infof("daily_pick: auto-review for %s", today)
		dailyPickSvc.RunDailyReview(today, "")
	})
	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: add auto-review cron failed: %v", err)
	} else {
		a.setCronEntry("dailyPickAutoReview", reviewEntryID)
	}

	// Check for unreviewed picks on startup
	go func() {
		time.Sleep(5 * time.Second) // wait for DB init
		today := time.Now().Format("2006-01-02")
		// Only auto-review on weekdays if market hasn't opened yet
		if isTradingDay(time.Now()) && !isTradingTime(time.Now()) {
			logger.SugaredLogger.Info("daily_pick: startup review check")
			dailyPickSvc.RunDailyReview(today, "")
		}
	}()
}

func syncAllStockInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()

	// Fetch all pages first before deleting, so a partial API failure doesn't wipe the table.
	var allDatas []models.AllStockInfo
	for page := 1; page < 3; page++ {
		res := data.NewStockDataApi().GetAllStocks(page, 3000, "", models.TechnicalIndicators{})
		if res == nil {
			logger.SugaredLogger.Errorf("syncAllStockInfo: GetAllStocks page %d returned nil", page)
			return
		}
		for _, data := range res.Result.Data {
			allDatas = append(allDatas, data.ToAllStockInfo())
		}
	}
	if len(allDatas) == 0 {
		logger.SugaredLogger.Warn("syncAllStockInfo: East Money API returned no data, fallback to StockBasic table")
		var basics []data.StockBasic
		db.Dao.Model(&data.StockBasic{}).Find(&basics)
		for _, b := range basics {
			if !strings.HasSuffix(b.TsCode, ".SH") && !strings.HasSuffix(b.TsCode, ".SZ") && !strings.HasSuffix(b.TsCode, ".BJ") {
				continue
			}
			allDatas = append(allDatas, models.AllStockInfo{
				SECUCODE:         b.TsCode,
				SECURITYCODE:     b.Symbol,
				SECURITYNAMEABBR: b.Name,
			})
		}
		if len(allDatas) == 0 {
			logger.SugaredLogger.Error("syncAllStockInfo: StockBasic table also empty, giving up")
			return
		}
		logger.SugaredLogger.Infof("syncAllStockInfo: fallback generated %d records from StockBasic", len(allDatas))
	}

	logger.SugaredLogger.Infof("syncAllStockInfo: fetched %d records, replacing all_stock_info table", len(allDatas))
	db.Dao.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{})
	err := db.Dao.CreateInBatches(&allDatas, 1000).Error
	if err != nil {
		logger.SugaredLogger.Errorf("syncAllStockInfo: CreateInBatches error: %s", err.Error())
	}
}
func (a *App) CheckStockBaseInfo(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		go runtime.EventsEmit(ctx, "loadingMsg", "done")
	}()
	stockBasics := &[]data.StockBasic{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_basic.json")

	db.Dao.Unscoped().Model(&data.StockBasic{}).Where("1=1").Delete(&data.StockBasic{})
	err := db.Dao.CreateInBatches(stockBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockBasic股票基础信息失败:%s", err.Error())
	}

	//count := int64(0)
	//db.Dao.Model(&data.StockBasic{}).Count(&count)
	//if count == int64(len(*stockBasics)) {
	//	return
	//}
	//for _, stock := range *stockBasics {
	//	stockInfo := &data.StockBasic{
	//		TsCode: stock.TsCode,
	//		Name:   stock.Name,
	//		Symbol: stock.Symbol,
	//		BKCode: stock.BKCode,
	//		BKName: stock.BKName,
	//	}
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&data.StockBasic{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Updates(stockInfo)
	//	}
	//}

	stockHKBasics := &[]models.StockInfoHK{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockHKBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_hk.json")

	db.Dao.Unscoped().Model(&models.StockInfoHK{}).Where("1=1").Delete(&models.StockInfoHK{})
	err = db.Dao.CreateInBatches(stockHKBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoHK股票基础信息失败:%s", err.Error())
	}

	//for _, stock := range *stockHKBasics {
	//	stockInfo := &models.StockInfoHK{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}
	stockUSBasics := &[]models.StockInfoUS{}
	data.SharedHTTPClient.R().
		SetHeader("user", "go-stock").
		SetResult(stockUSBasics).
		Get("http://8.134.249.145:18080/go-stock/stock_base_info_us.json")

	db.Dao.Unscoped().Model(&models.StockInfoUS{}).Where("1=1").Delete(&models.StockInfoUS{})
	err = db.Dao.CreateInBatches(stockUSBasics, 400).Error
	if err != nil {
		logger.SugaredLogger.Errorf("保存StockInfoUS股票基础信息失败:%s", err.Error())
	}
	//for _, stock := range *stockUSBasics {
	//	stockInfo := &models.StockInfoUS{
	//		Code:   stock.Code,
	//		Name:   stock.Name,
	//		BKName: stock.BKName,
	//		BKCode: stock.BKCode,
	//	}
	//	db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).First(stockInfo)
	//	if stockInfo.ID == 0 {
	//		db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo)
	//	} else {
	//		db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", stock.Code).Updates(stockInfo)
	//	}
	//}

}

func refreshTelegraphList() *[]string {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	response, err := data.SharedHTTPClient.R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		Get(clsURL)
	if err != nil {
		return &[]string{}
	}
	res := map[string]any{}
	if err := json.Unmarshal(response.Body(), &res); err != nil {
		return &[]string{}
	}
	var telegraph []string
	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return &[]string{}
		}
		dataMap, ok := res["data"].(map[string]any)
		if !ok {
			return &[]string{}
		}
		rollData, ok := dataMap["roll_data"].([]any)
		if !ok {
			return &[]string{}
		}
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			content, _ := news["content"].(string)
			if content != "" {
				telegraph = append(telegraph, content)
			}
		}
	}
	return &telegraph
}
