package system

import "time"

// MCPServer MCP 服务器
type MCPServer struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Description string    `json:"description" gorm:"size:500"`
	URL         string    `json:"url" gorm:"size:500"`
	Type        string    `json:"type" gorm:"size:20;default:streamable-http"`
	Headers     string    `json:"headers" gorm:"type:text"`
	Command     string    `json:"command" gorm:"size:500"`
	Args        string    `json:"args" gorm:"type:text"`
	Env         string    `json:"env" gorm:"type:text"`
	Enable      bool      `json:"enable" gorm:"default:true"`
	Status      string    `json:"status" gorm:"size:20;default:stopped"`
	TestResult  string    `json:"testResult" gorm:"size:500"`
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

// MCPServerQuery MCP 服务器查询参数
type MCPServerQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

// MCPServerPageResp MCP 服务器分页响应
type MCPServerPageResp struct {
	Total int         `json:"total"`
	Data  []MCPServer `json:"data"`
}

// MCPServerTool MCP 服务器工具
type MCPServerTool struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MCPServerID  uint      `json:"mcpServerId" gorm:"index;not null"`
	ToolName     string    `json:"toolName" gorm:"size:255;not null"`
	Description  string    `json:"description" gorm:"type:text"`
	ParamsSchema string    `json:"paramsSchema" gorm:"type:text"`
}

func (MCPServerTool) TableName() string {
	return "mcp_server_tools"
}
