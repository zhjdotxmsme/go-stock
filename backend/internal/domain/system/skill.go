package system

import "time"

// Skill 技能
type Skill struct {
	ID              uint      `json:"id" gorm:"primarykey"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Name            string    `json:"name" gorm:"size:255;not null"`
	Description     string    `json:"description" gorm:"size:500"`
	Category        string    `json:"category" gorm:"size:50"`
	SystemPrompt    string    `json:"systemPrompt" gorm:"type:text"`
	Examples        string    `json:"examples" gorm:"type:text"`
	TriggerKeywords string    `json:"triggerKeywords" gorm:"size:500"`
	MCPServerIDs    string    `json:"mcpServerIds" gorm:"size:500"`
	Enable          bool      `json:"enable" gorm:"default:true"`
	SortOrder       int       `json:"sortOrder" gorm:"default:0"`
	UsageCount      int       `json:"usageCount" gorm:"default:0"`
	AvgScore        float64   `json:"avgScore" gorm:"default:0"`
	Source          string    `json:"source" gorm:"default:user"`
	Version         int       `json:"version" gorm:"default:1"`
	Confidence      float64   `json:"confidence" gorm:"default:1"`
}

func (Skill) TableName() string {
	return "skills"
}

// SkillQuery 技能查询参数
type SkillQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Enable   *bool  `json:"enable"`
}

// SkillPageResp 技能分页响应
type SkillPageResp struct {
	Total int     `json:"total"`
	Data  []Skill `json:"data"`
}
