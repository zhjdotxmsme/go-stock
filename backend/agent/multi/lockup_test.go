package multi

import (
	"testing"

	"go-stock/backend/data"

	"github.com/stretchr/testify/assert"
)

func TestFormatF10Records(t *testing.T) {
	resp := &data.F10GenericResp{
		Result: &data.F10Result{
			Data: []map[string]any{
				{"FREE_SHARES_DATE": "2024-06-01", "FREE_SHARES_NUM": 1000000, "HOLDER_NAME": "大股东A"},
				{"FREE_SHARES_DATE": "2024-12-01", "FREE_SHARES_NUM": 500000, "HOLDER_NAME": "大股东B"},
			},
		},
	}

	out := formatF10Records(resp, 10)
	assert.Contains(t, out, "大股东A")
	assert.Contains(t, out, "2024-06-01")
	assert.Contains(t, out, "大股东B")
}

func TestFormatF10RecordsEmpty(t *testing.T) {
	assert.Equal(t, "暂无数据\n", formatF10Records(nil, 10))
	assert.Equal(t, "暂无数据\n", formatF10Records(&data.F10GenericResp{Result: &data.F10Result{Data: []map[string]any{}}}, 10))
}

func TestFormatF10RecordsMax(t *testing.T) {
	rows := make([]map[string]any, 0, 20)
	for i := 0; i < 20; i++ {
		rows = append(rows, map[string]any{"idx": i})
	}
	resp := &data.F10GenericResp{Result: &data.F10Result{Data: rows}}
	out := formatF10Records(resp, 5)
	assert.Contains(t, out, "idx: 0")
	assert.Contains(t, out, "idx: 4")
	assert.NotContains(t, out, "idx: 5")
}
