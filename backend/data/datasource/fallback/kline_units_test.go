package fallback

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
)

func legacyBars() []data.KLineData {
	return []data.KLineData{
		{Day: "2026-11-10 15:00", Open: "10", High: "11", Low: "9", Close: "10", Volume: "120", Amount: "100000"},
		{Day: "2026-11-11 15:00", Open: "10", High: "11", Low: "9", Close: "10", Volume: "130", Amount: "110000"},
	}
}

// 东财是唯一明确"手"口径的来源：×100 归一为股，成交额明确为人民币元。
func TestConvertKLineDataNormalizesEastMoneyUnits(t *testing.T) {
	out := ConvertKLineData("sh600000", "101", "eastmoney_kline", legacyBars())
	if out == nil || len(out.Bars) != 2 {
		t.Fatalf("转换结果异常: %+v", out)
	}
	b := out.Bars[1]
	if b.Volume != 13000 || b.VolumeUnit != "share" {
		t.Fatalf("东财成交量应为 130 手×100=13000 股: got=(%d,%s)", b.Volume, b.VolumeUnit)
	}
	if b.Amount != 110000 || b.AmountUnit != "CNY" {
		t.Fatalf("东财成交额应明确 CNY: got=(%v,%s)", b.Amount, b.AmountUnit)
	}
}

// mootdx（通达信）成交量已是股：只声明单位，数值不得变动；无成交额字段，币种保持未确认。
func TestConvertKLineDataMootdxStaysShares(t *testing.T) {
	out := ConvertKLineData("sh600000", "101", "mootdx_kline", legacyBars())
	b := out.Bars[1]
	if b.Volume != 130 || b.VolumeUnit != "share" {
		t.Fatalf("mootdx 量已是股，不得换算: got=(%d,%s)", b.Volume, b.VolumeUnit)
	}
	if b.AmountUnit != "" {
		t.Fatalf("mootdx 未提供成交额，币种应保持未确认: got=%s", b.AmountUnit)
	}
}

// 未确认来源（如场内基金多上游混源）数值原样、单位 unknown，不得反推。
func TestConvertKLineDataUnknownSourceKeepsRaw(t *testing.T) {
	out := ConvertKLineData("sh510300", "101", "fund_kline", legacyBars())
	b := out.Bars[0]
	if b.Volume != 120 || b.VolumeUnit != "unknown" {
		t.Fatalf("未确认来源应保持原值+unknown: got=(%d,%s)", b.Volume, b.VolumeUnit)
	}
	if b.AmountUnit != "" {
		t.Fatalf("未确认来源币种应保持空: got=%s", b.AmountUnit)
	}
}

// 防双重换算闸：数据源边界已换算并声明单位后，落库边界必须原样透传。
func TestBarsFromKLineDataCarriesDeclaredUnitsWithoutRescaling(t *testing.T) {
	converted := ConvertKLineData("sh600000", "101", "eastmoney_kline", legacyBars())
	bars := datasource.BarsFromKLineData("sh600000", "101", "sync", true, converted)
	if len(bars) != 2 {
		t.Fatalf("落库条数异常: %d", len(bars))
	}
	if bars[1].Volume != 13000 || bars[1].VolumeUnit != "share" {
		t.Fatalf("已换算数据被二次换算: got=(%d,%s)", bars[1].Volume, bars[1].VolumeUnit)
	}
	if bars[1].AmountUnit != "CNY" {
		t.Fatalf("落库丢失成交额单位: got=%s", bars[1].AmountUnit)
	}
}

// 未声明单位的 bar 按落库来源映射补齐：已知来源只换算一次，未知来源保持 unknown。
func TestBarsFromKLineDataFillsUnitsFromSource(t *testing.T) {
	raw := &datasource.KLineData{Code: "sh600000", Period: "101", Bars: []datasource.KLineBar{
		{Time: mustParseLocal("2026-11-11 15:00"), Open: 10, High: 11, Low: 9, Close: 10, Volume: 130, Amount: 110000},
	}}
	em := datasource.BarsFromKLineData("sh600000", "101", "eastmoney_kline", true, raw)
	if em[0].Volume != 13000 || em[0].VolumeUnit != "share" {
		t.Fatalf("未声明单位的东财数据应换算一次: got=(%d,%s)", em[0].Volume, em[0].VolumeUnit)
	}
	tc := datasource.BarsFromKLineData("sh600000", "101", "tencent_kline", true, raw)
	if tc[0].Volume != 130 || tc[0].VolumeUnit != "unknown" {
		t.Fatalf("未知来源不得换算: got=(%d,%s)", tc[0].Volume, tc[0].VolumeUnit)
	}
}

func mustParseLocal(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}
