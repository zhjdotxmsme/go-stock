package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("内存库创建失败: %v", err)
	}
	return db
}

// TestSQLiteMemorySaveRetrieve 保存/检索/计数 + 角色隔离 + topN + 收益率排序。
func TestSQLiteMemorySaveRetrieve(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	bull, err := NewSQLiteMemory(db, "bull_researcher")
	if err != nil {
		t.Fatalf("构造记忆库失败: %v", err)
	}
	t.Logf("FTS5 可用: %v", bull.FTSAvailable())

	records := []MemoryRecord{
		{SituationText: "600519 茅台 消费复苏 低估值", LessonText: "低估值消费龙头在复苏初期应积极介入", ReturnsPct: 8.5},
		{SituationText: "600519 茅台 高位放量 追涨", LessonText: "高位放量追涨易被埋", ReturnsPct: -6.2},
		{SituationText: "000001 平安银行 金融 低估", LessonText: "银行股波动小，适合防御", ReturnsPct: 2.1},
	}
	for _, r := range records {
		if err := bull.Save(ctx, r); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
	}

	// 计数
	n, err := bull.Count(ctx)
	if err != nil || n != 3 {
		t.Errorf("计数: got %d, err %v", n, err)
	}

	// 检索：命中 2 条茅台相关，按收益率倒序
	got, err := bull.Retrieve(ctx, "600519 茅台", 2)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("topN=2 应返回 2 条, got %d", len(got))
	}
	if got[0].ReturnsPct < got[1].ReturnsPct {
		t.Errorf("应按收益率倒序: %.1f vs %.1f", got[0].ReturnsPct, got[1].ReturnsPct)
	}
	for _, rec := range got {
		if !strings.Contains(rec.SituationText, "茅台") {
			t.Errorf("检索结果与情境无关: %s", rec.SituationText)
		}
		if rec.AgentRole != "bull_researcher" {
			t.Errorf("角色字段: %s", rec.AgentRole)
		}
		if rec.SituationHash == "" {
			t.Error("保存时应自动计算 situation_hash")
		}
	}

	// 角色隔离：bear 角色为空
	bear, err := NewSQLiteMemory(db, "bear_researcher")
	if err != nil {
		t.Fatalf("构造记忆库失败: %v", err)
	}
	got, err = bear.Retrieve(ctx, "600519 茅台", 2)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("角色隔离失败: bear 不应检索到 bull 的记忆, got %d 条", len(got))
	}
	n, _ = bear.Count(ctx)
	if n != 0 {
		t.Errorf("角色隔离失败: bear 计数 %d", n)
	}
}

// TestSQLiteMemoryLIKEFallback LIKE 降级：分词 OR 匹配 + 空检索词返回最新。
func TestSQLiteMemoryLIKEFallback(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	mem, err := NewSQLiteMemory(db, "trader")
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	// 强制禁用 FTS，验证 LIKE 路径
	mem.fts = false

	mem.Save(ctx, MemoryRecord{SituationText: "宁德时代 新能源 高增长", LessonText: "高增长赛道可容忍高估值", ReturnsPct: 12})
	mem.Save(ctx, MemoryRecord{SituationText: "贵州茅台 白酒 消费", LessonText: "消费白马防御属性强", ReturnsPct: 3})

	got, err := mem.Retrieve(ctx, "新能源", 5)
	if err != nil || len(got) != 1 {
		t.Fatalf("LIKE 分词检索: got %d 条, err %v", len(got), err)
	}
	if !strings.Contains(got[0].SituationText, "宁德时代") {
		t.Errorf("LIKE 检索结果错误: %s", got[0].SituationText)
	}

	// 空检索词：不按文本过滤，按收益率取 top
	got, err = mem.Retrieve(ctx, "", 1)
	if err != nil || len(got) != 1 || got[0].ReturnsPct != 12 {
		t.Errorf("空检索词应按收益率取 top1: %+v", got)
	}
}

// TestReflectorFourSteps 4 步反思流程：步骤串行、Prompt 包含前序输出、模型传递。
func TestReflectorFourSteps(t *testing.T) {
	var prompts []string
	call := func(ctx context.Context, model, prompt string) (string, error) {
		if model != "reflect-model" {
			t.Errorf("模型传递: got %q", model)
		}
		prompts = append(prompts, prompt)
		return []string{"因果分析结果", "改进建议结果", "经验总结结果", "浓缩查询句"}[len(prompts)-1], nil
	}
	r := NewReflector("reflect-model", call)
	in := ReflectionInput{
		AgentRole: "bull_researcher", StockCode: "600519", StockName: "贵州茅台",
		Situation: "消费复苏初期，PE 28", Decision: "买入，基于低估值与复苏逻辑", ReturnsPct: 8.5,
	}
	result, err := r.Reflect(context.Background(), in)
	if err != nil {
		t.Fatalf("反思失败: %v", err)
	}
	if len(prompts) != 4 {
		t.Fatalf("应调用 4 次 LLM, got %d", len(prompts))
	}
	// 各步 Prompt 包含前序输出（串行依赖）
	if !strings.Contains(prompts[1], "因果分析结果") {
		t.Error("第2步 Prompt 应包含推理评估输出")
	}
	if !strings.Contains(prompts[2], "因果分析结果") || !strings.Contains(prompts[2], "改进建议结果") {
		t.Error("第3步 Prompt 应包含前两步输出")
	}
	if !strings.Contains(prompts[3], "经验总结结果") {
		t.Error("第4步 Prompt 应包含经验总结输出")
	}
	// 第1步 Prompt 包含情境要素
	for _, want := range []string{"bull_researcher", "600519", "消费复苏初期", "8.50"} {
		if !strings.Contains(prompts[0], want) {
			t.Errorf("第1步 Prompt 缺少 %q", want)
		}
	}
	if result.Evaluation != "因果分析结果" || result.Improvement != "改进建议结果" ||
		result.Lesson != "经验总结结果" || result.Query != "浓缩查询句" {
		t.Errorf("结果字段映射错误: %+v", result)
	}
}

// TestReflectorStepFailure 中间步骤失败即返回错误并标注步骤。
func TestReflectorStepFailure(t *testing.T) {
	calls := 0
	call := func(ctx context.Context, model, prompt string) (string, error) {
		calls++
		if calls == 2 {
			return "", errors.New("llm down")
		}
		return "输出", nil
	}
	r := NewReflector("m", call)
	_, err := r.Reflect(context.Background(), ReflectionInput{Situation: "s", Decision: "d"})
	if err == nil || !strings.Contains(err.Error(), "第2步") {
		t.Errorf("应返回第2步错误: %v", err)
	}
}

// TestReflectAndRemember 反思后存库 + 检索闭环。
func TestReflectAndRemember(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	mem, err := NewSQLiteMemory(db, "invest_judge")
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	call := func(ctx context.Context, model, prompt string) (string, error) {
		if strings.Contains(prompt, "浓缩为一句话") {
			return "消费复苏期低估值白马应买入", nil
		}
		return "分析", nil
	}
	r := NewReflector("m", call)
	in := ReflectionInput{
		AgentRole: "invest_judge", StockCode: "600519",
		Situation: "600519 消费复苏 低估值", Decision: "买入", ReturnsPct: 8.5,
	}
	result, err := r.ReflectAndRemember(ctx, mem, in)
	if err != nil {
		t.Fatalf("反思存库失败: %v", err)
	}
	if result.Query != "消费复苏期低估值白马应买入" {
		t.Errorf("浓缩查询: %q", result.Query)
	}

	got, err := mem.Retrieve(ctx, "600519 消费复苏", 2)
	if err != nil || len(got) != 1 {
		t.Fatalf("存库后检索: got %d 条, err %v", len(got), err)
	}
	if got[0].LessonText != "消费复苏期低估值白马应买入" {
		t.Errorf("存库的 LessonText 应为浓缩查询句: %q", got[0].LessonText)
	}
	if got[0].SituationHash != HashSituation(in.Situation) {
		t.Error("situation_hash 与情境文本不一致")
	}
	if got[0].ReturnsPct != 8.5 {
		t.Errorf("收益率: %.1f", got[0].ReturnsPct)
	}
}

// TestHashSituation 哈希确定性。
func TestHashSituation(t *testing.T) {
	if HashSituation("abc") != HashSituation("abc") {
		t.Error("同文本应同哈希")
	}
	if HashSituation("abc") == HashSituation("abd") {
		t.Error("不同文本应不同哈希")
	}
	if len(HashSituation("abc")) != 64 {
		t.Errorf("SHA256 hex 长度: %d", len(HashSituation("abc")))
	}
}
