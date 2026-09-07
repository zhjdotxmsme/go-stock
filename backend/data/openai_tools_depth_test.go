package data

import "testing"

// resolveMaxToolDepth 决定带工具会话的终止边界（方案 Step 3.5）。
func TestResolveMaxToolDepth(t *testing.T) {
	tests := []struct {
		name string
		o    *OpenAi
		want int
	}{
		{"nil 会话回落默认", nil, defaultMaxToolDepth},
		{"未配置回落默认", &OpenAi{}, defaultMaxToolDepth},
		{"负数回落默认", &OpenAi{MaxToolDepth: -5}, defaultMaxToolDepth},
		{"正数生效", &OpenAi{MaxToolDepth: 3}, 3},
		{"大值生效", &OpenAi{MaxToolDepth: 50}, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveMaxToolDepth(tt.o); got != tt.want {
				t.Errorf("resolveMaxToolDepth() = %d, want %d", got, tt.want)
			}
		})
	}
}

// 默认上限必须收紧（旧值 200 轮可无限烧 token），且仍容纳链式工具流程。
func TestDefaultMaxToolDepthBounds(t *testing.T) {
	if defaultMaxToolDepth >= 200 {
		t.Errorf("defaultMaxToolDepth = %d, should be far below legacy 200", defaultMaxToolDepth)
	}
	if defaultMaxToolDepth < 5 {
		t.Errorf("defaultMaxToolDepth = %d, too small for chained flows like AI recommend (needs 5-7 rounds)", defaultMaxToolDepth)
	}
}
