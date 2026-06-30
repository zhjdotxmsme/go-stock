package data

import "go-stock/backend/models"

// CommodityRegistry 商品品种注册表
var CommodityRegistry = []models.CommodityAsset{
	// ── 国际现货 (通过 WallStreetCN) ──
	{Code: "XAUUSD", Name: "现货黄金", AssetType: models.AssetSpot, Exchange: "OTC", Symbol: "XAUUSD.OTC"},
	{Code: "XAGUSD", Name: "现货白银", AssetType: models.AssetSpot, Exchange: "OTC", Symbol: "XAGUSD.OTC"},
	{Code: "USCL", Name: "WTI原油", AssetType: models.AssetSpot, Exchange: "OTC", Symbol: "USCL.OTC"},

	// ── 国内期货 (通过 EastMoney push2) ──
	{Code: "AU", Name: "沪金", AssetType: models.AssetFutures, Exchange: "SHFE", Symbol: "113.AU0"},
	{Code: "AG", Name: "沪银", AssetType: models.AssetFutures, Exchange: "SHFE", Symbol: "113.AG0"},
	{Code: "SC", Name: "原油", AssetType: models.AssetFutures, Exchange: "INE", Symbol: "114.SC0"},

	// ── 商品ETF (通过现有基金/股票接口) ──
	{Code: "518880", Name: "华安黄金ETF", AssetType: models.AssetETF, Exchange: "SH", Symbol: "518880.SH"},
	{Code: "159930", Name: "能源ETF", AssetType: models.AssetETF, Exchange: "SZ", Symbol: "159930.SZ"},
	{Code: "159981", Name: "有色金属ETF", AssetType: models.AssetETF, Exchange: "SZ", Symbol: "159981.SZ"},
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
