package analysis

import (
	"time"
)

// CustomStrategy 自定义选股策略（与 models.CustomStrategy 逐字段一致）。
type CustomStrategy struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Query       string    `json:"query" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"size:500"`
	SortOrder   int       `json:"sortOrder" gorm:"default:0"`
}

func (CustomStrategy) TableName() string {
	return "custom_strategies"
}

// CustomStrategyQuery 分页查询参数
type CustomStrategyQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
}

// CustomStrategyPageData 分页数据
type CustomStrategyPageData struct {
	List       []CustomStrategy `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}
