package scoring

import (
	"testing"
)

// selCandidates 构造 6 只选中（分数降序）+ 落选池的确定性场景。
// cutoff = 74（最后一名），旋转池 = 分数在 [72.5, 75.5] 且完成后分析的候选。
func selCandidates() (selected, pool []ScoredCandidate) {
	selected = []ScoredCandidate{
		{Code: "A1", Score: 95, PostAnalysisCompleted: true},   // 第一名，稳定
		{Code: "A2", Score: 80, PostAnalysisCompleted: true},   // 范围外
		{Code: "A3", Score: 75, PostAnalysisCompleted: true},   // 可旋出
		{Code: "A4", Score: 74.5, PostAnalysisCompleted: true}, // 可旋出
		{Code: "A5", Score: 74.2, PostAnalysisCompleted: true}, // 可旋出
		{Code: "A6", Score: 74, PostAnalysisCompleted: true},   // 可旋出（cutoff）
	}
	pool = []ScoredCandidate{
		{Code: "B1", Score: 74.8, PostAnalysisCompleted: true},  // 范围内，可旋入
		{Code: "B2", Score: 74.6, PostAnalysisCompleted: true},  // 范围内，可旋入
		{Code: "B3", Score: 74.1, PostAnalysisCompleted: false}, // 未完成后分析，不可旋入
		{Code: "B4", Score: 76.5, PostAnalysisCompleted: true},  // 范围外，不可旋入
		{Code: "B5", Score: 60, PostAnalysisCompleted: true},    // 范围外，不可旋入
	}
	return selected, pool
}

func codesOf(cands []ScoredCandidate) []string {
	codes := make([]string, len(cands))
	for i, c := range cands {
		codes[i] = c.Code
	}
	return codes
}

func containsCode(cands []ScoredCandidate, code string) bool {
	for _, c := range cands {
		if c.Code == code {
			return true
		}
	}
	return false
}

// TestRotateDeterministic 同 seed 同结果；不同 seed 结果（大概率）不同。
func TestRotateDeterministic(t *testing.T) {
	sel, pool := selCandidates()
	r1 := RotateSelection(sel, pool, "strategy-x", "2026-08-06")
	r2 := RotateSelection(sel, pool, "strategy-x", "2026-08-06")
	if len(r1) != len(sel) {
		t.Fatalf("旋转后数量不变: got %d, want %d", len(r1), len(sel))
	}
	for i := range r1 {
		if r1[i].Code != r2[i].Code {
			t.Fatalf("同 seed 应同结果: %v vs %v", codesOf(r1), codesOf(r2))
		}
	}
	// 不同 period 触发不同哈希序（极大概率不同结果；若偶然相同不算实现错误，仅记录）
	r3 := RotateSelection(sel, pool, "strategy-x", "2026-08-07")
	t.Logf("period=2026-08-06: %v, period=2026-08-07: %v", codesOf(r1), codesOf(r3))
}

// TestRotateFirstPlaceStable 第一名稳定，永不旋出（成员资格与位置都保持）。
func TestRotateFirstPlaceStable(t *testing.T) {
	sel, pool := selCandidates()
	for _, seed := range []string{"s1", "s2", "s3", "s4", "s5"} {
		r := RotateSelection(sel, pool, seed, "p")
		if r[0].Code != "A1" {
			t.Errorf("seed=%s: 第一名位置变化: %v", seed, codesOf(r))
		}
	}
	// 极端场景：第一名分数落入旋转范围，其成员资格仍不可被旋出
	sel2, pool2 := selCandidates()
	sel2[0].Score = 75
	for _, seed := range []string{"s1", "s2", "s3", "s4", "s5"} {
		r := RotateSelection(sel2, pool2, seed, "p")
		if !containsCode(r, "A1") {
			t.Errorf("seed=%s: 第一名 A1 被旋出: %v", seed, codesOf(r))
		}
	}
}

// TestRotateMaxCount 旋转数量上限为 output_count//2。
func TestRotateMaxCount(t *testing.T) {
	// 10 只选中全部在范围内且完成后分析，池内 10 只也都在范围内
	selected := make([]ScoredCandidate, 10)
	pool := make([]ScoredCandidate, 10)
	for i := 0; i < 10; i++ {
		selected[i] = ScoredCandidate{Code: string(rune('A' + i)), Score: 50, PostAnalysisCompleted: true}
		pool[i] = ScoredCandidate{Code: string(rune('a' + i)), Score: 50, PostAnalysisCompleted: true}
	}
	r := RotateSelection(selected, pool, "seed", "p")
	swappedIn := 0
	for _, c := range r {
		if c.Code[0] >= 'a' {
			swappedIn++
		}
	}
	if swappedIn > 5 {
		t.Errorf("output_count=10 时最多旋转 5 个位置, got %d", swappedIn)
	}
	if swappedIn == 0 {
		t.Error("全部满足条件时应发生旋转")
	}
	if len(r) != 10 {
		t.Errorf("旋转后数量不变: got %d", len(r))
	}
}

// TestRotatePoolConstraints 范围外/未完成后分析的候选不参与旋转。
func TestRotatePoolConstraints(t *testing.T) {
	sel, pool := selCandidates()
	r := RotateSelection(sel, pool, "strategy-x", "2026-08-06")
	for _, excluded := range []string{"B3", "B4", "B5"} {
		if containsCode(r, excluded) {
			t.Errorf("%s 不应被旋入: %v", excluded, codesOf(r))
		}
	}
	// 范围外的 A2 不应被旋出
	if !containsCode(r, "A2") {
		t.Errorf("范围外的 A2 不应被旋出: %v", codesOf(r))
	}
}

// TestRotateKeepsRanking 只改变成员资格，不改变相对排名（输出按分数降序）。
func TestRotateKeepsRanking(t *testing.T) {
	sel, pool := selCandidates()
	r := RotateSelection(sel, pool, "strategy-x", "2026-08-06")
	for i := 1; i < len(r); i++ {
		if r[i-1].Score < r[i].Score {
			t.Errorf("输出应保持分数降序: %v", r)
		}
	}
}

// TestRotateNoEligible 无可旋转候选时原样返回。
func TestRotateNoEligible(t *testing.T) {
	sel := []ScoredCandidate{
		{Code: "A1", Score: 95, PostAnalysisCompleted: true},
		{Code: "A2", Score: 80, PostAnalysisCompleted: true},
	}
	r := RotateSelection(sel, nil, "seed", "p")
	if len(r) != 2 || r[0].Code != "A1" || r[1].Code != "A2" {
		t.Errorf("无池候选时应原样返回: %v", codesOf(r))
	}

	// 池候选全部未完成后分析
	pool := []ScoredCandidate{{Code: "B1", Score: 80, PostAnalysisCompleted: false}}
	r = RotateSelection(sel, pool, "seed", "p")
	if r[1].Code != "A2" {
		t.Errorf("未完成后分析的池候选不应旋入: %v", codesOf(r))
	}
}
