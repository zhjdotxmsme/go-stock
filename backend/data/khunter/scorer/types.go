package scorer

// DimScore 单维度评分结果
type DimScore struct {
	Score    float64
	Veto     bool   // 一票否决
	Reason   string // 否决原因或说明
	Degraded bool   // 数据缺失按中性分处理
	Detail   map[string]any
}

// Hit 一次策略命中
type Hit struct {
	Name   string
	Weight int
}
