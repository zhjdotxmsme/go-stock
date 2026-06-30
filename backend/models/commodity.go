package models

// AssetType 资产类型
type AssetType string

const (
	AssetSpot    AssetType = "spot"    // 国际现货: XAUUSD, XAGUSD, XPTUSD, USCL
	AssetFutures AssetType = "futures" // 国内期货: AU, AG, SC, CU
	AssetETF     AssetType = "etf"     // 商品ETF: 518880, 159930, 159981
)

// CommodityAsset 商品品种定义
type CommodityAsset struct {
	Code      string    `json:"code"`      // 统一代码: XAUUSD, AU, 518880
	Name      string    `json:"name"`      // 显示名称: 现货黄金, 沪金, 黄金ETF
	AssetType AssetType `json:"assetType"`
	Exchange  string    `json:"exchange"`  // OTC, SHFE, INE, COMEX
	Symbol    string    `json:"symbol"`    // 数据源原始代码
}
