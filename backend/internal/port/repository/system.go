package repository

import (
	"context"

	"go-stock/backend/internal/domain/system"
)

// SystemRepository abstracts persistence for system-related entities
// (定时任务 / MCP 服务器 / 技能 / AI 助手会话)。
// Implementations live in backend/internal/adapter/repository/sqlite.
type SystemRepository interface {
	// 定时任务
	CreateCronTask(ctx context.Context, task *system.CronTask) error
	// UpdateCronTask 按原 agent 层语义只更新 name/cron_expr/task_type/target/params/enable/status/description 八列。
	UpdateCronTask(ctx context.Context, task *system.CronTask) error
	DeleteCronTask(ctx context.Context, id uint) error
	GetCronTaskByID(ctx context.Context, id uint) (*system.CronTask, error)
	// GetCronTaskList 分页查询；page<1→1, pageSize<1→10, created_at DESC；查询出错返回错误。
	GetCronTaskList(ctx context.Context, query *system.CronTaskQuery) (*system.CronTaskPageResp, error)
	// SearchCronTasks keyword TrimSpace 后按 name/target/description 模糊匹配，Limit 20, created_at DESC。
	SearchCronTasks(ctx context.Context, keyword string) ([]system.CronTask, error)
	// EnableCronTask 仅更新 enable 列。
	EnableCronTask(ctx context.Context, id uint, enable bool) error

	// MCP 服务器
	CreateMCPServer(ctx context.Context, server *system.MCPServer) error
	// UpdateMCPServer 按原 data 层语义只更新 name/description/url/type/headers/command/args/env/enable/status 十列。
	UpdateMCPServer(ctx context.Context, server *system.MCPServer) error
	DeleteMCPServer(ctx context.Context, id uint) error
	GetMCPServerByID(ctx context.Context, id uint) (*system.MCPServer, error)
	// GetMCPServerList 分页查询；page<1→1, pageSize<1→10, created_at DESC；查询出错返回错误。
	GetMCPServerList(ctx context.Context, query *system.MCPServerQuery) (*system.MCPServerPageResp, error)
	// EnableMCPServer 仅更新 enable 列。
	EnableMCPServer(ctx context.Context, id uint, enable bool) error
	// GetMCPToolsByServerID 按 serverID 查询工具，tool_name ASC。
	GetMCPToolsByServerID(ctx context.Context, serverID uint) ([]system.MCPServerTool, error)
	// GetAllMCPTools 返回全部工具，mcp_server_id ASC, tool_name ASC。
	GetAllMCPTools(ctx context.Context) ([]system.MCPServerTool, error)

	// 技能
	CreateSkill(ctx context.Context, skill *system.Skill) error
	// UpdateSkill 按原 data 层语义只更新 name/description/category/system_prompt/examples/trigger_keywords/mcp_server_ids/enable/sort_order 九列。
	UpdateSkill(ctx context.Context, skill *system.Skill) error
	DeleteSkill(ctx context.Context, id uint) error
	GetSkillByID(ctx context.Context, id uint) (*system.Skill, error)
	// GetSkillList 分页查询；与原 data 层一致：无分页默认值、忽略查询错误，sort_order ASC, created_at DESC。
	GetSkillList(ctx context.Context, query *system.SkillQuery) (*system.SkillPageResp, error)
	// EnableSkill 仅更新 enable 列。
	EnableSkill(ctx context.Context, id uint, enable bool) error
	// GetAllSkills 与原 data.GetAll 一致：仅返回 enable=true 的技能，sort_order ASC, created_at DESC；忽略查询错误。
	GetAllSkills(ctx context.Context) ([]system.Skill, error)

	// AI 助手会话
	// GetAiAssistantSession sessionId 非空按 session_id 取，否则取最新（updated_at DESC）；
	// 与原 data 层一致：任何错误/空内容/反序列化失败都返回非 nil 的空 resp 和 nil error。
	GetAiAssistantSession(ctx context.Context, sessionId string) (*system.AiAssistantSessionResp, error)
	// SaveAiAssistantSession len==0 直接返回 nil；超长截尾；按 session_id upsert。
	SaveAiAssistantSession(ctx context.Context, sessionId string, messages []system.AiAssistantMessage) error
}
