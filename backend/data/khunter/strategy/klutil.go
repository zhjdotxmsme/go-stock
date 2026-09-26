package strategy

import (
	"math"
	"strings"

	"go-stock/backend/models"
)

const eps = 1e-6

func feq(a, b float64) bool { return math.Abs(a-b) < eps }

func Closes(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

func Opens(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Open
	}
	return out
}

func Highs(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.High
	}
	return out
}

func Lows(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Low
	}
	return out
}

func Volumes(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = float64(b.Volume)
	}
	return out
}

// AvgVolumeBefore idx 日之前（不含 idx）前 n 日均量；数据不足返回 0
func AvgVolumeBefore(vols []float64, idx, n int) float64 {
	if idx-n < 0 {
		return 0
	}
	sum := 0.0
	for i := idx - n; i < idx; i++ {
		sum += vols[i]
	}
	return sum / float64(n)
}

// IsInvalidName 过滤 ST/退市/异常名称
func IsInvalidName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "ST") || strings.HasPrefix(name, "*ST") {
		return true
	}
	for _, kw := range []string{"退", "未知", "已退"} {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// PctChange 第 i 根涨幅（对前收）；i=0 返回 0
func PctChange(bars []models.KLineBar, i int) float64 {
	if i <= 0 || i >= len(bars) {
		return 0
	}
	prev := bars[i-1].Close
	if prev == 0 {
		return 0
	}
	return (bars[i].Close - prev) / prev
}

// SwingHighs 返回正序下标：high[i] 严格大于左右各 leftRight 根的最高价（对称分形，对齐 KHunter）。
// 右侧不足 leftRight 根的下标不参与（无未来函数）。
func SwingHighs(highs []float64, leftRight int) []int {
	var out []int
	for i := leftRight; i+leftRight < len(highs); i++ {
		ok := true
		for j := i - leftRight; j <= i+leftRight; j++ {
			if j == i {
				continue
			}
			if highs[j] >= highs[i] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// IsLocalLow low[i] 为前后 window 根内最低（容差 eps）
func IsLocalLow(lows []float64, i, window int) bool {
	if i-window < 0 || i+window >= len(lows) {
		return false
	}
	for j := i - window; j <= i+window; j++ {
		if lows[j] < lows[i]-eps {
			return false
		}
	}
	return true
}
