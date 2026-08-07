// Package system 系统服务
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载系统域:定时任务/MCP 服务器/技能/AI 助手会话的 DB 编排;
// 文案与调度器交互保留在 handler 层。
package system

import (
	"context"
	"fmt"

	"go-stock/backend/internal/domain/system"
	"go-stock/backend/internal/port/repository"
)

// Service 系统服务
type Service struct {
	repo repository.SystemRepository
}

// NewService 创建系统服务。
func NewService(repo repository.SystemRepository) *Service {
	return &Service{repo: repo}
}

// ---------------------------------------------------------------------------
// 定时任务
// ---------------------------------------------------------------------------

// CreateCronTask 创建定时任务。
func (s *Service) CreateCronTask(ctx context.Context, task *system.CronTask) error {
	return s.repo.CreateCronTask(ctx, task)
}

// UpdateCronTask 更新定时任务；nil/ID==0 校验错误文本与原 agent 层逐字一致。
func (s *Service) UpdateCronTask(ctx context.Context, task *system.CronTask) error {
	if task == nil || task.ID == 0 {
		return fmt.Errorf("无效的任务ID")
	}
	return s.repo.UpdateCronTask(ctx, task)
}

// DeleteCronTask 删除定时任务。
func (s *Service) DeleteCronTask(ctx context.Context, id uint) error {
	return s.repo.DeleteCronTask(ctx, id)
}

// GetCronTaskByID 按 ID 查询定时任务。
func (s *Service) GetCronTaskByID(ctx context.Context, id uint) (*system.CronTask, error) {
	return s.repo.GetCronTaskByID(ctx, id)
}

// GetCronTaskList 分页查询定时任务；出错时返回 (nil, err)，由 handler 保持 nil 契约。
func (s *Service) GetCronTaskList(ctx context.Context, query *system.CronTaskQuery) (*system.CronTaskPageResp, error) {
	return s.repo.GetCronTaskList(ctx, query)
}

// SearchCronTasks 关键词搜索定时任务。
func (s *Service) SearchCronTasks(ctx context.Context, keyword string) ([]system.CronTask, error) {
	return s.repo.SearchCronTasks(ctx, keyword)
}

// EnableCronTask 启用/禁用定时任务。
func (s *Service) EnableCronTask(ctx context.Context, id uint, enable bool) error {
	return s.repo.EnableCronTask(ctx, id, enable)
}

// ---------------------------------------------------------------------------
// MCP 服务器
// ---------------------------------------------------------------------------

// CreateMCPServer 创建 MCP 服务器。
func (s *Service) CreateMCPServer(ctx context.Context, server *system.MCPServer) error {
	return s.repo.CreateMCPServer(ctx, server)
}

// UpdateMCPServer 更新 MCP 服务器；nil/ID==0 校验错误文本与原 data 层逐字一致。
func (s *Service) UpdateMCPServer(ctx context.Context, server *system.MCPServer) error {
	if server == nil || server.ID == 0 {
		return fmt.Errorf("无效的服务器ID")
	}
	return s.repo.UpdateMCPServer(ctx, server)
}

// DeleteMCPServer 删除 MCP 服务器。
func (s *Service) DeleteMCPServer(ctx context.Context, id uint) error {
	return s.repo.DeleteMCPServer(ctx, id)
}

// GetMCPServerByID 按 ID 查询 MCP 服务器。
func (s *Service) GetMCPServerByID(ctx context.Context, id uint) (*system.MCPServer, error) {
	return s.repo.GetMCPServerByID(ctx, id)
}

// GetMCPServerList 分页查询 MCP 服务器；出错时返回 (nil, err)，由 handler 保持 nil 契约。
func (s *Service) GetMCPServerList(ctx context.Context, query *system.MCPServerQuery) (*system.MCPServerPageResp, error) {
	return s.repo.GetMCPServerList(ctx, query)
}

// EnableMCPServer 启用/禁用 MCP 服务器。
func (s *Service) EnableMCPServer(ctx context.Context, id uint, enable bool) error {
	return s.repo.EnableMCPServer(ctx, id, enable)
}

// GetMCPToolsByServerID 按服务器 ID 查询工具。
func (s *Service) GetMCPToolsByServerID(ctx context.Context, serverID uint) ([]system.MCPServerTool, error) {
	return s.repo.GetMCPToolsByServerID(ctx, serverID)
}

// GetAllMCPTools 返回全部 MCP 工具。
func (s *Service) GetAllMCPTools(ctx context.Context) ([]system.MCPServerTool, error) {
	return s.repo.GetAllMCPTools(ctx)
}

// ---------------------------------------------------------------------------
// 技能
// ---------------------------------------------------------------------------

// CreateSkill 创建技能。
func (s *Service) CreateSkill(ctx context.Context, skill *system.Skill) error {
	return s.repo.CreateSkill(ctx, skill)
}

// UpdateSkill 更新技能；nil/ID==0 校验错误文本与原 data 层逐字一致。
func (s *Service) UpdateSkill(ctx context.Context, skill *system.Skill) error {
	if skill == nil || skill.ID == 0 {
		return fmt.Errorf("无效的技能ID")
	}
	return s.repo.UpdateSkill(ctx, skill)
}

// DeleteSkill 删除技能。
func (s *Service) DeleteSkill(ctx context.Context, id uint) error {
	return s.repo.DeleteSkill(ctx, id)
}

// GetSkillByID 按 ID 查询技能。
func (s *Service) GetSkillByID(ctx context.Context, id uint) (*system.Skill, error) {
	return s.repo.GetSkillByID(ctx, id)
}

// GetSkillList 分页查询技能。
func (s *Service) GetSkillList(ctx context.Context, query *system.SkillQuery) (*system.SkillPageResp, error) {
	return s.repo.GetSkillList(ctx, query)
}

// EnableSkill 启用/禁用技能。
func (s *Service) EnableSkill(ctx context.Context, id uint, enable bool) error {
	return s.repo.EnableSkill(ctx, id, enable)
}

// GetAllSkills 返回全部启用的技能（与原 data.GetAll 语义一致）。
func (s *Service) GetAllSkills(ctx context.Context) ([]system.Skill, error) {
	return s.repo.GetAllSkills(ctx)
}

// ---------------------------------------------------------------------------
// AI 助手会话
// ---------------------------------------------------------------------------

// GetAiAssistantSession 获取会话消息；repo 保证任何错误都返回非 nil 空 resp 和 nil error。
func (s *Service) GetAiAssistantSession(ctx context.Context, sessionId string) (*system.AiAssistantSessionResp, error) {
	return s.repo.GetAiAssistantSession(ctx, sessionId)
}

// SaveAiAssistantSession 保存会话消息（upsert）。
func (s *Service) SaveAiAssistantSession(ctx context.Context, sessionId string, messages []system.AiAssistantMessage) error {
	return s.repo.SaveAiAssistantSession(ctx, sessionId, messages)
}
