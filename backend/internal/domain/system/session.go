package system

import (
	"encoding/json"
	"time"
)

// AiAssistantSession AI 助手会话
type AiAssistantSession struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	SessionId string    `json:"sessionId" gorm:"index;size:64"` // 会话ID，时间戳格式
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Messages  string    `json:"messages" gorm:"type:text"` // JSON 数组，每项 { role, content, reasoning, time, modelName }
}

func (AiAssistantSession) TableName() string {
	return "ai_assistant_sessions"
}

// AiAssistantMessage 单条消息，供前后端 JSON 序列化
type AiAssistantMessage struct {
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	Reasoning   string          `json:"reasoning"`
	Time        string          `json:"time"`                // 消息时间，格式如 "2006-01-02 15:04:05"
	ModelName   string          `json:"modelName,omitempty"` // 助手回复所用模型展示名（如配置名称 + 模型名）
	ToolCalls   json.RawMessage `json:"toolCalls,omitempty"`
	ToolResults json.RawMessage `json:"toolResults,omitempty"`
	Timeline    json.RawMessage `json:"timeline,omitempty"` // 前端按时间序存储的正文/工具块
}

// AiAssistantSessionResp 会话查询响应
type AiAssistantSessionResp struct {
	Messages  []AiAssistantMessage `json:"messages"`
	SessionId string               `json:"sessionId"`
}
