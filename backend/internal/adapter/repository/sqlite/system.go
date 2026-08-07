package sqlite

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/internal/domain/system"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// maxSavedMessages 与原 data 层一致。
const maxSavedMessages = 65535 * 10000

// SystemRepository implements repository.SystemRepository.
// 查询条件/分页默认值/排序/错误语义逐项复刻原 data/agent 层实现。
type SystemRepository struct{}

// NewSystemRepository creates a new SystemRepository.
func NewSystemRepository() *SystemRepository {
	return &SystemRepository{}
}

// ---------------------------------------------------------------------------
// 定时任务（cron_tasks）
// ---------------------------------------------------------------------------

func (r *SystemRepository) CreateCronTask(ctx context.Context, task *system.CronTask) error {
	return db.Dao.Create(task).Error
}

func (r *SystemRepository) UpdateCronTask(ctx context.Context, task *system.CronTask) error {
	// nil/ID==0 校验在 service 层；此处与原 agent 层一致只更新八列
	updates := map[string]any{
		"name":        task.Name,
		"cron_expr":   task.CronExpr,
		"task_type":   task.TaskType,
		"target":      task.Target,
		"params":      task.Params,
		"enable":      task.Enable,
		"status":      task.Status,
		"description": task.Description,
	}
	return db.Dao.Model(&system.CronTask{}).
		Where("id = ?", task.ID).
		Updates(updates).Error
}

func (r *SystemRepository) DeleteCronTask(ctx context.Context, id uint) error {
	return db.Dao.Delete(&system.CronTask{}, id).Error
}

func (r *SystemRepository) GetCronTaskByID(ctx context.Context, id uint) (*system.CronTask, error) {
	var task system.CronTask
	if err := db.Dao.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *SystemRepository) GetCronTaskList(ctx context.Context, query *system.CronTaskQuery) (*system.CronTaskPageResp, error) {
	var tasks []system.CronTask
	var total int64

	dbQuery := db.Dao.Model(&system.CronTask{})

	if query.Name != "" {
		dbQuery = dbQuery.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.TaskType != "" {
		dbQuery = dbQuery.Where("task_type = ?", query.TaskType)
	}
	if query.Status != "" {
		dbQuery = dbQuery.Where("status = ?", query.Status)
	}
	if query.Enable != nil {
		dbQuery = dbQuery.Where("enable = ?", *query.Enable)
	}

	dbQuery.Count(&total)

	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	err := dbQuery.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	if err != nil {
		logger.SugaredLogger.Errorf("查询定时任务列表失败:%s", err.Error())
		return nil, err
	}

	return &system.CronTaskPageResp{
		Total: int(total),
		Data:  tasks,
	}, nil
}

func (r *SystemRepository) SearchCronTasks(ctx context.Context, keyword string) ([]system.CronTask, error) {
	var tasks []system.CronTask
	query := db.Dao.Model(&system.CronTask{})
	if keyword != "" {
		keyword = strings.TrimSpace(keyword)
		query = query.Where("name LIKE ? OR target LIKE ? OR description LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := query.Order("created_at DESC").Limit(20).Find(&tasks).Error
	return tasks, err
}

func (r *SystemRepository) EnableCronTask(ctx context.Context, id uint, enable bool) error {
	return db.Dao.Model(&system.CronTask{}).Where("id = ?", id).Updates(map[string]any{
		"enable": enable,
	}).Error
}

// ---------------------------------------------------------------------------
// MCP 服务器（mcp_servers / mcp_server_tools）
// ---------------------------------------------------------------------------

func (r *SystemRepository) CreateMCPServer(ctx context.Context, server *system.MCPServer) error {
	return db.Dao.Create(server).Error
}

func (r *SystemRepository) UpdateMCPServer(ctx context.Context, server *system.MCPServer) error {
	// nil/ID==0 校验在 service 层；此处与原 data 层一致只更新十列
	updates := map[string]any{
		"name":        server.Name,
		"description": server.Description,
		"url":         server.URL,
		"type":        server.Type,
		"headers":     server.Headers,
		"command":     server.Command,
		"args":        server.Args,
		"env":         server.Env,
		"enable":      server.Enable,
		"status":      server.Status,
	}
	return db.Dao.Model(&system.MCPServer{}).
		Where("id = ?", server.ID).
		Updates(updates).Error
}

func (r *SystemRepository) DeleteMCPServer(ctx context.Context, id uint) error {
	return db.Dao.Delete(&system.MCPServer{}, id).Error
}

func (r *SystemRepository) GetMCPServerByID(ctx context.Context, id uint) (*system.MCPServer, error) {
	var server system.MCPServer
	if err := db.Dao.First(&server, id).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *SystemRepository) GetMCPServerList(ctx context.Context, query *system.MCPServerQuery) (*system.MCPServerPageResp, error) {
	var servers []system.MCPServer
	var total int64

	dbQuery := db.Dao.Model(&system.MCPServer{})

	if query.Name != "" {
		dbQuery = dbQuery.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != "" {
		dbQuery = dbQuery.Where("status = ?", query.Status)
	}
	if query.Enable != nil {
		dbQuery = dbQuery.Where("enable = ?", *query.Enable)
	}

	dbQuery.Count(&total)

	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	err := dbQuery.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Errorf("查询MCP服务器列表失败:%s", err.Error())
		return nil, err
	}

	return &system.MCPServerPageResp{
		Total: int(total),
		Data:  servers,
	}, nil
}

func (r *SystemRepository) EnableMCPServer(ctx context.Context, id uint, enable bool) error {
	return db.Dao.Model(&system.MCPServer{}).Where("id = ?", id).Updates(map[string]any{
		"enable": enable,
	}).Error
}

func (r *SystemRepository) GetMCPToolsByServerID(ctx context.Context, serverID uint) ([]system.MCPServerTool, error) {
	var tools []system.MCPServerTool
	err := db.Dao.Where("mcp_server_id = ?", serverID).Order("tool_name ASC").Find(&tools).Error
	return tools, err
}

func (r *SystemRepository) GetAllMCPTools(ctx context.Context) ([]system.MCPServerTool, error) {
	var tools []system.MCPServerTool
	err := db.Dao.Order("mcp_server_id ASC, tool_name ASC").Find(&tools).Error
	return tools, err
}

// ---------------------------------------------------------------------------
// 技能（skills）
// ---------------------------------------------------------------------------

func (r *SystemRepository) CreateSkill(ctx context.Context, skill *system.Skill) error {
	return db.Dao.Create(skill).Error
}

func (r *SystemRepository) UpdateSkill(ctx context.Context, skill *system.Skill) error {
	// nil/ID==0 校验在 service 层；此处与原 data 层一致只更新九列
	updates := map[string]any{
		"name":             skill.Name,
		"description":      skill.Description,
		"category":         skill.Category,
		"system_prompt":    skill.SystemPrompt,
		"examples":         skill.Examples,
		"trigger_keywords": skill.TriggerKeywords,
		"mcp_server_ids":   skill.MCPServerIDs,
		"enable":           skill.Enable,
		"sort_order":       skill.SortOrder,
	}
	return db.Dao.Model(&system.Skill{}).Where("id = ?", skill.ID).Updates(updates).Error
}

func (r *SystemRepository) DeleteSkill(ctx context.Context, id uint) error {
	return db.Dao.Delete(&system.Skill{}, id).Error
}

func (r *SystemRepository) GetSkillByID(ctx context.Context, id uint) (*system.Skill, error) {
	var skill system.Skill
	if err := db.Dao.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *SystemRepository) GetSkillList(ctx context.Context, query *system.SkillQuery) (*system.SkillPageResp, error) {
	var skills []system.Skill
	var total int64

	q := db.Dao.Model(&system.Skill{})

	if query.Name != "" {
		q = q.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Category != "" {
		q = q.Where("category = ?", query.Category)
	}
	if query.Enable != nil {
		q = q.Where("enable = ?", *query.Enable)
	}

	q.Count(&total)

	// 与原 data 层一致：无分页默认值，忽略 Find 错误
	offset := (query.Page - 1) * query.PageSize
	q.Order("sort_order ASC, created_at DESC").Offset(offset).Limit(query.PageSize).Find(&skills)

	return &system.SkillPageResp{
		Total: int(total),
		Data:  skills,
	}, nil
}

func (r *SystemRepository) EnableSkill(ctx context.Context, id uint, enable bool) error {
	return db.Dao.Model(&system.Skill{}).Where("id = ?", id).Update("enable", enable).Error
}

func (r *SystemRepository) GetAllSkills(ctx context.Context) ([]system.Skill, error) {
	// 与原 data.GetAll 一致：仅 enable=true，忽略 Find 错误
	var skills []system.Skill
	db.Dao.Where("enable = ?", true).Order("sort_order ASC, created_at DESC").Find(&skills)
	return skills, nil
}

// ---------------------------------------------------------------------------
// AI 助手会话（ai_assistant_sessions）
// ---------------------------------------------------------------------------

func (r *SystemRepository) GetAiAssistantSession(ctx context.Context, sessionId string) (*system.AiAssistantSessionResp, error) {
	var row system.AiAssistantSession
	var err error
	if sessionId != "" {
		err = db.Dao.Model(&system.AiAssistantSession{}).Where("session_id = ?", sessionId).First(&row).Error
	} else {
		err = db.Dao.Model(&system.AiAssistantSession{}).Order("updated_at DESC").First(&row).Error
	}
	// 与原 data 层一致：任何错误都返回非 nil 的空 resp（Messages 为空切片）和 nil error
	resp := &system.AiAssistantSessionResp{
		Messages:  []system.AiAssistantMessage{},
		SessionId: row.SessionId,
	}
	if err != nil {
		return resp, nil
	}
	if row.Messages == "" {
		return resp, nil
	}
	var list []system.AiAssistantMessage
	if err := json.Unmarshal([]byte(row.Messages), &list); err != nil {
		return resp, nil
	}
	resp.Messages = list
	return resp, nil
}

func (r *SystemRepository) SaveAiAssistantSession(ctx context.Context, sessionId string, messages []system.AiAssistantMessage) error {
	if len(messages) == 0 {
		return nil
	}
	toSave := messages
	if len(toSave) > maxSavedMessages {
		toSave = toSave[len(toSave)-maxSavedMessages:]
	}
	raw, err := json.Marshal(toSave)
	if err != nil {
		return err
	}
	payload := string(raw)

	var existing system.AiAssistantSession
	err = db.Dao.Model(&system.AiAssistantSession{}).Where("session_id = ?", sessionId).First(&existing).Error
	if err == nil {
		return db.Dao.Model(&system.AiAssistantSession{}).Where("session_id = ?", sessionId).Updates(map[string]interface{}{
			"messages":   payload,
			"updated_at": time.Now(),
		}).Error
	}
	return db.Dao.Create(&system.AiAssistantSession{SessionId: sessionId, Messages: payload}).Error
}

// ---------------------------------------------------------------------------
// data/models <-> domain 显式映射（不反射）
// ---------------------------------------------------------------------------

// CronTaskToDomain maps a data-layer model to the domain model.
func CronTaskToDomain(t *models.CronTask) *system.CronTask {
	if t == nil {
		return nil
	}
	return &system.CronTask{
		ID:            t.ID,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		Name:          t.Name,
		CronExpr:      t.CronExpr,
		TaskType:      t.TaskType,
		Target:        t.Target,
		Params:        t.Params,
		Enable:        t.Enable,
		LastRunAt:     t.LastRunAt,
		NextRunAt:     t.NextRunAt,
		RunCount:      t.RunCount,
		Status:        t.Status,
		Description:   t.Description,
		LastRunResult: t.LastRunResult,
	}
}

// CronTaskFromDomain maps a domain model to the data-layer GORM model.
func CronTaskFromDomain(t *system.CronTask) *models.CronTask {
	if t == nil {
		return nil
	}
	return &models.CronTask{
		ID:            t.ID,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		Name:          t.Name,
		CronExpr:      t.CronExpr,
		TaskType:      t.TaskType,
		Target:        t.Target,
		Params:        t.Params,
		Enable:        t.Enable,
		LastRunAt:     t.LastRunAt,
		NextRunAt:     t.NextRunAt,
		RunCount:      t.RunCount,
		Status:        t.Status,
		Description:   t.Description,
		LastRunResult: t.LastRunResult,
	}
}

// CronTaskQueryToDomain maps a models query to the domain query.
func CronTaskQueryToDomain(q *models.CronTaskQuery) *system.CronTaskQuery {
	if q == nil {
		return nil
	}
	return &system.CronTaskQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		Name:     q.Name,
		TaskType: q.TaskType,
		Status:   q.Status,
		Enable:   q.Enable,
	}
}

// CronTaskListFromDomain maps a domain slice to the models slice;nil 输入保持 nil（JSON null 契约）。
func CronTaskListFromDomain(list []system.CronTask) []models.CronTask {
	if list == nil {
		return nil
	}
	out := make([]models.CronTask, 0, len(list))
	for i := range list {
		out = append(out, *CronTaskFromDomain(&list[i]))
	}
	return out
}

// CronTaskPageRespFromDomain maps a domain page to the models page.
func CronTaskPageRespFromDomain(p *system.CronTaskPageResp) *models.CronTaskPageResp {
	if p == nil {
		return nil
	}
	return &models.CronTaskPageResp{
		Total: p.Total,
		Data:  CronTaskListFromDomain(p.Data),
	}
}

// MCPServerToDomain maps a data-layer model to the domain model.
func MCPServerToDomain(s *models.MCPServer) *system.MCPServer {
	if s == nil {
		return nil
	}
	return &system.MCPServer{
		ID:          s.ID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Name:        s.Name,
		Description: s.Description,
		URL:         s.URL,
		Type:        s.Type,
		Headers:     s.Headers,
		Command:     s.Command,
		Args:        s.Args,
		Env:         s.Env,
		Enable:      s.Enable,
		Status:      s.Status,
		TestResult:  s.TestResult,
	}
}

// MCPServerFromDomain maps a domain model to the data-layer GORM model.
func MCPServerFromDomain(s *system.MCPServer) *models.MCPServer {
	if s == nil {
		return nil
	}
	return &models.MCPServer{
		ID:          s.ID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Name:        s.Name,
		Description: s.Description,
		URL:         s.URL,
		Type:        s.Type,
		Headers:     s.Headers,
		Command:     s.Command,
		Args:        s.Args,
		Env:         s.Env,
		Enable:      s.Enable,
		Status:      s.Status,
		TestResult:  s.TestResult,
	}
}

// MCPServerQueryToDomain maps a models query to the domain query.
func MCPServerQueryToDomain(q *models.MCPServerQuery) *system.MCPServerQuery {
	if q == nil {
		return nil
	}
	return &system.MCPServerQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		Name:     q.Name,
		Status:   q.Status,
		Enable:   q.Enable,
	}
}

// MCPServerPageRespFromDomain maps a domain page to the models page.
func MCPServerPageRespFromDomain(p *system.MCPServerPageResp) *models.MCPServerPageResp {
	if p == nil {
		return nil
	}
	out := &models.MCPServerPageResp{Total: p.Total}
	for i := range p.Data {
		out.Data = append(out.Data, *MCPServerFromDomain(&p.Data[i]))
	}
	return out
}

// MCPServerToolListFromDomain maps a domain slice to the models slice;nil 输入保持 nil（JSON null 契约）。
func MCPServerToolListFromDomain(list []system.MCPServerTool) []models.MCPServerTool {
	if list == nil {
		return nil
	}
	out := make([]models.MCPServerTool, 0, len(list))
	for i := range list {
		t := &list[i]
		out = append(out, models.MCPServerTool{
			ID:           t.ID,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
			MCPServerID:  t.MCPServerID,
			ToolName:     t.ToolName,
			Description:  t.Description,
			ParamsSchema: t.ParamsSchema,
		})
	}
	return out
}

// SkillToDomain maps a data-layer model to the domain model.
func SkillToDomain(s *models.Skill) *system.Skill {
	if s == nil {
		return nil
	}
	return &system.Skill{
		ID:              s.ID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		Name:            s.Name,
		Description:     s.Description,
		Category:        s.Category,
		SystemPrompt:    s.SystemPrompt,
		Examples:        s.Examples,
		TriggerKeywords: s.TriggerKeywords,
		MCPServerIDs:    s.MCPServerIDs,
		Enable:          s.Enable,
		SortOrder:       s.SortOrder,
		UsageCount:      s.UsageCount,
		AvgScore:        s.AvgScore,
		Source:          s.Source,
		Version:         s.Version,
		Confidence:      s.Confidence,
	}
}

// SkillFromDomain maps a domain model to the data-layer GORM model.
func SkillFromDomain(s *system.Skill) *models.Skill {
	if s == nil {
		return nil
	}
	return &models.Skill{
		ID:              s.ID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		Name:            s.Name,
		Description:     s.Description,
		Category:        s.Category,
		SystemPrompt:    s.SystemPrompt,
		Examples:        s.Examples,
		TriggerKeywords: s.TriggerKeywords,
		MCPServerIDs:    s.MCPServerIDs,
		Enable:          s.Enable,
		SortOrder:       s.SortOrder,
		UsageCount:      s.UsageCount,
		AvgScore:        s.AvgScore,
		Source:          s.Source,
		Version:         s.Version,
		Confidence:      s.Confidence,
	}
}

// SkillQueryToDomain maps a models query to the domain query.
func SkillQueryToDomain(q *models.SkillQuery) *system.SkillQuery {
	if q == nil {
		return nil
	}
	return &system.SkillQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		Name:     q.Name,
		Category: q.Category,
		Enable:   q.Enable,
	}
}

// SkillListFromDomain maps a domain slice to the models slice;nil 输入保持 nil（JSON null 契约）。
func SkillListFromDomain(list []system.Skill) []models.Skill {
	if list == nil {
		return nil
	}
	out := make([]models.Skill, 0, len(list))
	for i := range list {
		out = append(out, *SkillFromDomain(&list[i]))
	}
	return out
}

// SkillPageRespFromDomain maps a domain page to the models page.
func SkillPageRespFromDomain(p *system.SkillPageResp) *models.SkillPageResp {
	if p == nil {
		return nil
	}
	return &models.SkillPageResp{
		Total: p.Total,
		Data:  SkillListFromDomain(p.Data),
	}
}

// AiAssistantMessagesToDomain maps a models slice to the domain slice.
func AiAssistantMessagesToDomain(list []models.AiAssistantMessage) []system.AiAssistantMessage {
	out := make([]system.AiAssistantMessage, 0, len(list))
	for i := range list {
		m := &list[i]
		out = append(out, system.AiAssistantMessage{
			Role:        m.Role,
			Content:     m.Content,
			Reasoning:   m.Reasoning,
			Time:        m.Time,
			ModelName:   m.ModelName,
			ToolCalls:   m.ToolCalls,
			ToolResults: m.ToolResults,
			Timeline:    m.Timeline,
		})
	}
	return out
}

// AiAssistantMessagesFromDomain maps a domain slice to the models slice.
func AiAssistantMessagesFromDomain(list []system.AiAssistantMessage) []models.AiAssistantMessage {
	out := make([]models.AiAssistantMessage, 0, len(list))
	for i := range list {
		m := &list[i]
		out = append(out, models.AiAssistantMessage{
			Role:        m.Role,
			Content:     m.Content,
			Reasoning:   m.Reasoning,
			Time:        m.Time,
			ModelName:   m.ModelName,
			ToolCalls:   m.ToolCalls,
			ToolResults: m.ToolResults,
			Timeline:    m.Timeline,
		})
	}
	return out
}

// AiAssistantSessionRespFromDomain maps a domain response to the models response.
func AiAssistantSessionRespFromDomain(r *system.AiAssistantSessionResp) *models.AiAssistantSessionResp {
	if r == nil {
		return nil
	}
	return &models.AiAssistantSessionResp{
		Messages:  AiAssistantMessagesFromDomain(r.Messages),
		SessionId: r.SessionId,
	}
}
