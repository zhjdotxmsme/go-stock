package indicator

// 黄金值对照测试：读 testdata/golden_a1.json（由 testdata/gen_a1.mjs 以
// frontend/src/components/kline/calc.ts 原函数生成），逐位对比本包移植实现：
//   - NaN 位置必须一致（JS 的 null ↔ Go 的 NaN）；
//   - 非 NaN 值满足 |got-want| <= 1e-8（绝对）或 <= 1e-6*max(|got|,|want|)（相对）。
// 输入序列（合成 OHLC）同存于 JSON，测试从 JSON 读取，自包含可重复。

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// series 支持 JSON null → NaN 的反序列化。
type series []float64

func (s *series) UnmarshalJSON(b []byte) error {
	var raw []*float64
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(series, len(raw))
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

// goldenBundle 承载 JS 多输出函数（对象返回值）的各条序列。
type goldenBundle struct {
	ADX         series `json:"adx"`
	DiP         series `json:"diP"`
	DiM         series `json:"diM"`
	Up          series `json:"up"`
	Down        series `json:"down"`
	SMI         series `json:"smi"`
	Signal      series `json:"signal"`
	FractalHigh series `json:"fractalHigh"`
	FractalLow  series `json:"fractalLow"`
}

type goldenCase struct {
	Fn     string          `json:"fn"`
	Params []float64       `json:"params"`
	Out    json.RawMessage `json:"out"`
}

type goldenFile struct {
	Input struct {
		High  series `json:"high"`
		Low   series `json:"low"`
		Close series `json:"close"`
	} `json:"input"`
	Cases []goldenCase `json:"cases"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden_a1.json"))
	if err != nil {
		t.Fatalf("读取黄金值文件失败: %v", err)
	}
	var gf goldenFile
	if err := json.Unmarshal(b, &gf); err != nil {
		t.Fatalf("解析黄金值文件失败: %v", err)
	}
	if len(gf.Cases) == 0 || len(gf.Input.Close) == 0 {
		t.Fatalf("黄金值文件为空: cases=%d bars=%d", len(gf.Cases), len(gf.Input.Close))
	}
	return gf
}

func (c goldenCase) param(i int) int {
	if i >= len(c.Params) {
		return 0
	}
	return int(c.Params[i])
}

func (c goldenCase) label() string {
	return fmt.Sprintf("%s(%v)", c.Fn, c.Params)
}

// compareSeries 逐位对比：NaN 位置一致；非 NaN 值满足 1e-8 绝对或 1e-6 相对容差。
func compareSeries(t *testing.T, name string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: 序列长度不一致 got=%d want=%d", name, len(got), len(want))
	}
	maxDiff := 0.0
	for i := range want {
		switch {
		case math.IsNaN(want[i]) && !math.IsNaN(got[i]):
			t.Fatalf("%s[%d]: 期望 NaN（JS null），got=%v", name, i, got[i])
		case !math.IsNaN(want[i]) && math.IsNaN(got[i]):
			t.Fatalf("%s[%d]: 意外 NaN，want=%v", name, i, want[i])
		case math.IsNaN(want[i]):
			continue
		}
		diff := math.Abs(got[i] - want[i])
		if diff > maxDiff {
			maxDiff = diff
		}
		if diff > 1e-8 && diff > 1e-6*math.Max(math.Abs(got[i]), math.Abs(want[i])) {
			t.Fatalf("%s[%d]: got=%.12g want=%.12g diff=%.3g 超出容差", name, i, got[i], want[i], diff)
		}
	}
	t.Logf("%s: pass (max|diff|=%.3g)", name, maxDiff)
}

// runGoldenCase 按 fn 名分派到对应 Go 函数并与黄金值逐位对比。
func runGoldenCase(t *testing.T, gf goldenFile, c goldenCase) {
	t.Helper()
	h, l, cl := gf.Input.High, gf.Input.Low, gf.Input.Close
	p := c.param

	decodeBundle := func() goldenBundle {
		var b goldenBundle
		if err := json.Unmarshal(c.Out, &b); err != nil {
			t.Fatalf("%s: 解析黄金输出失败: %v", c.label(), err)
		}
		return b
	}
	decodeSeries := func() series {
		var s series
		if err := json.Unmarshal(c.Out, &s); err != nil {
			t.Fatalf("%s: 解析黄金输出失败: %v", c.label(), err)
		}
		return s
	}

	switch c.Fn {
	case "sma": // JS smaValues → 私有 smaValues（AO 依赖，包内验证）
		compareSeries(t, c.label(), smaValues(cl, p(0)), decodeSeries())
	case "ema": // JS emaFinite → EMA
		compareSeries(t, c.label(), EMA(cl, p(0)), decodeSeries())
	case "wma": // JS weightedMaValues → 私有 weightedMaValues（HMA 依赖，包内验证）
		compareSeries(t, c.label(), weightedMaValues(cl, p(0)), decodeSeries())
	case "hma": // JS hullMaValues → HMA
		compareSeries(t, c.label(), HMA(cl, p(0)), decodeSeries())
	case "kama": // JS kamaValues → KAMA
		compareSeries(t, c.label(), KAMA(cl, p(0), p(1), p(2)), decodeSeries())
	case "adx": // JS adxValues → ADX
		adx, diP, diM := ADX(h, l, cl, p(0))
		b := decodeBundle()
		compareSeries(t, c.label()+" adx", adx, b.ADX)
		compareSeries(t, c.label()+" diP", diP, b.DiP)
		compareSeries(t, c.label()+" diM", diM, b.DiM)
	case "aroon": // JS aroonValues → Aroon
		up, down := Aroon(h, l, p(0))
		b := decodeBundle()
		compareSeries(t, c.label()+" up", up, b.Up)
		compareSeries(t, c.label()+" down", down, b.Down)
	case "cmo": // JS cmoValues → CMO
		compareSeries(t, c.label(), CMO(cl, p(0)), decodeSeries())
	case "roc": // JS rocValues → ROC
		compareSeries(t, c.label(), ROC(cl, p(0)), decodeSeries())
	case "trix": // JS trixValues → TRIX
		compareSeries(t, c.label(), TRIX(cl, p(0)), decodeSeries())
	case "ao": // JS aoValues → AO
		compareSeries(t, c.label(), AO(h, l, p(0), p(1)), decodeSeries())
	case "smi": // JS smiValues → smiBundle（含导出的 SMI 主线）
		smi, signal := smiBundle(h, l, cl, p(0), p(1), p(2))
		compareSeries(t, c.label()+" SMI()=smi主线", SMI(h, l, cl, p(0), p(1), p(2)), smi)
		b := decodeBundle()
		compareSeries(t, c.label()+" smi", smi, b.SMI)
		compareSeries(t, c.label()+" signal", signal, b.Signal)
	case "fractal": // JS fractalValues → FractalLevels
		upLevels, downLevels := FractalLevels(h, l, p(0))
		b := decodeBundle()
		compareSeries(t, c.label()+" fractalHigh", upLevels, b.FractalHigh)
		compareSeries(t, c.label()+" fractalLow", downLevels, b.FractalLow)
	default:
		t.Fatalf("未知黄金值 case: fn=%q", c.Fn)
	}
}

// hasValid 断言序列至少含一个有效值，防止黄金值全 NaN 的空对照。
func hasValid(t *testing.T, name string, s []float64) {
	t.Helper()
	for _, v := range s {
		if !math.IsNaN(v) {
			return
		}
	}
	t.Fatalf("黄金值自检失败: %s 全为 NaN", name)
}

func TestGoldenMA(t *testing.T) {
	gf := loadGolden(t)
	ran := 0
	for _, c := range gf.Cases {
		switch c.Fn {
		case "sma", "ema", "wma", "hma", "kama":
			t.Run(c.label(), func(t *testing.T) { runGoldenCase(t, gf, c) })
			ran++
		}
	}
	if ran == 0 {
		t.Fatal("黄金值文件中没有均线族 case")
	}
	hasValid(t, "EMA(12)", EMA(gf.Input.Close, 12))
}

func TestGoldenMomentum(t *testing.T) {
	gf := loadGolden(t)
	ran := 0
	for _, c := range gf.Cases {
		switch c.Fn {
		case "adx", "aroon", "cmo", "roc", "trix", "ao", "smi":
			t.Run(c.label(), func(t *testing.T) { runGoldenCase(t, gf, c) })
			ran++
		}
	}
	if ran == 0 {
		t.Fatal("黄金值文件中没有动量族 case")
	}
	adx, _, _ := ADX(gf.Input.High, gf.Input.Low, gf.Input.Close, 14)
	hasValid(t, "ADX(14)", adx)
}

func TestGoldenFractal(t *testing.T) {
	gf := loadGolden(t)
	ran := 0
	for _, c := range gf.Cases {
		if c.Fn != "fractal" {
			continue
		}
		t.Run(c.label(), func(t *testing.T) { runGoldenCase(t, gf, c) })
		ran++
	}
	if ran == 0 {
		t.Fatal("黄金值文件中没有 fractal case")
	}
	// 非退化自检：随机游走数据应产生真实分形
	upLevels, downLevels := FractalLevels(gf.Input.High, gf.Input.Low, 2)
	hasValid(t, "FractalLevels(2) upLevels", upLevels)
	hasValid(t, "FractalLevels(2) downLevels", downLevels)
}
