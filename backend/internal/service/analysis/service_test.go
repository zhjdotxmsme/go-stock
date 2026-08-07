package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-stock/backend/internal/domain/analysis"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic,可暴露 service 对 port 的误用。
type stubRepo struct {
	repository.AnalysisRepository

	strategy   *analysis.CustomStrategy
	strategies []analysis.CustomStrategy
	created    *analysis.CustomStrategy
	updated    *analysis.CustomStrategy
	createErr  error
	updateErr  error
	deleteErr  error
	deletedID  uint

	template     *analysis.PromptTemplate
	tmplCreated  *analysis.PromptTemplate
	tmplUpdated  *analysis.PromptTemplate
	tmplCreateEr error
	tmplUpdateEr error
	tmplDeleteEr error
	tmplDeleted  uint

	recommends   []analysis.AiRecommendStocks
	recommendErr error

	upsertErr      error
	upsertArgs     [4]string
	delErr         error
	batchDelErr    error
	alertErr       error
	savedAIResp    *analysis.AIResponseResult
	saveAIRespErr  error
	latestAIResp   *analysis.AIResponseResult
	latestAIRespEr error
}

func (s *stubRepo) GetCustomStrategyByID(ctx context.Context, id uint) (*analysis.CustomStrategy, error) {
	return s.strategy, nil
}
func (s *stubRepo) CreateCustomStrategy(ctx context.Context, st *analysis.CustomStrategy) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = st
	return nil
}
func (s *stubRepo) UpdateCustomStrategy(ctx context.Context, st *analysis.CustomStrategy) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = st
	return nil
}
func (s *stubRepo) DeleteCustomStrategy(ctx context.Context, id uint) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedID = id
	return nil
}
func (s *stubRepo) ListAllCustomStrategies(ctx context.Context) ([]analysis.CustomStrategy, error) {
	return s.strategies, nil
}
func (s *stubRepo) GetPromptTemplateByID(ctx context.Context, id uint) (*analysis.PromptTemplate, error) {
	return s.template, nil
}
func (s *stubRepo) CreatePromptTemplate(ctx context.Context, t *analysis.PromptTemplate) error {
	if s.tmplCreateEr != nil {
		return s.tmplCreateEr
	}
	s.tmplCreated = t
	return nil
}
func (s *stubRepo) UpdatePromptTemplate(ctx context.Context, t *analysis.PromptTemplate) error {
	if s.tmplUpdateEr != nil {
		return s.tmplUpdateEr
	}
	s.tmplUpdated = t
	return nil
}
func (s *stubRepo) DeletePromptTemplate(ctx context.Context, id uint) error {
	if s.tmplDeleteEr != nil {
		return s.tmplDeleteEr
	}
	s.tmplDeleted = id
	return nil
}
func (s *stubRepo) UpsertPromptByRoleKey(ctx context.Context, roleKey, name, content, ptype string) error {
	s.upsertArgs = [4]string{roleKey, name, content, ptype}
	return s.upsertErr
}
func (s *stubRepo) ListAllAiRecommendStocks(ctx context.Context) ([]analysis.AiRecommendStocks, error) {
	return s.recommends, s.recommendErr
}
func (s *stubRepo) DeleteAIResponseResult(ctx context.Context, id uint) error { return s.delErr }
func (s *stubRepo) BatchDeleteAIResponseResult(ctx context.Context, ids []uint) error {
	return s.batchDelErr
}
func (s *stubRepo) DeleteAiRecommendStocks(ctx context.Context, id uint) error { return s.delErr }
func (s *stubRepo) UpdateAiRecommendStocksAlert(ctx context.Context, id uint, enableAlert bool) error {
	return s.alertErr
}
func (s *stubRepo) SaveAIResponseResult(ctx context.Context, item *analysis.AIResponseResult) error {
	if s.saveAIRespErr != nil {
		return s.saveAIRespErr
	}
	s.savedAIResp = item
	return nil
}
func (s *stubRepo) GetLatestAIResponseResult(ctx context.Context, stockCode string) (*analysis.AIResponseResult, error) {
	return s.latestAIResp, s.latestAIRespEr
}

func TestSaveCustomStrategy(t *testing.T) {
	ctx := context.Background()

	t.Run("新增成功", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo, nil)
		got := svc.SaveCustomStrategy(ctx, analysis.CustomStrategy{Name: "均线策略", Query: "ma5>ma10"})
		if got != "添加成功" {
			t.Errorf("got %q", got)
		}
		if repo.created == nil || repo.created.Name != "均线策略" {
			t.Errorf("created=%+v", repo.created)
		}
	})

	t.Run("新增失败", func(t *testing.T) {
		repo := &stubRepo{createErr: errors.New("db err")}
		svc := NewService(repo, nil)
		if got := svc.SaveCustomStrategy(ctx, analysis.CustomStrategy{}); got != "添加失败" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("更新_策略不存在", func(t *testing.T) {
		repo := &stubRepo{} // strategy nil
		svc := NewService(repo, nil)
		if got := svc.SaveCustomStrategy(ctx, analysis.CustomStrategy{ID: 5}); got != "策略不存在" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("更新成功", func(t *testing.T) {
		repo := &stubRepo{strategy: &analysis.CustomStrategy{ID: 5}}
		svc := NewService(repo, nil)
		if got := svc.SaveCustomStrategy(ctx, analysis.CustomStrategy{ID: 5, Name: "新名"}); got != "更新成功" {
			t.Errorf("got %q", got)
		}
		if repo.updated == nil || repo.updated.Name != "新名" {
			t.Errorf("updated=%+v", repo.updated)
		}
	})

	t.Run("更新失败", func(t *testing.T) {
		repo := &stubRepo{strategy: &analysis.CustomStrategy{ID: 5}, updateErr: errors.New("x")}
		svc := NewService(repo, nil)
		if got := svc.SaveCustomStrategy(ctx, analysis.CustomStrategy{ID: 5}); got != "更新失败" {
			t.Errorf("got %q", got)
		}
	})
}

func TestDeleteCustomStrategy(t *testing.T) {
	ctx := context.Background()

	t.Run("不存在", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		if got := svc.DeleteCustomStrategy(ctx, 9); got != "策略不存在" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("删除成功", func(t *testing.T) {
		repo := &stubRepo{strategy: &analysis.CustomStrategy{ID: 9}}
		svc := NewService(repo, nil)
		if got := svc.DeleteCustomStrategy(ctx, 9); got != "删除成功" {
			t.Errorf("got %q", got)
		}
		if repo.deletedID != 9 {
			t.Errorf("deletedID=%d", repo.deletedID)
		}
	})
	t.Run("删除失败", func(t *testing.T) {
		repo := &stubRepo{strategy: &analysis.CustomStrategy{ID: 9}, deleteErr: errors.New("x")}
		svc := NewService(repo, nil)
		if got := svc.DeleteCustomStrategy(ctx, 9); got != "删除失败" {
			t.Errorf("got %q", got)
		}
	})
}

func TestSavePromptTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("不存在则添加", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo, nil)
		if got := svc.SavePromptTemplate(ctx, analysis.PromptTemplate{Name: "p1"}); got != "添加成功" {
			t.Errorf("got %q", got)
		}
		if repo.tmplCreated == nil || repo.tmplCreated.Name != "p1" {
			t.Errorf("created=%+v", repo.tmplCreated)
		}
	})
	t.Run("添加失败", func(t *testing.T) {
		repo := &stubRepo{tmplCreateEr: errors.New("x")}
		svc := NewService(repo, nil)
		if got := svc.SavePromptTemplate(ctx, analysis.PromptTemplate{}); got != "添加失败" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("存在则更新", func(t *testing.T) {
		repo := &stubRepo{template: &analysis.PromptTemplate{ID: 3}}
		svc := NewService(repo, nil)
		if got := svc.SavePromptTemplate(ctx, analysis.PromptTemplate{ID: 3, Content: "c"}); got != "更新成功" {
			t.Errorf("got %q", got)
		}
		if repo.tmplUpdated == nil || repo.tmplUpdated.Content != "c" {
			t.Errorf("updated=%+v", repo.tmplUpdated)
		}
	})
	t.Run("更新失败", func(t *testing.T) {
		repo := &stubRepo{template: &analysis.PromptTemplate{ID: 3}, tmplUpdateEr: errors.New("x")}
		svc := NewService(repo, nil)
		if got := svc.SavePromptTemplate(ctx, analysis.PromptTemplate{ID: 3}); got != "更新失败" {
			t.Errorf("got %q", got)
		}
	})
}

func TestDeletePromptTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("不存在", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		if got := svc.DeletePromptTemplate(ctx, 7); got != "模板信息不存在" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("删除成功", func(t *testing.T) {
		repo := &stubRepo{template: &analysis.PromptTemplate{ID: 7}}
		svc := NewService(repo, nil)
		if got := svc.DeletePromptTemplate(ctx, 7); got != "删除成功" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("删除失败", func(t *testing.T) {
		repo := &stubRepo{template: &analysis.PromptTemplate{ID: 7}, tmplDeleteEr: errors.New("x")}
		svc := NewService(repo, nil)
		if got := svc.DeletePromptTemplate(ctx, 7); got != "删除失败" {
			t.Errorf("got %q", got)
		}
	})
}

func TestUpdateMultiAgentPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("成功_固定multi_agent类型", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo, nil)
		if got := svc.UpdateMultiAgentPrompt(ctx, "multi_news", "新闻分析师", "内容"); got != "更新成功" {
			t.Errorf("got %q", got)
		}
		if repo.upsertArgs != [4]string{"multi_news", "新闻分析师", "内容", "multi_agent"} {
			t.Errorf("upsertArgs=%v", repo.upsertArgs)
		}
	})
	t.Run("失败带错误详情", func(t *testing.T) {
		repo := &stubRepo{upsertErr: errors.New("db locked")}
		svc := NewService(repo, nil)
		if got := svc.UpdateMultiAgentPrompt(ctx, "k", "n", "c"); got != "更新失败: db locked" {
			t.Errorf("got %q", got)
		}
	})
}

func TestDeleteAndAlertTexts(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		repo *stubRepo
		call func(svc *Service) string
		want string
	}{
		{"删除AI结果成功", &stubRepo{}, func(s *Service) string { return s.DeleteAIResponseResult(ctx, 1) }, "删除成功"},
		{"删除AI结果失败", &stubRepo{delErr: errors.New("x")}, func(s *Service) string { return s.DeleteAIResponseResult(ctx, 1) }, "删除失败"},
		{"批量删除成功", &stubRepo{}, func(s *Service) string { return s.BatchDeleteAIResponseResult(ctx, []uint{1}) }, "删除成功"},
		{"批量删除失败", &stubRepo{batchDelErr: errors.New("x")}, func(s *Service) string { return s.BatchDeleteAIResponseResult(ctx, []uint{1}) }, "删除失败"},
		{"删除推荐成功", &stubRepo{}, func(s *Service) string { return s.DeleteAiRecommendStocks(ctx, 1) }, "删除成功"},
		{"删除推荐失败", &stubRepo{delErr: errors.New("x")}, func(s *Service) string { return s.DeleteAiRecommendStocks(ctx, 1) }, "删除失败"},
		{"预警更新成功", &stubRepo{}, func(s *Service) string { return s.UpdateAiRecommendStocksAlert(ctx, 1, true) }, "更新预警状态成功"},
		{"预警更新失败", &stubRepo{alertErr: errors.New("x")}, func(s *Service) string { return s.UpdateAiRecommendStocksAlert(ctx, 1, true) }, "更新预警状态失败"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.call(NewService(c.repo, nil)); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestGetAiRecommendStats(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2024, 1, 2, 10, 0, 0, 0, time.Local)

	t.Run("空数据返回空统计", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		stats, err := svc.GetAiRecommendStats(ctx)
		if err != nil || stats == nil {
			t.Fatalf("err=%v stats=%v", err, stats)
		}
		if stats.ByModel != nil || stats.BySector != nil || stats.DailyCount != nil {
			t.Errorf("空数据时应为 nil 切片: %+v", stats)
		}
	})

	t.Run("胜率与平均收益计算", func(t *testing.T) {
		repo := &stubRepo{recommends: []analysis.AiRecommendStocks{
			{ModelName: "gpt", BkName: "半导体", StockPrice: "100", StockCurrentPrice: "110", DataTime: &day}, // 胜,+10%
			{ModelName: "gpt", BkName: "半导体", StockPrice: "100", StockCurrentPrice: "90", DataTime: &day},  // 负,-10%
			{ModelName: "", BkName: "", StockPrice: "50", StockCurrentPrice: "50", DataTime: &day},            // unknown/未知,平价不计胜
			{ModelName: "gpt", BkName: "医药", StockPrice: "", StockCurrentPrice: "100", DataTime: &day},       // 原价缺失不计入收益
		}}
		svc := NewService(repo, nil)
		stats, err := svc.GetAiRecommendStats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// gpt: total=3 wins=1 → 33.3%;收益 (0.1-0.1+0)/2? —— 注意原实现 retSum/total(含无价格条目)
		var gpt *analysis.ModelStat
		var unknown *analysis.ModelStat
		for i := range stats.ByModel {
			switch stats.ByModel[i].ModelName {
			case "gpt":
				gpt = &stats.ByModel[i]
			case "unknown":
				unknown = &stats.ByModel[i]
			}
		}
		if gpt == nil || unknown == nil {
			t.Fatalf("ByModel=%+v", stats.ByModel)
		}
		if gpt.WinRate != 33.3 || gpt.Count != 3 {
			t.Errorf("gpt stat=%+v", gpt)
		}
		// retSum: (110-100)/100 + (90-100)/100 + 0 = 0;avgRet=0/3=0
		if gpt.AvgReturn != 0 {
			t.Errorf("gpt.AvgReturn=%v, want 0", gpt.AvgReturn)
		}
		if unknown.Count != 1 {
			t.Errorf("unknown stat=%+v", unknown)
		}
		// BySector: 半导体 2, 医药 1, 未知 1
		secMap := map[string]int{}
		for _, s := range stats.BySector {
			secMap[s.BkName] = s.Count
		}
		if secMap["半导体"] != 2 || secMap["医药"] != 1 || secMap["未知"] != 1 {
			t.Errorf("BySector=%v", secMap)
		}
		// DailyCount: 2024-01-02 × 4
		if len(stats.DailyCount) != 1 || stats.DailyCount[0].Date != "2024-01-02" || stats.DailyCount[0].Count != 4 {
			t.Errorf("DailyCount=%+v", stats.DailyCount)
		}
	})
}

func TestSaveAndGetAIResponseResult(t *testing.T) {
	ctx := context.Background()

	t.Run("保存字段映射", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo, nil)
		err := svc.SaveAIResponseResult(ctx, "600519", "贵州茅台", "内容", "chat1", "问题", "deepseek-v3")
		if err != nil {
			t.Fatal(err)
		}
		s := repo.savedAIResp
		if s == nil || s.StockCode != "600519" || s.StockName != "贵州茅台" || s.Content != "内容" ||
			s.ChatId != "chat1" || s.Question != "问题" || s.ModelName != "deepseek-v3" {
			t.Errorf("saved=%+v", s)
		}
	})

	t.Run("取最新_不存在返回nil", func(t *testing.T) {
		svc := NewService(&stubRepo{}, nil)
		got, err := svc.GetAIResponseResult(ctx, "600519")
		if err != nil || got != nil {
			t.Errorf("got=%v err=%v", got, err)
		}
	})
}

type recommendListStub struct {
	repository.AnalysisRepository
	page *analysis.AiRecommendStocksPageData
}

func (s *recommendListStub) GetAiRecommendStocksList(ctx context.Context, q analysis.AiRecommendStocksQuery) (*analysis.AiRecommendStocksPageData, error) {
	return s.page, nil
}

func TestGetAiRecommendStocksList_Enrich(t *testing.T) {
	ctx := context.Background()
	newPage := func() *analysis.AiRecommendStocksPageData {
		return &analysis.AiRecommendStocksPageData{List: []analysis.AiRecommendStocks{{StockCode: "600519"}}}
	}

	t.Run("enrichFn被调用", func(t *testing.T) {
		var enriched []analysis.AiRecommendStocks
		svc := NewService(&recommendListStub{page: newPage()}, func(items []analysis.AiRecommendStocks) {
			enriched = items
			items[0].StockCurrentPrice = "1688.5"
		})
		got, err := svc.GetAiRecommendStocksList(ctx, analysis.AiRecommendStocksQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(enriched) != 1 {
			t.Fatal("enrichFn 未被调用")
		}
		if got.List[0].StockCurrentPrice != "1688.5" {
			t.Errorf("补全未生效: %+v", got.List[0])
		}
	})

	t.Run("nil enrichFn不补全", func(t *testing.T) {
		svc := NewService(&recommendListStub{page: newPage()}, nil)
		got, err := svc.GetAiRecommendStocksList(ctx, analysis.AiRecommendStocksQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if got.List[0].StockCurrentPrice != "" {
			t.Errorf("不应补全: %+v", got.List[0])
		}
	})
}
