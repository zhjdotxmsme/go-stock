// Package indicator 提供选股引擎所需的纯函数技术指标实现。
//
// 算法移植自前端 K 线指标库 frontend/src/components/kline/calc.ts（同名函数、
// 相同默认参数），由黄金值对照测试保证前后端数值一致（golden_*_test.go）。
//
// 约定：
//   - 所有序列函数返回与输入等长的切片；预热期（数据不足）位置填 math.NaN()。
//   - 取最新有效值用 LastValid；返回 false 表示整条序列无有效值（数据不足）。
//   - 本包纯函数、无 init 副作用、仅依赖标准库（math），可被 data 与 scoring 安全引用。
package indicator

import "math"

// LastValid 返回序列中最后一个非 NaN 的值；全 NaN 或空序列返回 false。
func LastValid(s []float64) (float64, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if !math.IsNaN(s[i]) {
			return s[i], true
		}
	}
	return 0, false
}

// At 返回序列倒数第 offset 个有效值（offset=0 即最新），不足返回 false。
func At(s []float64, offset int) (float64, bool) {
	if offset < 0 {
		return 0, false
	}
	idx := len(s) - 1 - offset
	if idx < 0 || math.IsNaN(s[idx]) {
		return 0, false
	}
	return s[idx], true
}

// CrossOver 判断最近一根 K 线 a 上穿 b（前一根 a<=b，当前 a>b）。
// 任一序列最新两根含 NaN 返回 false。
func CrossOver(a, b []float64) bool {
	n := len(a)
	if n < 2 || len(b) < n {
		return false
	}
	a1, a0 := a[n-2], a[n-1]
	b1, b0 := b[n-2], b[n-1]
	if isNaN4(a1, a0, b1, b0) {
		return false
	}
	return a1 <= b1 && a0 > b0
}

// CrossUnder 判断最近一根 K 线 a 下穿 b。
func CrossUnder(a, b []float64) bool {
	n := len(a)
	if n < 2 || len(b) < n {
		return false
	}
	a1, a0 := a[n-2], a[n-1]
	b1, b0 := b[n-2], b[n-1]
	if isNaN4(a1, a0, b1, b0) {
		return false
	}
	return a1 >= b1 && a0 < b0
}

// HighestIn 返回最近 n 根内的最高有效值；无有效值返回 false。
func HighestIn(s []float64, n int) (float64, bool) {
	if n <= 0 {
		n = len(s)
	}
	start := len(s) - n
	if start < 0 {
		start = 0
	}
	found := false
	best := math.Inf(-1)
	for i := start; i < len(s); i++ {
		if !math.IsNaN(s[i]) && s[i] > best {
			best = s[i]
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return best, true
}

// LowestIn 返回最近 n 根内的最低有效值；无有效值返回 false。
func LowestIn(s []float64, n int) (float64, bool) {
	if n <= 0 {
		n = len(s)
	}
	start := len(s) - n
	if start < 0 {
		start = 0
	}
	found := false
	best := math.Inf(1)
	for i := start; i < len(s); i++ {
		if !math.IsNaN(s[i]) && s[i] < best {
			best = s[i]
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return best, true
}

func isNaN4(a, b, c, d float64) bool {
	return math.IsNaN(a) || math.IsNaN(b) || math.IsNaN(c) || math.IsNaN(d)
}
