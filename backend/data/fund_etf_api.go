package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"gorm.io/gorm"
)

// 场内基金（ETF/LOF）专属数据层：
//   - 真实单位净值来自东财 FundMNFInfo（NAV/PDATE），替代「K线收盘价冒充净值」的旧逻辑
//   - 溢价率自行计算：(市价 − 净值) / 净值 × 100，正=溢价、负=折价
//   - 总份额/换手率来自东财 push2 行情（f84 总份额、f168 换手率）
//   - 份额按天落 fund_share_history 快照，作为「资金流向代理」供 AI 分析使用
//
// 数据口径说明：QDII 类 ETF（如纳指）净值滞后一个交易日，溢价率相对最新公布净值计算，
// 与集思录等主流行情站口径一致。

// FundShareHistory 基金份额日度快照（仅场内基金）。
type FundShareHistory struct {
	gorm.Model
	Code        string   `json:"code" gorm:"index"`
	Date        string   `json:"date"` // 2006-01-02
	Shares      float64  `json:"shares"` // 总份额（份）
	PremiumRate *float64 `json:"premiumRate"`
}

func (FundShareHistory) TableName() string {
	return "fund_share_history"
}

// EtfQuoteSnapshot 场内基金实时参考数据快照，供多智能体分析等消费方使用。
type EtfQuoteSnapshot struct {
	Code         string             `json:"code"`
	Name         string             `json:"name"`
	Price        float64            `json:"price"`      // 最新市价
	Nav          float64            `json:"nav"`        // 最新公布单位净值
	NavDate      string             `json:"navDate"`    // 净值日期
	QuoteTime    string             `json:"quoteTime"`  // 行情时间
	PremiumRate  *float64           `json:"premiumRate"` // 溢价率%（正=溢价，负=折价）
	Shares       *float64           `json:"shares"`      // 总份额（份）
	SharesTrend  []FundShareHistory `json:"sharesTrend"` // 近 N 日份额快照（日期升序）
	TurnoverRate float64            `json:"turnoverRate"` // 换手率%
}

// calcEtfPremiumRate 溢价率 = (市价 − 净值) / 净值 × 100，任一值非法返回 nil。
func calcEtfPremiumRate(price, nav float64) *float64 {
	if price <= 0 || nav <= 0 {
		return nil
	}
	p := mathutil.RoundToFloat((price-nav)/nav*100, 2)
	return &p
}

// fetchEtfMobileInfo 调东财基金移动端接口取场内基金实时信息（真净值 + 最新价）。
func (f *FundApi) fetchEtfMobileInfo(code string) (*MobileFundInfo, error) {
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		Get(fundMobileInfoURL(code))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("FundMNFInfo http %d", resp.StatusCode())
	}
	var result MobileAPIResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}
	if !result.Success || len(result.Datas) == 0 {
		return nil, fmt.Errorf("FundMNFInfo empty datas")
	}
	return &result.Datas[0], nil
}

// fetchEtfSharesViaPush2 调东财 push2 行情取基金总份额（f84）与换手率（f168）。
func (f *FundApi) fetchEtfSharesViaPush2(code string) (shares float64, turnoverRate float64) {
	secid := fundKLineSecid(code)
	if secid == "" {
		return 0, 0
	}
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&invt=2&fltt=2&fields=f43,f84,f116,f168", secid)
	resp, err := f.client.SetTimeout(time.Duration(f.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return 0, 0
	}
	// 部分字段可能返回 "-" 字符串，用 map + convertor 容错解析
	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil || result.Data == nil {
		return 0, 0
	}
	shares, _ = convertor.ToFloat(result.Data["f84"])
	turnoverRate, _ = convertor.ToFloat(result.Data["f168"])
	return shares, turnoverRate
}

// fetchEtfQuoteLive 实时抓取场内基金参考数据（2 个 HTTP 请求），不落库。
func (f *FundApi) fetchEtfQuoteLive(code string) *EtfQuoteSnapshot {
	info, err := f.fetchEtfMobileInfo(code)
	if err != nil {
		logger.SugaredLogger.Warnf("fetchEtfQuoteLive %s FundMNFInfo failed: %v", code, err)
		return nil
	}
	snap := &EtfQuoteSnapshot{
		Code:      code,
		Name:      info.SHORTNAME,
		QuoteTime: info.HQDATE,
	}
	snap.Price, _ = convertor.ToFloat(info.NEWPRICE)
	snap.Nav, _ = convertor.ToFloat(info.NAV)
	snap.NavDate = info.PDATE
	snap.PremiumRate = calcEtfPremiumRate(snap.Price, snap.Nav)
	if shares, turnover := f.fetchEtfSharesViaPush2(code); shares > 0 {
		snap.Shares = &shares
		snap.TurnoverRate = turnover
	}
	return snap
}

// crawlEtfQuoteExtra 抓取并落库场内基金的单位净值/溢价率/份额，同时写份额日快照。
// 监控循环（CrawlFundNetUnitValue 的场内分支）与关注列表批量刷新都会走到这里。
func (f *FundApi) crawlEtfQuoteExtra(code string) {
	if !IsOnExchangeFund(code) {
		return
	}
	snap := f.fetchEtfQuoteLive(code)
	if snap == nil {
		return
	}
	fund := &FollowedFund{Code: code}
	if snap.Nav > 0 {
		fund.NetUnitValue = &snap.Nav
		fund.NetUnitValueDate = snap.NavDate
	}
	fund.PremiumRate = snap.PremiumRate
	fund.Shares = snap.Shares
	db.Dao.Model(fund).Where("code=?", fund.Code).Updates(fund)

	if snap.Shares != nil && *snap.Shares > 0 {
		SaveFundShareDaily(code, time.Now().Format("2006-01-02"), *snap.Shares, snap.PremiumRate)
	}
}

// GetEtfQuoteSnapshot 供 AI 分析使用：监控循环已刷新（本地数据为今日且含溢价率）时
// 直接读库，否则实时抓取一次；份额趋势始终读本地快照。
func (f *FundApi) GetEtfQuoteSnapshot(code string) *EtfQuoteSnapshot {
	code = PureFundCode(code)
	today := time.Now().Format("2006-01-02")

	var fund FollowedFund
	if err := db.Dao.Model(&FollowedFund{}).Where("code=?", code).First(&fund).Error; err == nil {
		if fund.PremiumRate != nil && strings.Contains(fund.NetEstimatedTime, today) {
			snap := &EtfQuoteSnapshot{
				Code:        code,
				Name:        fund.Name,
				Price:       derefFloat(fund.NetEstimatedUnit),
				Nav:         derefFloat(fund.NetUnitValue),
				NavDate:     fund.NetUnitValueDate,
				QuoteTime:   fund.NetEstimatedTime,
				PremiumRate: fund.PremiumRate,
				Shares:      fund.Shares,
			}
			if snap.Nav > 0 && snap.Price > 0 {
				snap.SharesTrend = GetFundShareTrend(code, 20)
				return snap
			}
		}
	}

	snap := f.fetchEtfQuoteLive(code)
	if snap == nil {
		return nil
	}
	snap.SharesTrend = GetFundShareTrend(code, 20)
	return snap
}

// SaveFundShareDaily 份额日度快照按 (code, date) 幂等 upsert。
func SaveFundShareDaily(code, date string, shares float64, premium *float64) {
	var count int64
	db.Dao.Model(&FundShareHistory{}).Where("code=? AND date=?", code, date).Count(&count)
	if count > 0 {
		db.Dao.Model(&FundShareHistory{}).Where("code=? AND date=?", code, date).
			Updates(map[string]interface{}{"shares": shares, "premium_rate": premium})
		return
	}
	db.Dao.Create(&FundShareHistory{Code: code, Date: date, Shares: shares, PremiumRate: premium})
}

// GetFundShareTrend 近 N 日份额快照，返回按日期升序。
func GetFundShareTrend(code string, limit int) []FundShareHistory {
	if limit <= 0 {
		limit = 20
	}
	var rows []FundShareHistory
	if err := db.Dao.Model(&FundShareHistory{}).Where("code=?", code).
		Order("date DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
