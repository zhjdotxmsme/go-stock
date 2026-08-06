package scoring

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
)

// 种子化选股旋转（方案 §8.1 D9）：解决"每次运行选出的都是同一批股票"的问题。
//
// 规则：
//   - 尾部最多旋转 output_count//2 个位置（第一名稳定，永不旋出）；
//   - 旋转池：分数在 cutoff ± 1.5 范围内且完成所有后分析的候选
//     （cutoff = 当前选中结果最后一名的分数）；
//   - 用 SHA256(seed + period + code) 对可旋入候选做确定性排序；
//   - 只改变成员资格，不改变相对排名（输出仍按分数降序）。

// RotationRange 旋转池分数范围（cutoff ± 该值），方案固定为 1.5。
const RotationRange = 1.5

// ScoredCandidate 参与旋转的候选（由调用方从选股结果装配）。
type ScoredCandidate struct {
	Code                  string
	Score                 float64
	PostAnalysisCompleted bool // 是否完成所有后分析（D10），未完成不参与旋转
}

// RotateSelection 对按分数降序的选中结果做种子化旋转。
// selected 为当前选中列表（分数降序，长度即 output_count），pool 为落选候选池。
// seed/period 决定轮换的确定性（如 seed=策略标识，period=交易日期）。
// 返回新的选中列表（长度不变，分数降序）。
func RotateSelection(selected, pool []ScoredCandidate, seed, period string) []ScoredCandidate {
	outputCount := len(selected)
	maxRotate := outputCount / 2
	if maxRotate == 0 {
		return append([]ScoredCandidate{}, selected...)
	}
	cutoff := selected[outputCount-1].Score
	inPool := func(c ScoredCandidate) bool {
		return c.PostAnalysisCompleted && math.Abs(c.Score-cutoff) <= RotationRange
	}

	// 可旋出的尾部槽位（第一名 index 0 稳定，不参与）
	var slots []int
	for i := 1; i < outputCount; i++ {
		if inPool(selected[i]) {
			slots = append(slots, i)
		}
	}

	// 可旋入的落选候选，按 SHA256(seed+period+code) 确定性排序
	var incoming []ScoredCandidate
	for _, c := range pool {
		if inPool(c) {
			incoming = append(incoming, c)
		}
	}
	sort.SliceStable(incoming, func(i, j int) bool {
		return rotationHash(seed, period, incoming[i].Code) < rotationHash(seed, period, incoming[j].Code)
	})

	r := min(maxRotate, len(slots), len(incoming))
	if r == 0 {
		return append([]ScoredCandidate{}, selected...)
	}

	// 从最尾部旋出 r 个槽位成员，旋入哈希序前 r 个候选
	removed := make(map[int]bool, r)
	for _, idx := range slots[len(slots)-r:] {
		removed[idx] = true
	}
	kept := make([]ScoredCandidate, 0, outputCount)
	for i, c := range selected {
		if !removed[i] {
			kept = append(kept, c)
		}
	}
	kept = append(kept, incoming[:r]...)

	// 只改变成员资格，不改变相对排名：按分数稳定降序重排
	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].Score > kept[j].Score
	})
	return kept
}

// rotationHash 计算候选的确定性旋转键。
func rotationHash(seed, period, code string) string {
	sum := sha256.Sum256([]byte(seed + period + code))
	return hex.EncodeToString(sum[:])
}
