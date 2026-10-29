package data

import (
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// data_cleanup.go — 历史价值低、只增不减的大表清理统一入口。
//
// 这些表由高频采集任务写入，页面与工具只用到最近若干天。
//
// 清理只挂在凌晨 cron 上并不可靠：桌面应用夜里通常没有运行，cron 就不会执行，
// 表会一直膨胀（上游实测 concept_fund_flow 积累 180 万行、跨 3 个月）。
// 因此启动后也会补跑一次，与凌晨低峰清理形成双保险。
//
// 保留策略：资金流向快照 3 天；快讯与标签、词频/情感分析结果 30 天。

// deleteOldRowsInBatches 分批删除大表的历史数据。
// 单条 DELETE 一次删掉上百万行会长时间持有写锁并产生超大 WAL，期间其他读写全部卡住；
// 这里限制每批行数，批间让出短暂间隔，把阻塞摊平。
// table/column 均为代码内常量，不涉及外部输入。
func deleteOldRowsInBatches(table, column, cutoff string) int64 {
	const batchSize = 20000
	var total int64
	for {
		res := db.Dao.Exec(fmt.Sprintf(
			"DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE %s < ? LIMIT %d)",
			table, table, column, batchSize), cutoff)
		if res.Error != nil {
			logger.SugaredLogger.Errorf("clean %s error: %v", table, res.Error)
			break
		}
		total += res.RowsAffected
		if res.RowsAffected == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return total
}

// CleanupHistoryData 清理入口：启动补跑与凌晨低峰 cron 共用。
func CleanupHistoryData() {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("CleanupHistoryData panic: %v", r)
		}
	}()
	if db.Dao == nil {
		return
	}
	bk := NewBKFundFlowApi().CleanOldData(3)
	concept := NewConceptFundFlowApi().CleanOldData(3)
	telegraph, links := NewMarketNewsApi().CleanOldTelegraph(30)
	words, sentiments := CleanOldSentimentAnalyzes(30)
	logger.SugaredLogger.Infof(
		"历史数据清理完成：bk_fund_flow=%d, concept_fund_flow=%d, telegraph=%d(标签%d), word_analyzes=%d, sentiment_result_analyzes=%d",
		bk, concept, telegraph, links, words, sentiments)
}

// CleanOldSentimentAnalyzes 清理只写不读的词频/情感分析结果表。
func CleanOldSentimentAnalyzes(days int) (int64, int64) {
	if days <= 0 || db.Dao == nil {
		return 0, 0
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	words := db.Dao.Unscoped().Where("data_time < ?", cutoff).Delete(&models.WordAnalyze{})
	if words.Error != nil {
		logger.SugaredLogger.Warnf("清理 word_analyzes 失败:%v", words.Error)
	}
	sentiments := db.Dao.Unscoped().Where("data_time < ?", cutoff).Delete(&models.SentimentResultAnalyze{})
	if sentiments.Error != nil {
		logger.SugaredLogger.Warnf("清理 sentiment_result_analyzes 失败:%v", sentiments.Error)
	}
	return words.RowsAffected, sentiments.RowsAffected
}
