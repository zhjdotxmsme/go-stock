// 黄金值生成脚本：以 frontend/src/components/kline/calc.ts 为唯一基准，
// 对确定性合成 OHLCV 逐个调用指标函数，输入+完整输出写入 golden_a1.json
// （null 位置即 NaN）。运行：
//   node --experimental-strip-types gen_a1.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

import {
  smaValues,
  emaFinite,
  weightedMaValues,
  hullMaValues,
  kamaValues,
  adxValues,
  aroonValues,
  cmoValues,
  rocValues,
  trixValues,
  aoValues,
  smiValues,
  fractalValues,
} from "../../../../frontend/src/components/kline/calc.ts";

// ---- 确定性合成 OHLCV：固定种子 LCG 随机游走 + 均值回复，价格夹在 [5, 200] ----
function lcg(seed) {
  let s = seed >>> 0;
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    return s / 4294967296;
  };
}

function genOHLCV(seed, n) {
  const rnd = lcg(seed);
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  let prev = 100;
  for (let i = 0; i < n; i++) {
    const drift = (100 - prev) * 0.02; // 轻度均值回复，制造趋势与震荡交替
    const step = (rnd() - 0.5) * 8 + drift;
    const c = Math.min(200, Math.max(5, prev + step));
    const h = Math.min(200, c + rnd() * 3);
    const l = Math.max(5, c - rnd() * 3);
    open.push(prev);
    high.push(h);
    low.push(l);
    close.push(c);
    volume.push(1000 + rnd() * 9000);
    prev = c;
  }
  return { open, high, low, close, volume };
}

// NaN/null 序列化为 null
const ser = (arr) => arr.map((v) => (v == null || !Number.isFinite(v) ? null : v));

const { high, low, close } = genOHLCV(20240601, 120);

// 每个 JS 函数至少 2 组合理参数；输出结构与 JS 返回值一一对应
const cases = [];
const push = (fn, params, out) => cases.push({ fn, params, out });

push("sma", [12], ser(smaValues(close, 12)));
push("sma", [26], ser(smaValues(close, 26)));

push("ema", [12], ser(emaFinite(close, 12)));
push("ema", [26], ser(emaFinite(close, 26)));

push("wma", [9], ser(weightedMaValues(close, 9)));
push("wma", [20], ser(weightedMaValues(close, 20)));

push("hma", [9], ser(hullMaValues(close, 9)));
push("hma", [16], ser(hullMaValues(close, 16)));

push("kama", [10, 2, 30], ser(kamaValues(close, 10, 2, 30)));
push("kama", [5, 2, 10], ser(kamaValues(close, 5, 2, 10)));

{
  const r1 = adxValues(high, low, close, 14);
  push("adx", [14], { adx: ser(r1.adx), diP: ser(r1.diP), diM: ser(r1.diM) });
  const r2 = adxValues(high, low, close, 20);
  push("adx", [20], { adx: ser(r2.adx), diP: ser(r2.diP), diM: ser(r2.diM) });
}

{
  const r1 = aroonValues(high, low, 25);
  push("aroon", [25], { up: ser(r1.up), down: ser(r1.down) });
  const r2 = aroonValues(high, low, 14);
  push("aroon", [14], { up: ser(r2.up), down: ser(r2.down) });
}

push("cmo", [14], ser(cmoValues(close, 14)));
push("cmo", [21], ser(cmoValues(close, 21)));

push("roc", [12], ser(rocValues(close, 12)));
push("roc", [6], ser(rocValues(close, 6)));

push("trix", [15], ser(trixValues(close, 15)));
push("trix", [9], ser(trixValues(close, 9)));

push("ao", [5, 34], ser(aoValues(high, low, 5, 34)));
push("ao", [3, 10], ser(aoValues(high, low, 3, 10)));

{
  const r1 = smiValues(high, low, close, 14, 3, 3);
  push("smi", [14, 3, 3], { smi: ser(r1.smi), signal: ser(r1.signal) });
  const r2 = smiValues(high, low, close, 10, 5, 3);
  push("smi", [10, 5, 3], { smi: ser(r2.smi), signal: ser(r2.signal) });
}

{
  const r1 = fractalValues(high, low, 2);
  push("fractal", [2], { fractalHigh: ser(r1.fractalHigh), fractalLow: ser(r1.fractalLow) });
  const r2 = fractalValues(high, low, 3);
  push("fractal", [3], { fractalHigh: ser(r2.fractalHigh), fractalLow: ser(r2.fractalLow) });
}

const golden = {
  meta: {
    source: "frontend/src/components/kline/calc.ts",
    generator: "gen_a1.mjs",
    seed: 20240601,
    bars: close.length,
    note: "null 位置即 NaN；out 数组或对象字段与 JS 返回值一一对应",
  },
  input: { high: ser(high), low: ser(low), close: ser(close) },
  cases,
};

const outPath = path.join(path.dirname(fileURLToPath(import.meta.url)), "golden_a1.json");
writeFileSync(outPath, JSON.stringify(golden));
console.log(`written: ${outPath}`);
console.log(`bars=${close.length} cases=${cases.length}`);
// 自检：确认关键序列非退化（避免黄金值全 NaN 的无效对照）
const hasVal = (a) => a.some((v) => v != null);
const findCase = (fn, params) =>
  cases.find((c) => c.fn === fn && JSON.stringify(c.params) === JSON.stringify(params));
const nonVacuous = [
  ["ema/12", findCase("ema", [12]).out],
  ["adx/14", findCase("adx", [14]).out.adx],
  ["smi/14", findCase("smi", [14, 3, 3]).out.smi],
  ["fractal/2", [
    ...findCase("fractal", [2]).out.fractalHigh,
    ...findCase("fractal", [2]).out.fractalLow,
  ]],
];
for (const [name, arr] of nonVacuous) {
  if (!hasVal(arr)) {
    console.error(`golden self-check failed: ${name} 全为 null`);
    process.exit(1);
  }
}
console.log("self-check ok: 关键黄金序列均含有效值");
