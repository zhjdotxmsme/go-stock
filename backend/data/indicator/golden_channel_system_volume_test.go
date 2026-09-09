package indicator

// 黄金值对照测试（通道/系统/量能族）：读 testdata/golden_a2.json
// （由 testdata/gen_a2.mjs 直接 import frontend/src/components/kline/calc.ts 原函数生成），
// 逐 case 对比本包移植实现：
//   - NaN 位置必须一致（JS null ↔ Go NaN；SATS 的 tqi 预热期为 0，与 JS 一致）；
//   - 非 NaN 值满足 |got-want| <= 1e-8（绝对）或 <= 1e-6*|want|（相对）；
//   - bool 序列逐位一致；SMCEvents 逐事件对比（Index/Type/Bullish 精确，Level 同容差）。
// 输入为 JSON 内自带的确定性合成 OHLCV（150 根，固定种子 LCG）。

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// a2Series 支持 JSON null → NaN 的反序列化。
type a2Series []float64

func (s *a2Series) UnmarshalJSON(b []byte) error {
	var raw []*float64
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(a2Series, len(raw))
	for i, v := range raw {
		if v == nil {
			out[i] = math.NaN()
		} else {
			out[i] = *v
		}
	}
	*s = out
	return nil
}

type a2OHLCV struct {
	Open   a2Series `json:"open"`
	High   a2Series `json:"high"`
	Low    a2Series `json:"low"`
	Close  a2Series `json:"close"`
	Volume a2Series `json:"volume"`
}

type a2Event struct {
	Index   int     `json:"index"`
	Type    string  `json:"type"`
	Bullish bool    `json:"bullish"`
	Level   float64 `json:"level"`
}

type a2Case struct {
	Fn      string          `json:"fn"`
	Label   string          `json:"label"`
	Outputs json.RawMessage `json:"outputs"`
}

type a2File struct {
	Bars  int      `json:"bars"`
	Data  a2OHLCV  `json:"data"`
	Cases []a2Case `json:"cases"`
}

func a2Load(t *testing.T) a2File {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden_a2.json"))
	if err != nil {
		t.Fatalf("读取 golden_a2.json 失败: %v", err)
	}
	var gf a2File
	if err := json.Unmarshal(b, &gf); err != nil {
		t.Fatalf("解析 golden_a2.json 失败: %v", err)
	}
	if len(gf.Cases) == 0 || len(gf.Data.Close) == 0 {
		t.Fatalf("golden_a2.json 为空: cases=%d bars=%d", len(gf.Cases), len(gf.Data.Close))
	}
	return gf
}

// a2GoldenClose 容差判定：NaN 位置必须一致；非 NaN 值 1e-8 绝对或 1e-6 相对。
func a2GoldenClose(got, want float64) bool {
	if math.IsNaN(want) || math.IsNaN(got) {
		return math.IsNaN(want) && math.IsNaN(got)
	}
	diff := math.Abs(got - want)
	return diff <= 1e-8 || diff <= 1e-6*math.Abs(want)
}

func a2CompareSeries(t *testing.T, name string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: 序列长度不一致 got=%d want=%d", name, len(got), len(want))
	}
	maxDiff := 0.0
	nanMismatch := 0
	for i := range want {
		if !a2GoldenClose(got[i], want[i]) {
			switch {
			case math.IsNaN(want[i]):
				nanMismatch++
				if nanMismatch <= 3 {
					t.Errorf("%s[%d]: 期望 NaN（JS null），got=%v", name, i, got[i])
				}
			case math.IsNaN(got[i]):
				t.Errorf("%s[%d]: 意外 NaN，want=%v", name, i, want[i])
			default:
				t.Errorf("%s[%d]: got=%.12g want=%.12g diff=%.3g 超出容差", name, i, got[i], want[i], math.Abs(got[i]-want[i]))
			}
		}
		if d := math.Abs(got[i] - want[i]); !math.IsNaN(d) && d > maxDiff {
			maxDiff = d
		}
	}
	if nanMismatch > 3 {
		t.Errorf("%s: 共 %d 处 NaN 位置不一致", name, nanMismatch)
	}
	t.Logf("%s: pass (max|diff|=%.3g)", name, maxDiff)
}

func a2CompareBools(t *testing.T, name string, got, want []bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: 序列长度不一致 got=%d want=%d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d]: got=%v want=%v", name, i, got[i], want[i])
		}
	}
	t.Logf("%s: pass", name)
}

func a2Outputs(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析 outputs 失败: %v", err)
	}
	return m
}

func a2SeriesOf(t *testing.T, outs map[string]json.RawMessage, key string) a2Series {
	t.Helper()
	raw, ok := outs[key]
	if !ok {
		t.Fatalf("outputs 缺少 %q", key)
	}
	var s a2Series
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("解析 outputs.%s 失败: %v", key, err)
	}
	return s
}

func a2BoolsOf(t *testing.T, outs map[string]json.RawMessage, key string) []bool {
	t.Helper()
	raw, ok := outs[key]
	if !ok {
		t.Fatalf("outputs 缺少 %q", key)
	}
	var s []bool
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("解析 outputs.%s 失败: %v", key, err)
	}
	return s
}

func a2ParseInts(t *testing.T, label string) []int {
	t.Helper()
	parts := strings.Split(label, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			t.Fatalf("label %q 解析 int 失败: %v", label, err)
		}
		out = append(out, n)
	}
	return out
}

func a2ParseFloats(t *testing.T, label string) []float64 {
	t.Helper()
	parts := strings.Split(label, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			t.Fatalf("label %q 解析 float 失败: %v", label, err)
		}
		out = append(out, f)
	}
	return out
}

// a2SatsOptsFor 与 gen_a2.mjs 的 satsVariants 保持一致。
func a2SatsOptsFor(label string) satsOpts {
	o := defaultSatsOpts()
	switch label {
	case "defaults":
		// calc.ts 全默认
	case "short":
		o.atrBaselineLen = 40
		o.erLen = 10
		o.tqiStructLen = 14
		o.tqiMomLen = 8
		o.volLen = 12
	case "legacy":
		o.useTqi = false
		o.useAdaptive = false
		o.useAsymBands = false
		o.smoothMult = false
		o.useCharFlip = false
		o.useEffAtr = false
	}
	return o
}

// a2ParseMixed 解析“前 nInt 个为 int、其余为 float”的逗号分隔标签，
// 避免 "12,6,2"（末位 float 恰为整数值）被整体当 float 解析的歧义。
func a2ParseMixed(t *testing.T, label string, nInt int) (ints []int, floats []float64) {
	t.Helper()
	parts := strings.Split(label, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if i < nInt {
			n, err := strconv.Atoi(p)
			if err != nil {
				t.Fatalf("label %q 解析 int 失败: %v", label, err)
			}
			ints = append(ints, n)
		} else {
			f, err := strconv.ParseFloat(p, 64)
			if err != nil {
				t.Fatalf("label %q 解析 float 失败: %v", label, err)
			}
			floats = append(floats, f)
		}
	}
	return ints, floats
}

func a2RunCase(t *testing.T, h, l, c, o, v []float64, tc a2Case) {
	t.Helper()
	outs := a2Outputs(t, tc.Outputs)

	switch tc.Fn {
	case "Keltner": // JS keltnerChannelValues → Keltner
		ip, fp := a2ParseMixed(t, tc.Label, 2)
		mid, up, lo := Keltner(h, l, c, ip[0], ip[1], fp[0])
		a2CompareSeries(t, "mid", mid, a2SeriesOf(t, outs, "mid"))
		a2CompareSeries(t, "upper", up, a2SeriesOf(t, outs, "upper"))
		a2CompareSeries(t, "lower", lo, a2SeriesOf(t, outs, "lower"))
	case "Donchian": // JS donchianChannelValues → Donchian
		up, mid, lo := Donchian(h, l, a2ParseInts(t, tc.Label)[0])
		a2CompareSeries(t, "upper", up, a2SeriesOf(t, outs, "upper"))
		a2CompareSeries(t, "mid", mid, a2SeriesOf(t, outs, "mid"))
		a2CompareSeries(t, "lower", lo, a2SeriesOf(t, outs, "lower"))
	case "SuperTrend": // JS supertrendValues → SuperTrend
		ip, fp := a2ParseMixed(t, tc.Label, 1)
		line, bull := SuperTrend(h, l, c, ip[0], fp[0])
		a2CompareSeries(t, "line", line, a2SeriesOf(t, outs, "line"))
		a2CompareBools(t, "bull", bull, a2BoolsOf(t, outs, "bull"))
	case "TTMSqueeze": // JS ttmSqueezeValues → TTMSqueeze
		ip, fp := a2ParseMixed(t, tc.Label, 3)
		sq, mo := TTMSqueeze(h, l, c, ip[0], ip[1], ip[2], fp[0], fp[1])
		a2CompareBools(t, "squeezeOn", sq, a2BoolsOf(t, outs, "squeezeOn"))
		a2CompareSeries(t, "momentum", mo, a2SeriesOf(t, outs, "momentum"))
	case "Ichimoku": // JS ichimokuValues → Ichimoku（spanB=JS senkouB）
		ip := a2ParseInts(t, tc.Label)
		tenkan, kijun, spanA, spanB := Ichimoku(h, l, c, ip[0], ip[1], ip[2])
		a2CompareSeries(t, "tenkan", tenkan, a2SeriesOf(t, outs, "tenkan"))
		a2CompareSeries(t, "kijun", kijun, a2SeriesOf(t, outs, "kijun"))
		a2CompareSeries(t, "spanA", spanA, a2SeriesOf(t, outs, "spanA"))
		a2CompareSeries(t, "spanB", spanB, a2SeriesOf(t, outs, "spanB"))
	case "Alligator": // JS alligatorValues → Alligator
		ip := a2ParseInts(t, tc.Label)
		jaw, teeth, lips := Alligator(h, l, c, ip[0], ip[1], ip[2], ip[3], ip[4], ip[5])
		a2CompareSeries(t, "jaw", jaw, a2SeriesOf(t, outs, "jaw"))
		a2CompareSeries(t, "teeth", teeth, a2SeriesOf(t, outs, "teeth"))
		a2CompareSeries(t, "lips", lips, a2SeriesOf(t, outs, "lips"))
	case "PSAR": // JS sarValues → PSAR
		fp := a2ParseFloats(t, tc.Label)
		sar, bull := PSAR(h, l, c, fp[0], fp[1])
		a2CompareSeries(t, "sar", sar, a2SeriesOf(t, outs, "sar"))
		a2CompareBools(t, "bull", bull, a2BoolsOf(t, outs, "bull"))
	case "SATS": // JS satsValues → SATS（defaults 走导出契约；short/legacy 走 satsRun）
		switch tc.Label {
		case "defaults":
			line, bull, tqi := SATS(h, l, c, v)
			st, up, lo, tq2, bull2 := satsRun(h, l, c, v, defaultSatsOpts())
			a2CompareSeries(t, "line(SATS)", line, a2SeriesOf(t, outs, "line"))
			a2CompareBools(t, "bull(SATS)", bull, a2BoolsOf(t, outs, "bull"))
			a2CompareSeries(t, "tqi(SATS)", tqi, a2SeriesOf(t, outs, "tqi"))
			// 导出契约 SATS() 与内部 satsRun 必须同源
			a2CompareSeries(t, "line(satsRun)", st, a2SeriesOf(t, outs, "line"))
			a2CompareSeries(t, "tqi(satsRun)", tq2, a2SeriesOf(t, outs, "tqi"))
			a2CompareBools(t, "bull(satsRun)", bull2, a2BoolsOf(t, outs, "bull"))
			a2CompareSeries(t, "upper", up, a2SeriesOf(t, outs, "upper"))
			a2CompareSeries(t, "lower", lo, a2SeriesOf(t, outs, "lower"))
		case "short", "legacy":
			line, up, lo, tqi, bull := satsRun(h, l, c, v, a2SatsOptsFor(tc.Label))
			a2CompareSeries(t, "line", line, a2SeriesOf(t, outs, "line"))
			a2CompareSeries(t, "upper", up, a2SeriesOf(t, outs, "upper"))
			a2CompareSeries(t, "lower", lo, a2SeriesOf(t, outs, "lower"))
			a2CompareBools(t, "bull", bull, a2BoolsOf(t, outs, "bull"))
			a2CompareSeries(t, "tqi", tqi, a2SeriesOf(t, outs, "tqi"))
		default:
			t.Fatalf("未知 SATS label %q", tc.Label)
		}
	case "SMCEvents": // JS smcValues bosLines+chochLines → SMCEvents
		ip := a2ParseInts(t, tc.Label)
		var wantEv struct {
			Events []a2Event `json:"events"`
		}
		if err := json.Unmarshal(tc.Outputs, &wantEv); err != nil {
			t.Fatalf("解析 events 失败: %v", err)
		}
		got := SMCEvents(o, h, l, c, ip[0], ip[1])
		if len(got) != len(wantEv.Events) {
			t.Fatalf("events 数量不一致 got=%d want=%d", len(got), len(wantEv.Events))
		}
		for i := range got {
			g, w := got[i], wantEv.Events[i]
			if g.Index != w.Index || g.Type != w.Type || g.Bullish != w.Bullish || !a2GoldenClose(g.Level, w.Level) {
				t.Fatalf("events[%d]: got {index:%d type:%s bullish:%v level:%v} want {index:%d type:%s bullish:%v level:%v}",
					i, g.Index, g.Type, g.Bullish, g.Level, w.Index, w.Type, w.Bullish, w.Level)
			}
		}
		t.Logf("events: pass (%d 个事件)", len(got))
	case "ElderRay": // JS elderRayValues → ElderRay
		bp, br := ElderRay(h, l, c, a2ParseInts(t, tc.Label)[0])
		a2CompareSeries(t, "bullPower", bp, a2SeriesOf(t, outs, "bullPower"))
		a2CompareSeries(t, "bearPower", br, a2SeriesOf(t, outs, "bearPower"))
	case "OBV": // JS obvValues → OBV
		a2CompareSeries(t, "obv", OBV(c, v), a2SeriesOf(t, outs, "obv"))
	case "MFI": // JS mfiValues → MFI
		a2CompareSeries(t, "mfi", MFI(h, l, c, v, a2ParseInts(t, tc.Label)[0]), a2SeriesOf(t, outs, "mfi"))
	case "CMF": // JS cmfValues → CMF
		a2CompareSeries(t, "cmf", CMF(h, l, c, v, a2ParseInts(t, tc.Label)[0]), a2SeriesOf(t, outs, "cmf"))
	case "ADLine": // JS adValues → ADLine
		a2CompareSeries(t, "ad", ADLine(h, l, c, v), a2SeriesOf(t, outs, "ad"))
	case "ForceIndex": // JS forceIndexValues → ForceIndex
		a2CompareSeries(t, "fi", ForceIndex(c, v, a2ParseInts(t, tc.Label)[0]), a2SeriesOf(t, outs, "fi"))
	case "ChaikinOsc": // JS chaikinOscValues → ChaikinOsc
		ip := a2ParseInts(t, tc.Label)
		a2CompareSeries(t, "co", ChaikinOsc(h, l, c, v, ip[0], ip[1]), a2SeriesOf(t, outs, "co"))
	default:
		t.Fatalf("未知黄金值 case: fn=%q", tc.Fn)
	}
}

func TestGoldenA2(t *testing.T) {
	gf := a2Load(t)
	if len(gf.Data.Close) != gf.Bars {
		t.Fatalf("数据长度与 bars 不一致: %d != %d", len(gf.Data.Close), gf.Bars)
	}
	h, l, c := gf.Data.High, gf.Data.Low, gf.Data.Close
	o, v := gf.Data.Open, gf.Data.Volume
	for _, tc := range gf.Cases {
		tc := tc
		t.Run(tc.Fn+"("+tc.Label+")", func(t *testing.T) {
			a2RunCase(t, h, l, c, o, v, tc)
		})
	}
}
