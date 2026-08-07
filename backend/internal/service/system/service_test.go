package system

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/internal/domain/system"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic,可暴露 service 对 port 的误用。
type stubRepo struct {
	repository.SystemRepository

	updatedCron   *system.CronTask
	cronUpdateErr error
	cronDeletedID uint
	cronDeleteErr error
	cronTask      *system.CronTask
	cronTaskErr   error
	cronPage      *system.CronTaskPageResp
	cronPageErr   error
	cronQuery     *system.CronTaskQuery
	searchKeyword string
	searchResult  []system.CronTask
	cronEnableID  uint
	cronEnableVal bool
	cronEnableErr error

	updatedMCP   *system.MCPServer
	mcpUpdateErr error
	mcpPage      *system.MCPServerPageResp
	mcpPageErr   error
	mcpServer    *system.MCPServer
	mcpServerErr error
	mcpEnableErr error
	mcpTools     []system.MCPServerTool

	updatedSkill   *system.Skill
	skillUpdateErr error
	skill          *system.Skill
	skillErr       error
	skillPage      *system.SkillPageResp
	skillEnableErr error
	allSkills      []system.Skill

	sessionResp    *system.AiAssistantSessionResp
	sessionRespErr error
	savedSessionID string
	savedMessages  []system.AiAssistantMessage
	saveSessionErr error
}

func (s *stubRepo) UpdateCronTask(ctx context.Context, t *system.CronTask) error {
	if s.cronUpdateErr != nil {
		return s.cronUpdateErr
	}
	s.updatedCron = t
	return nil
}
func (s *stubRepo) DeleteCronTask(ctx context.Context, id uint) error {
	if s.cronDeleteErr != nil {
		return s.cronDeleteErr
	}
	s.cronDeletedID = id
	return nil
}
func (s *stubRepo) GetCronTaskByID(ctx context.Context, id uint) (*system.CronTask, error) {
	return s.cronTask, s.cronTaskErr
}
func (s *stubRepo) GetCronTaskList(ctx context.Context, q *system.CronTaskQuery) (*system.CronTaskPageResp, error) {
	s.cronQuery = q
	return s.cronPage, s.cronPageErr
}
func (s *stubRepo) SearchCronTasks(ctx context.Context, keyword string) ([]system.CronTask, error) {
	s.searchKeyword = keyword
	return s.searchResult, nil
}
func (s *stubRepo) EnableCronTask(ctx context.Context, id uint, enable bool) error {
	s.cronEnableID = id
	s.cronEnableVal = enable
	return s.cronEnableErr
}

func (s *stubRepo) UpdateMCPServer(ctx context.Context, srv *system.MCPServer) error {
	if s.mcpUpdateErr != nil {
		return s.mcpUpdateErr
	}
	s.updatedMCP = srv
	return nil
}
func (s *stubRepo) GetMCPServerByID(ctx context.Context, id uint) (*system.MCPServer, error) {
	return s.mcpServer, s.mcpServerErr
}
func (s *stubRepo) GetMCPServerList(ctx context.Context, q *system.MCPServerQuery) (*system.MCPServerPageResp, error) {
	return s.mcpPage, s.mcpPageErr
}
func (s *stubRepo) EnableMCPServer(ctx context.Context, id uint, enable bool) error {
	return s.mcpEnableErr
}
func (s *stubRepo) GetMCPToolsByServerID(ctx context.Context, serverID uint) ([]system.MCPServerTool, error) {
	return s.mcpTools, nil
}
func (s *stubRepo) GetAllMCPTools(ctx context.Context) ([]system.MCPServerTool, error) {
	return s.mcpTools, nil
}

func (s *stubRepo) UpdateSkill(ctx context.Context, sk *system.Skill) error {
	if s.skillUpdateErr != nil {
		return s.skillUpdateErr
	}
	s.updatedSkill = sk
	return nil
}
func (s *stubRepo) GetSkillByID(ctx context.Context, id uint) (*system.Skill, error) {
	return s.skill, s.skillErr
}
func (s *stubRepo) GetSkillList(ctx context.Context, q *system.SkillQuery) (*system.SkillPageResp, error) {
	return s.skillPage, nil
}
func (s *stubRepo) EnableSkill(ctx context.Context, id uint, enable bool) error {
	return s.skillEnableErr
}
func (s *stubRepo) GetAllSkills(ctx context.Context) ([]system.Skill, error) {
	return s.allSkills, nil
}

func (s *stubRepo) GetAiAssistantSession(ctx context.Context, sessionId string) (*system.AiAssistantSessionResp, error) {
	return s.sessionResp, s.sessionRespErr
}
func (s *stubRepo) SaveAiAssistantSession(ctx context.Context, sessionId string, messages []system.AiAssistantMessage) error {
	if s.saveSessionErr != nil {
		return s.saveSessionErr
	}
	s.savedSessionID = sessionId
	s.savedMessages = messages
	return nil
}

func TestUpdateCronTask_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil任务", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateCronTask(ctx, nil)
		if err == nil || err.Error() != "无效的任务ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("ID为0", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateCronTask(ctx, &system.CronTask{})
		if err == nil || err.Error() != "无效的任务ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("正常透传", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		if err := svc.UpdateCronTask(ctx, &system.CronTask{ID: 3, Name: "任务A"}); err != nil {
			t.Fatal(err)
		}
		if repo.updatedCron == nil || repo.updatedCron.ID != 3 || repo.updatedCron.Name != "任务A" {
			t.Errorf("updated=%+v", repo.updatedCron)
		}
	})
	t.Run("repo错误透传", func(t *testing.T) {
		repo := &stubRepo{cronUpdateErr: errors.New("db err")}
		svc := NewService(repo)
		err := svc.UpdateCronTask(ctx, &system.CronTask{ID: 3})
		if err == nil || err.Error() != "db err" {
			t.Errorf("err=%v", err)
		}
	})
}

func TestUpdateMCPServer_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil服务器", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateMCPServer(ctx, nil)
		if err == nil || err.Error() != "无效的服务器ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("ID为0", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateMCPServer(ctx, &system.MCPServer{})
		if err == nil || err.Error() != "无效的服务器ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("正常透传", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		if err := svc.UpdateMCPServer(ctx, &system.MCPServer{ID: 2, Name: "srv"}); err != nil {
			t.Fatal(err)
		}
		if repo.updatedMCP == nil || repo.updatedMCP.ID != 2 {
			t.Errorf("updated=%+v", repo.updatedMCP)
		}
	})
}

func TestUpdateSkill_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil技能", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateSkill(ctx, nil)
		if err == nil || err.Error() != "无效的技能ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("ID为0", func(t *testing.T) {
		svc := NewService(&stubRepo{})
		err := svc.UpdateSkill(ctx, &system.Skill{})
		if err == nil || err.Error() != "无效的技能ID" {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("正常透传", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		if err := svc.UpdateSkill(ctx, &system.Skill{ID: 9, Name: "sk"}); err != nil {
			t.Fatal(err)
		}
		if repo.updatedSkill == nil || repo.updatedSkill.ID != 9 {
			t.Errorf("updated=%+v", repo.updatedSkill)
		}
	})
}

func TestListErrorPropagation(t *testing.T) {
	ctx := context.Background()

	t.Run("Cron列表错误透传为nil", func(t *testing.T) {
		repo := &stubRepo{cronPageErr: errors.New("query err")}
		svc := NewService(repo)
		page, err := svc.GetCronTaskList(ctx, &system.CronTaskQuery{})
		if err == nil || err.Error() != "query err" || page != nil {
			t.Errorf("page=%v err=%v", page, err)
		}
	})
	t.Run("MCP列表错误透传为nil", func(t *testing.T) {
		repo := &stubRepo{mcpPageErr: errors.New("query err")}
		svc := NewService(repo)
		page, err := svc.GetMCPServerList(ctx, &system.MCPServerQuery{})
		if err == nil || err.Error() != "query err" || page != nil {
			t.Errorf("page=%v err=%v", page, err)
		}
	})
	t.Run("查询对象原样传递", func(t *testing.T) {
		enable := true
		repo := &stubRepo{}
		svc := NewService(repo)
		q := &system.CronTaskQuery{Page: 2, PageSize: 5, Name: "n", TaskType: "stock_analysis", Enable: &enable}
		if _, err := svc.GetCronTaskList(ctx, q); err != nil {
			t.Fatal(err)
		}
		if repo.cronQuery != q {
			t.Errorf("query 未原样传递: %+v", repo.cronQuery)
		}
	})
}

func TestPassthrough(t *testing.T) {
	ctx := context.Background()

	t.Run("GetByID透传", func(t *testing.T) {
		repo := &stubRepo{cronTask: &system.CronTask{ID: 7}}
		svc := NewService(repo)
		got, err := svc.GetCronTaskByID(ctx, 7)
		if err != nil || got == nil || got.ID != 7 {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
	t.Run("GetByID错误透传", func(t *testing.T) {
		repo := &stubRepo{cronTaskErr: errors.New("not found")}
		svc := NewService(repo)
		got, err := svc.GetCronTaskByID(ctx, 7)
		if err == nil || got != nil {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
	t.Run("Enable透传参数", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		if err := svc.EnableCronTask(ctx, 4, true); err != nil {
			t.Fatal(err)
		}
		if repo.cronEnableID != 4 || !repo.cronEnableVal {
			t.Errorf("enable 参数: id=%d val=%v", repo.cronEnableID, repo.cronEnableVal)
		}
	})
	t.Run("Search关键词透传", func(t *testing.T) {
		repo := &stubRepo{searchResult: []system.CronTask{{ID: 1}}}
		svc := NewService(repo)
		got, err := svc.SearchCronTasks(ctx, " 异动 ")
		if err != nil || len(got) != 1 {
			t.Errorf("got=%v err=%v", got, err)
		}
		if repo.searchKeyword != " 异动 " {
			t.Errorf("keyword=%q", repo.searchKeyword)
		}
	})
	t.Run("MCP工具透传", func(t *testing.T) {
		repo := &stubRepo{mcpTools: []system.MCPServerTool{{ToolName: "t1"}}}
		svc := NewService(repo)
		got, err := svc.GetAllMCPTools(ctx)
		if err != nil || len(got) != 1 || got[0].ToolName != "t1" {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
	t.Run("GetAllSkills透传", func(t *testing.T) {
		repo := &stubRepo{allSkills: []system.Skill{{ID: 1}, {ID: 2}}}
		svc := NewService(repo)
		got, err := svc.GetAllSkills(ctx)
		if err != nil || len(got) != 2 {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
}

func TestSessionPassthrough(t *testing.T) {
	ctx := context.Background()

	t.Run("读取透传repo返回值不加工", func(t *testing.T) {
		resp := &system.AiAssistantSessionResp{
			Messages:  []system.AiAssistantMessage{},
			SessionId: "s1",
		}
		repo := &stubRepo{sessionResp: resp}
		svc := NewService(repo)
		got, err := svc.GetAiAssistantSession(ctx, "s1")
		if err != nil || got != resp {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
	t.Run("保存参数透传", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		msgs := []system.AiAssistantMessage{{Role: "user", Content: "hi"}}
		if err := svc.SaveAiAssistantSession(ctx, "s2", msgs); err != nil {
			t.Fatal(err)
		}
		if repo.savedSessionID != "s2" || len(repo.savedMessages) != 1 || repo.savedMessages[0].Content != "hi" {
			t.Errorf("saved: id=%q msgs=%+v", repo.savedSessionID, repo.savedMessages)
		}
	})
	t.Run("保存错误透传", func(t *testing.T) {
		repo := &stubRepo{saveSessionErr: errors.New("db locked")}
		svc := NewService(repo)
		err := svc.SaveAiAssistantSession(ctx, "s2", []system.AiAssistantMessage{{Role: "user"}})
		if err == nil || err.Error() != "db locked" {
			t.Errorf("err=%v", err)
		}
	})
}
