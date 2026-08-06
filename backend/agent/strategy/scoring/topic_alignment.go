package scoring

import "strings"

// TopicAlignmentFactor 主题匹配因子（4 参数）：
// 候选股票的行业/概念 token 集合与热点主题关键词 token 集合做重叠匹配，
// 得分 = 命中数 / 热点 token 总数 × 100（ capped ）。支持双向包含匹配
// （token 互为子串即算命中，兼容"机器人" vs "人形机器人"）。
// 无热点 token 时返回 0（该因子默认权重为 0，启用时必须提供热点 token）。
type TopicAlignmentFactor struct {
	PerHitBonus float64 // 每个命中的最低保障分（避免单个命中得分过低），默认 15
	FullScore   float64 // 满分值，默认 100
	MinTokenLen int     // 参与匹配的最短 token 长度，默认 2
	UseName     bool    // 是否将股票名称纳入匹配，默认 false
}

func NewTopicAlignmentFactor() *TopicAlignmentFactor {
	return &TopicAlignmentFactor{
		PerHitBonus: 15,
		FullScore:   100,
		MinTokenLen: 2,
		UseName:     false,
	}
}

func (f *TopicAlignmentFactor) Name() string { return "topic_alignment" }

// tokenize 将文本按常见分隔符切分为 token 集合，过滤过短 token。
func (f *TopicAlignmentFactor) tokenize(texts ...string) []string {
	seen := map[string]bool{}
	var tokens []string
	for _, text := range texts {
		for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
			switch r {
			case ' ', ',', '，', '、', '/', '\\', '|', ';', '；', '(', ')', '（', '）':
				return true
			}
			return false
		}) {
			tok = strings.TrimSpace(tok)
			if len([]rune(tok)) < f.MinTokenLen || seen[tok] {
				continue
			}
			seen[tok] = true
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// tokenMatch 双向包含匹配：完全相等或互为子串。
func tokenMatch(a, b string) bool {
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func (f *TopicAlignmentFactor) Score(input *FactorInput) FactorResult {
	if len(input.HotTopicTokens) == 0 {
		return FactorResult{Name: f.Name(), Score: 0, Detail: map[string]float64{"hits": 0}}
	}

	stockTexts := append([]string{input.Industry}, input.Concepts...)
	if f.UseName {
		stockTexts = append(stockTexts, input.Name)
	}
	stockTokens := f.tokenize(stockTexts...)
	hotTokens := f.tokenize(input.HotTopicTokens...)
	if len(hotTokens) == 0 {
		return FactorResult{Name: f.Name(), Score: 0, Detail: map[string]float64{"hits": 0}}
	}

	hits := 0.0
	for _, hot := range hotTokens {
		for _, st := range stockTokens {
			if tokenMatch(hot, st) {
				hits++
				break
			}
		}
	}

	score := hits / float64(len(hotTokens)) * f.FullScore
	if hits > 0 && score < f.PerHitBonus*hits {
		score = f.PerHitBonus * hits
	}

	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"hits":        hits,
			"hot_tokens":  float64(len(hotTokens)),
			"stock_token": float64(len(stockTokens)),
		},
	}
}
