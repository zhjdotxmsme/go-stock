package data

// DefaultSponsorAESKeyHex 与 main.checkDir 在 BuildKey 为空时的回退值一致，
// 供 ai-assistant-web 等独立进程解密本地配置中的赞助码。
const DefaultSponsorAESKeyHex = ""

// SponsorDecryptKeyHex 由主程序在启动时同步为 ldflags 注入的 BuildKey；为空则使用 DefaultSponsorAESKeyHex。
var SponsorDecryptKeyHex string

// EffectiveSponsorVipLevel 返回功能权限等级。
// VIP 策略已移除（2026-08，按重构方案 §2.19 与用户决定）：所有功能对全部用户开放，
// 此处固定返回 (2, true)，前端各功能门禁（K线查看/浮动助手/AI推荐等）随之全部解锁。
// 赞助码验证（CheckSponsorCode）保留，仅用于赞助信息展示，不再控制任何功能。
func EffectiveSponsorVipLevel() (level int, active bool) {
	return 2, true
}
