package risk

import "strings"

// SectorBucket 板块桶：同一桶内的标的视为同质化持仓。
type SectorBucket struct {
	Name     string   // 桶名
	Keywords []string // 匹配关键词（对行业/概念做包含匹配）
}

// DefaultSectorBuckets 方案 §8.1 D3 规格的 7 个板块桶。
// 匹配按桶顺序取第一个命中，桶内任一关键词命中即归入该桶。
var DefaultSectorBuckets = []SectorBucket{
	{Name: "金融", Keywords: []string{"券商", "银行", "保险", "金融"}},
	{Name: "地产链", Keywords: []string{"地产", "房地产", "建材", "家居", "物业"}},
	{Name: "新能源", Keywords: []string{"新能源", "光伏", "锂电", "电池", "储能"}},
	{Name: "AI算力", Keywords: []string{"AI算力", "算力", "数据中心", "服务器", "光模块"}},
	{Name: "消费", Keywords: []string{"白酒", "食品", "家电", "零售", "消费"}},
	{Name: "医药", Keywords: []string{"医药", "医疗", "创新药"}},
	{Name: "半导体", Keywords: []string{"半导体", "芯片"}},
}

// Holding 组合中一只持仓的板块归属信息。
type Holding struct {
	Code     string
	Industry string   // 所属行业
	Concepts []string // 所属概念列表
}

// DiversityResult 组合多样性评估结果。
type DiversityResult struct {
	Penalty     float64             // 集中度总罚分（超额只数 × 每只罚分）
	Assignments map[string]string   // 股票代码 -> 板块桶名（未匹配到桶的不出现）
	Excess      map[string][]string // 板块桶名 -> 超额股票代码（按传入顺序，前 N 只之外的部分）
}

// PortfolioDiversity 组合多样性约束：同板块桶最多 MaxPerBucket 只，超额每只扣分。
type PortfolioDiversity struct {
	MaxPerBucket  int            // 同桶最大只数，默认 1
	ExcessPenalty float64        // 超额每只罚分，默认 4.0
	Buckets       []SectorBucket // 板块桶定义，默认 DefaultSectorBuckets
}

// NewPortfolioDiversity 构造默认参数的组合多样性评估器。
func NewPortfolioDiversity() *PortfolioDiversity {
	return &PortfolioDiversity{
		MaxPerBucket:  1,
		ExcessPenalty: 4.0,
		Buckets:       DefaultSectorBuckets,
	}
}

// BucketFor 返回标的所属板块桶名；行业与概念做关键词包含匹配，
// 按桶顺序取第一个命中，无匹配返回空串。
func (d *PortfolioDiversity) BucketFor(industry string, concepts []string) string {
	texts := append([]string{industry}, concepts...)
	for _, bucket := range d.Buckets {
		for _, kw := range bucket.Keywords {
			for _, text := range texts {
				if text != "" && strings.Contains(text, kw) {
					return bucket.Name
				}
			}
		}
	}
	return ""
}

// Evaluate 评估一组持仓的板块集中度。
// 同桶标的按传入顺序前 MaxPerBucket 只免罚，其余每只扣 ExcessPenalty 分。
func (d *PortfolioDiversity) Evaluate(holdings []Holding) DiversityResult {
	result := DiversityResult{
		Assignments: make(map[string]string, len(holdings)),
		Excess:      map[string][]string{},
	}
	maxPerBucket := d.MaxPerBucket
	if maxPerBucket <= 0 {
		maxPerBucket = 1
	}

	counts := map[string]int{}
	for _, h := range holdings {
		bucket := d.BucketFor(h.Industry, h.Concepts)
		if bucket == "" {
			continue
		}
		result.Assignments[h.Code] = bucket
		counts[bucket]++
		if counts[bucket] > maxPerBucket {
			result.Penalty += d.ExcessPenalty
			result.Excess[bucket] = append(result.Excess[bucket], h.Code)
		}
	}
	return result
}
