package scorer

import (
	"encoding/json"
	"os"
)

// 技术面一票否决组合（对齐 KHunter veto_config）
var technicalVetoPair = [2]string{"M头策略", "多死叉共振策略"}

// TechnicalScorer 技术面得分 = Σ 命中策略权重（Overrides 可按名称覆盖权重）
type TechnicalScorer struct {
	Overrides map[string]int
}

func (s TechnicalScorer) Score(hits []Hit) DimScore {
	total := 0.0
	names := map[string]bool{}
	for _, h := range hits {
		w := h.Weight
		if ow, ok := s.Overrides[h.Name]; ok {
			w = ow
		}
		total += float64(w)
		names[h.Name] = true
	}
	d := DimScore{Score: total, Detail: map[string]any{"hits": len(hits)}}
	if names[technicalVetoPair[0]] && names[technicalVetoPair[1]] {
		d.Veto = true
		d.Score = -100
		d.Reason = "M头策略与多死叉共振策略同时命中"
	}
	return d
}

// LoadTechnicalOverrides 加载权重覆盖配置；文件不存在或解析失败返回 nil
func LoadTechnicalOverrides(path string) map[string]int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Overrides map[string]int `json:"overrides"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return cfg.Overrides
}
