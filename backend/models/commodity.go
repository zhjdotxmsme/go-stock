package models

// AssetType 资产类型
type AssetType string

const (
	AssetSpot    AssetType = "spot"    // 国际现货: XAUUSD, XAGUSD, XAU, XAG, USCL, USCO
	AssetFutures AssetType = "futures" // 国内期货: AU, AG, SC
	AssetETF     AssetType = "etf"     // 商品ETF/LOF: 518880, 161226
	AssetMacro   AssetType = "macro"   // 宏观指标: TLT, TIP, US2YR, US5YR, US7YR, US10YR, US30YR, DXY
)

// CommodityCategory 分类
type CommodityCategory string

const (
	CategoryPreciousMetal CommodityCategory = "贵金属" // gold, silver
	CategoryEnergy        CommodityCategory = "能源"    // crude oil
	CategoryFund          CommodityCategory = "基金"    // ETF/LOF
	CategoryMacro         CommodityCategory = "宏观"    // treasury, DXY, TIPS
)

// CommodityAsset 商品品种定义
type CommodityAsset struct {
	Code             string            `json:"code"`             // 统一代码: XAUUSD, AU, 518880, TLT
	Name             string            `json:"name"`             // 显示名称: 现货黄金(纽约), 沪金, 黄金ETF
	AssetType        AssetType         `json:"assetType"`        // spot/futures/etf/macro
	Category         CommodityCategory `json:"category"`         // 贵金属/能源/基金/宏观
	Exchange         string            `json:"exchange"`         // OTC, SHFE, INE, LBMA, NASDAQ, NYSEArca, SZ, SH
	Symbol           string            `json:"symbol"`           // 数据源原始代码
	InternationalRef string            `json:"internationalRef"` // 国际参考代码, 如 GC=F/SI=F/CL=F/BZ=F
	IsTradable       bool              `json:"isTradable"`       // 是否为可交易标的（宏观指标不可交易）
}
