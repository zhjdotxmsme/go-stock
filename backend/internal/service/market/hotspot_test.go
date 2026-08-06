package market

import "testing"

func analyzer() *HotspotAnalyzer {
	return NewHotspotAnalyzer(DefaultHotspotConfig())
}

// TestClassifyStage 5 阶段生命周期分类用例。
func TestClassifyStage(t *testing.T) {
	cases := []struct {
		name  string
		input HotspotInput
		want  LifecycleStage
	}{
		{"状态明示降温", HotspotInput{State: "降温中", Latest: 70, Trend: 3}, StageFading},
		{"降温分过高", HotspotInput{Cooling: 25, Latest: 60, Trend: 2}, StageFading},
		{"趋势转负且热度低", HotspotInput{Trend: -6, Latest: 40}, StageFading},
		{"状态明示分歧", HotspotInput{State: "高位分歧", Latest: 70, Trend: 1}, StageDiverging},
		{"高热降温抬头", HotspotInput{Latest: 70, Cooling: 10, Trend: 1}, StageDiverging},
		{"加速主升", HotspotInput{Trend: 8, Latest: 75, PersistenceDays: 4, WatchCount: 30}, StageAccelerating},
		{"确认扩散", HotspotInput{Trend: 3, Latest: 60, PersistenceDays: 2, WatchCount: 20}, StageSpreading},
		{"初次异动", HotspotInput{Trend: 2, Latest: 48, PersistenceDays: 1, WatchCount: 5}, StageEmerging},
		{"观察不足不扩散", HotspotInput{Trend: 3, Latest: 60, PersistenceDays: 2, WatchCount: 3}, StageEmerging},
		{"趋势走弱兜底退潮", HotspotInput{Trend: -1, Latest: 40, Cooling: 0}, StageFading},
		{"退潮优先于主升", HotspotInput{Trend: 8, Latest: 75, PersistenceDays: 4, Cooling: 25}, StageFading},
		{"分歧优先于主升", HotspotInput{Trend: 8, Latest: 75, PersistenceDays: 4, Cooling: 10}, StageDiverging},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := analyzer().ClassifyStage(&tc.input)
			if got != tc.want {
				t.Errorf("ClassifyStage: got %s, want %s (reason=%s)", got, tc.want, reason)
			}
			if reason == "" {
				t.Error("应给出分类理由")
			}
		})
	}
}

// TestAssignRole 5 角色边界用例（topScore=80，前3龙头阈值 max(68, 80-8)=72）。
func TestAssignRole(t *testing.T) {
	a := analyzer()
	cases := []struct {
		name     string
		rank     int
		score    float64
		chg      float64
		topScore float64
		want     StockRole
	}{
		{"第1名分达标", 1, 70, 2, 80, RoleLeader},
		{"第1名分刚好69.9", 1, 69.9, 8, 80, RoleSupporter}, // 不满足龙头两路径，落助攻
		{"前3分涨幅双达标", 2, 73, 5, 80, RoleLeader},        // 73 ≥ max(68,72)
		{"前3涨幅不足", 3, 73, 4.9, 80, RoleSupporter},     // 涨幅 <5 落选龙头
		{"前3分不足", 2, 71, 6, 80, RoleSupporter},        // 71 < 72
		{"top低阈值取68", 2, 69, 6, 70, RoleLeader},       // max(68, 62)=68
		{"助攻边界", 4, 62, 3, 80, RoleSupporter},
		{"助攻分不足落补涨", 4, 61.9, 3, 80, RoleLaggard},
		{"补涨边界", 5, 48, 0, 80, RoleLaggard},
		{"补涨涨幅负落后排", 5, 50, -1, 80, RoleBackrow},
		{"后排边界", 6, 38, -2, 80, RoleBackrow},
		{"掉队", 7, 37.9, -3, 80, RoleDropped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := a.AssignRole(tc.rank, tc.score, tc.chg, tc.topScore); got != tc.want {
				t.Errorf("AssignRole(rank=%d, score=%.1f, chg=%.1f, top=%.0f): got %s, want %s",
					tc.rank, tc.score, tc.chg, tc.topScore, got, tc.want)
			}
		})
	}
}

// TestAnalyze 集成：阶段分类 + 全股票角色分配（topScore 取自第 1 名）。
func TestAnalyze(t *testing.T) {
	in := &HotspotInput{
		Name: "AI算力", Latest: 75, Trend: 8, PersistenceDays: 4, WatchCount: 50,
	}
	stocks := []HotStock{
		{Code: "A", Rank: 1, Score: 85, ChangePct: 7},  // 龙头
		{Code: "B", Rank: 2, Score: 78, ChangePct: 6},  // 龙头（≥max(68,77)）
		{Code: "C", Rank: 3, Score: 70, ChangePct: 4},  // 助攻
		{Code: "D", Rank: 4, Score: 50, ChangePct: 1},  // 补涨
		{Code: "E", Rank: 5, Score: 30, ChangePct: -2}, // 掉队
	}
	r := analyzer().Analyze(in, stocks)
	if r.Stage != StageAccelerating {
		t.Errorf("阶段: got %s, want 加速主升", r.Stage)
	}
	wantRoles := []StockRole{RoleLeader, RoleLeader, RoleSupporter, RoleLaggard, RoleDropped}
	if len(r.Stocks) != len(wantRoles) {
		t.Fatalf("角色分配数量: %d", len(r.Stocks))
	}
	for i, want := range wantRoles {
		if r.Stocks[i].Role != want {
			t.Errorf("股票 %s 角色: got %s, want %s", r.Stocks[i].Code, r.Stocks[i].Role, want)
		}
	}
}
