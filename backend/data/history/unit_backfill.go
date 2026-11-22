package history

// K 线单位回填与污染重拉。
//
// 背景：单位声明机制（volume_unit/amount_unit）落地前，旧版二进制写入的行单位为空；
// 且限流降级响应（只有价格、量额为 "-"）曾被整块落库，把同键的有效历史行覆盖成
// 零量额行。数据闸门（ValidateDailyBar）对空单位/零量额一律禁止确认，导致
// 狩猎场确认在生产库上永远无法放行。
//
// 两个一次性（幂等，可重复执行）维护：
//   - BackfillKLineUnits：按量纲恒等式（成交额 ≈ 成交量 × 收盘价，0.5~2.0 倍带
//     即量=股、额=元）与已确认来源（seed/stockdb 导入、mootdx/eastmoney 供应器
//     均为股）回填空单位；带外行保持空单位 fail-closed，不得反推。
//   - RequeuePoisonedSyncTasks：把近期存在零量额 bar 的已完成任务打回待同步，
//     从最早污染日前一天起重拉覆盖（重拉走降级拒绝，只会写入健康数据）。

import (
	"context"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/util/timeutil"
)

// unitRatioBand 恒等式判定带：amount/(volume×close) 落在带内即视为 量=股、额=元。
// 上界放宽到 2.0 覆盖低价股高换手日；带外的行不回填。
const unitRatioBandLow, unitRatioBandHigh = 0.5, 2.0

// BackfillKLineUnits 幂等回填 day 周期空单位行，返回回填行数。
func BackfillKLineUnits(ctx context.Context) (int64, error) {
	// R1 恒等式证明（不限来源）：比值带内 → 量=股、额=元。
	res := db.Dao.WithContext(ctx).Exec(`
UPDATE kline_bars SET volume_unit = 'share', amount_unit = 'CNY'
WHERE period = 'day' AND IFNULL(volume_unit, '') = ''
  AND volume > 0 AND amount > 0 AND close > 0
  AND amount / (volume * close) BETWEEN ? AND ?`, unitRatioBandLow, unitRatioBandHigh)
	if res.Error != nil {
		return 0, res.Error
	}
	filled := res.RowsAffected

	// R2 已确认来源（量=股）：量>0 但额=0 或价=0（比值不可判定）的行只补量单位；
	//    额单位保持空，由数据闸门对 amount<=0 的既有拒绝兜底。
	//    比值带 ≥50（疑似"手"量纲误标的导入行）保持空单位 fail-closed，不得反推。
	res2 := db.Dao.WithContext(ctx).Exec(`
UPDATE kline_bars SET volume_unit = 'share'
WHERE period = 'day' AND IFNULL(volume_unit, '') = '' AND volume > 0
  AND source IN ('seed', 'stockdb', 'mootdx_kline', 'eastmoney_kline')
  AND (amount <= 0 OR close <= 0 OR amount / (volume * close) < 50.0)`)
	if res2.Error != nil {
		return filled, res2.Error
	}
	return filled + res2.RowsAffected, nil
}

// RequeuePoisonedSyncTasks 把 since 以来存在零量额/未声明单位 sync 行的已完成任务
// 打回待同步，断点回拨到最早污染日前一天（重拉覆盖整段）。返回重排任务数。
func RequeuePoisonedSyncTasks(ctx context.Context, since string) (int64, error) {
	res := db.Dao.WithContext(ctx).Exec(`
UPDATE kline_sync_log SET
	status = 'pending',
	error_msg = '零量额/未声明单位行重拉',
	last_sync_date = (SELECT date(MIN(b.trade_date), '-1 day') FROM kline_bars b
		WHERE b.stock_code = kline_sync_log.stock_code AND b.period = 'day'
		  AND b.adjusted = 1 AND b.source = 'sync'
		  AND IFNULL(b.volume_unit, '') IN ('', 'unknown')
		  AND (b.volume <= 0 OR b.amount <= 0) AND b.trade_date >= ?),
	end_date = CASE WHEN end_date < ? THEN ? ELSE end_date END,
	updated_at = CURRENT_TIMESTAMP
WHERE status = 'done' AND period = 'day'
  AND EXISTS (
	SELECT 1 FROM kline_bars b
	WHERE b.stock_code = kline_sync_log.stock_code AND b.period = 'day'
	  AND b.adjusted = 1 AND b.source = 'sync'
	  AND IFNULL(b.volume_unit, '') IN ('', 'unknown')
	  AND (b.volume <= 0 OR b.amount <= 0) AND b.trade_date >= ?)`, since, timeutil.TodayStr(), timeutil.TodayStr(), since)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// RunUnitBackfillMaintenance 启动维护入口：回填单位 + 重拉污染行（均幂等）。
func RunUnitBackfillMaintenance(ctx context.Context) {
	n, err := BackfillKLineUnits(ctx)
	if err != nil {
		logger.SugaredLogger.Warnf("K线单位回填失败: %v", err)
	} else if n > 0 {
		logger.SugaredLogger.Infof("K线单位回填：补写 %d 行 volume/amount 单位", n)
	}
	poisoned, err := RequeuePoisonedSyncTasks(ctx, timeutil.DaysAgoStr(30))
	if err != nil {
		logger.SugaredLogger.Warnf("污染行重拉排队失败: %v", err)
		return
	}
	if poisoned > 0 {
		logger.SugaredLogger.Infof("K线污染重拉：%d 条 done 任务存在零量额行，已回拨断点重拉近 30 天", poisoned)
	}
}
