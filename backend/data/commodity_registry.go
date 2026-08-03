package data

import "go-stock/backend/models"

// CommodityRegistry 商品品种注册表
var CommodityRegistry = []models.CommodityAsset{
	// ── 贵金属 · 国际现货 (通过 Yahoo Finance / AURUM Rates / WallStreetCN) ──
	{Code: "XAUUSD", Name: "现货黄金(纽约)", AssetType: models.AssetSpot, Category: models.CategoryPreciousMetal, Exchange: "OTC", Symbol: "XAUUSD.OTC", InternationalRef: "GC=F", IsTradable: true},
	{Code: "XAGUSD", Name: "现货白银(纽约)", AssetType: models.AssetSpot, Category: models.CategoryPreciousMetal, Exchange: "OTC", Symbol: "XAGUSD.OTC", InternationalRef: "SI=F", IsTradable: true},
	{Code: "XAU", Name: "现货黄金(伦敦)", AssetType: models.AssetSpot, Category: models.CategoryPreciousMetal, Exchange: "LBMA", Symbol: "XAU.OTC", InternationalRef: "GC=F", IsTradable: true},
	{Code: "XAG", Name: "现货白银(伦敦)", AssetType: models.AssetSpot, Category: models.CategoryPreciousMetal, Exchange: "LBMA", Symbol: "XAG.OTC", InternationalRef: "SI=F", IsTradable: true},

	// ── 贵金属 · 国内期货 (通过 EastMoney push2 / Sina) ──
	{Code: "AU", Name: "沪金(上海)", AssetType: models.AssetFutures, Category: models.CategoryPreciousMetal, Exchange: "SHFE", Symbol: "113.AU0", InternationalRef: "GC=F", IsTradable: true},
	{Code: "AG", Name: "沪银(上海)", AssetType: models.AssetFutures, Category: models.CategoryPreciousMetal, Exchange: "SHFE", Symbol: "113.AG0", InternationalRef: "SI=F", IsTradable: true},

	// ── 贵金属 · 基金 (通过现有基金/股票接口) ──
	{Code: "518880", Name: "华安黄金ETF", AssetType: models.AssetETF, Category: models.CategoryPreciousMetal, Exchange: "SH", Symbol: "518880.SH", IsTradable: true},
	{Code: "161226", Name: "国投白银LOF", AssetType: models.AssetETF, Category: models.CategoryPreciousMetal, Exchange: "SZ", Symbol: "161226.SZ", IsTradable: true},

	// ── 能源 · 国际现货 ──
	{Code: "USCL", Name: "WTI原油(纽约)", AssetType: models.AssetSpot, Category: models.CategoryEnergy, Exchange: "OTC", Symbol: "USCL.OTC", InternationalRef: "CL=F", IsTradable: true},
	{Code: "USCO", Name: "布伦特原油(伦敦)", AssetType: models.AssetSpot, Category: models.CategoryEnergy, Exchange: "OTC", Symbol: "USCO.OTC", InternationalRef: "BZ=F", IsTradable: true},

	// ── 能源 · 国内期货 ──
	{Code: "SC", Name: "原油(上海)", AssetType: models.AssetFutures, Category: models.CategoryEnergy, Exchange: "INE", Symbol: "114.SC0", InternationalRef: "CL=F", IsTradable: true},

	// ── 宏观指标 (通过 Yahoo Finance / WallStreetCN / FRED) ──
	{Code: "DXY", Name: "美元指数", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "DXY.OTC", IsTradable: false},
	{Code: "US2YR", Name: "美国2年期国债收益率", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "US2YR.OTC", IsTradable: false},
	{Code: "US5YR", Name: "美国5年期国债收益率", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "US5YR.OTC", IsTradable: false},
	{Code: "US7YR", Name: "美国7年期国债收益率", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "US7YR.OTC", IsTradable: false},
	{Code: "US10YR", Name: "美国10年期国债收益率", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "US10YR.OTC", IsTradable: false},
	{Code: "US30YR", Name: "美国30年期国债收益率", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "OTC", Symbol: "US30YR.OTC", IsTradable: false},
	{Code: "TLT", Name: "20+年美债ETF", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "NASDAQ", Symbol: "TLT", IsTradable: false},
	{Code: "TIP", Name: "TIPS债券ETF", AssetType: models.AssetMacro, Category: models.CategoryMacro, Exchange: "NYSEArca", Symbol: "TIP", IsTradable: false},
}

// FindCommodityByCode 按统一代码查找品种
func FindCommodityByCode(code string) *models.CommodityAsset {
	for _, a := range CommodityRegistry {
		if a.Code == code {
			return &a
		}
	}
	return nil
}

// TradableCommodities 返回所有可交易标的（排除宏观指标）
func TradableCommodities() []models.CommodityAsset {
	result := make([]models.CommodityAsset, 0, len(CommodityRegistry))
	for _, a := range CommodityRegistry {
		if a.IsTradable {
			result = append(result, a)
		}
	}
	return result
}
