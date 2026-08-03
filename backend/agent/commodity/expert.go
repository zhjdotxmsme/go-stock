package commodity

import (
	"context"
	"go-stock/backend/models"
)

// Expert 通用专家接口
type Expert interface {
	Role() string
	Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error)
}

// CategoryExpert 品种专属专家接口，按 CommodityCategory 路由
type CategoryExpert interface {
	Expert
	Categories() []models.CommodityCategory
}

var defaultExperts []Expert
var categoryExperts []CategoryExpert

// RegisterExpert 注册通用专家（始终运行）
func RegisterExpert(e Expert) {
	defaultExperts = append(defaultExperts, e)
}

// RegisterCategoryExpert 注册品种专属专家（按 Category 路由）
func RegisterCategoryExpert(e CategoryExpert) {
	categoryExperts = append(categoryExperts, e)
}

// GetDefaultExperts 返回通用专家列表
func GetDefaultExperts() []Expert {
	return defaultExperts
}

// GetExpertsForCategory 返回通用专家 + 匹配的品种专属专家
func GetExpertsForCategory(cat models.CommodityCategory) []Expert {
	experts := make([]Expert, 0, len(defaultExperts)+3)
	experts = append(experts, defaultExperts...)
	for _, ce := range categoryExperts {
		for _, c := range ce.Categories() {
			if c == cat {
				experts = append(experts, ce)
				break
			}
		}
	}
	return experts
}
