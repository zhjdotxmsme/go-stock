// gen_a2.mjs —— 生成黄金值对照数据 golden_a2.json（通道/系统/量能三族函数）。
//
// 基准 = 前端实现 frontend/src/components/kline/calc.ts（本脚本直接 import 之；
// calc.ts 无任何本地 import，node --experimental-strip-types 可直接加载，
// 无需“复制即基准”的兜底）。
//
// 运行（仓库根目录）：
//   & "C:\Users\54389\AppData\Roaming\nvm\v24.18.0\node.exe" --experimental-strip-types backend/data/indicator/testdata/gen_a2.mjs
//
// 输出 backend/data/indicator/testdata/golden_a2.json：
//   - data：确定性合成 OHLCV（固定种子 LCG 随机游走，150 根，收盘价钳制在 5~200）；
//   - cases：逐函数逐参数组的完整输出序列（null = JS null/NaN 预热位，
//     布尔按 JS 实际；SATS 的 tqi 预热期为 0，与 JS outTqi 填 0 一致）。
//   - SMCEvents：由 smcValues 的 bosLines+chochLines 合并（JS 事件字段映射：
//     time→index、type 'bos'/'choch'→"BOS"/"CHoCH"、bull→bullish、fromPrice→level），
//     同一根 K 线先高点事件后低点事件（与 JS 扫描顺序一致）。
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  adValues,
  alligatorValues,
  chaikinOscValues,
  cmfValues,
  donchianChannelValues,
  elderRayValues,
  forceIndexValues,
  ichimokuValues,
  keltnerChannelValues,
  mfiValues,
  obvValues,
  sarValues,
  satsValues,
  smcValues,
  supertrendValues,
  ttmSqueezeValues,
} from '../../../../frontend/src/components/kline/calc.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));

// —— 确定性合成数据：简易 LCG（Numerical Recipes 参数）随机游走 ——
function makeData() {
  let seed = 20250917 >>> 0;
  const rnd = () => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed / 4294967296;
  };
  const N = 150;
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  let prevClose = 100;
  for (let i = 0; i < N; i++) {
    const o = prevClose;
    let c = o + (rnd() - 0.5) * 6;
    c = Math.min(200, Math.max(5, c));
    const h = Math.max(o, c) + rnd() * 2.5;
    const l = Math.max(0.5, Math.min(o, c) - rnd() * 2.5);
    const v = 500 + Math.round(rnd() * 12000);
    open.push(o);
    high.push(h);
    low.push(l);
    close.push(c);
    volume.push(v);
    prevClose = c;
  }
  return { open, high, low, close, volume };
}

const { open, high, low, close, volume } = makeData();

const cases = [];
function add(fn, label, params, outputs) {
  cases.push({ fn, label, params, outputs });
}
const dirBull = (direction) => direction.map((d) => d === 1);

// —— 通道类 ——
for (const [ema, atr, mult] of [[20, 10, 1.5], [12, 6, 2.0]]) {
  const r = keltnerChannelValues(high, low, close, ema, atr, mult);
  add('Keltner', `${ema},${atr},${mult}`, { emaPeriod: ema, atrPeriod: atr, mult },
    { mid: r.mid, upper: r.upper, lower: r.lower });
}
for (const p of [20, 10]) {
  const r = donchianChannelValues(high, low, p);
  add('Donchian', `${p}`, { period: p }, { upper: r.upper, mid: r.mid, lower: r.lower });
}
for (const [atrP, mult] of [[10, 3], [14, 2]]) {
  const r = supertrendValues(high, low, close, atrP, mult);
  add('SuperTrend', `${atrP},${mult}`, { atrPeriod: atrP, mult },
    { line: r.supertrend, bull: dirBull(r.direction) });
}
// Go 签名顺序 bollPeriod,keltnerPeriod,keltnerAtrPeriod,bollMult,keltnerMult；
// JS 参数顺序为 bollPeriod,bollMult,keltnerPeriod,keltnerAtrPeriod,keltnerMult。
for (const [bp, bm, kp, ka, km] of [[20, 2, 20, 10, 1.5], [14, 2, 14, 7, 1.5]]) {
  const r = ttmSqueezeValues(high, low, close, bp, bm, kp, ka, km);
  add('TTMSqueeze', `${bp},${kp},${ka},${bm},${km}`,
    { bollPeriod: bp, keltnerPeriod: kp, keltnerAtrPeriod: ka, bollMult: bm, keltnerMult: km },
    { squeezeOn: r.squeeze, momentum: r.momentum });
}

// —— 系统类 ——
for (const [t, k, s] of [[9, 26, 52], [7, 22, 44]]) {
  const r = ichimokuValues(high, low, close, t, k, s);
  // spanB 即 JS senkouB；spanA/senkouB 本就按当前下标存放，无位移；chikou 不导出。
  add('Ichimoku', `${t},${k},${s}`, { tenkanP: t, kijunP: k, senkouBP: s },
    { tenkan: r.tenkan, kijun: r.kijun, spanA: r.spanA, spanB: r.senkouB });
}
for (const [jl, tl, ll, jo, to, lo] of [[13, 8, 5, 8, 5, 3], [10, 5, 4, 6, 4, 2]]) {
  const r = alligatorValues(high, low, close, jl, tl, ll, jo, to, lo);
  add('Alligator', `${jl},${tl},${ll},${jo},${to},${lo}`,
    { jawLen: jl, teethLen: tl, lipsLen: ll, jawOffset: jo, teethOffset: to, lipsOffset: lo },
    { jaw: r.jaw, teeth: r.teeth, lips: r.lips });
}
for (const [st, mx] of [[0.02, 0.2], [0.04, 0.4]]) {
  const r = sarValues(high, low, close, st, mx);
  add('PSAR', `${st},${mx}`, { step: st, maxStep: mx },
    { sar: r.sar, bull: dirBull(r.direction) });
}
// SATS 三组：defaults=calc.ts 全默认；short=短窗口（更多有效样本）；
// legacy=关闭 TQI/自适应/非对称/平滑/性格翻转/有效 ATR 的回退分支。
const satsVariants = [
  { label: 'defaults', opts: {} },
  { label: 'short', opts: { atrBaselineLen: 40, erLen: 10, tqiStructLen: 14, tqiMomLen: 8, volLen: 12 } },
  { label: 'legacy', opts: { useTqi: false, useAdaptive: false, useAsymBands: false, smoothMult: false, useCharFlip: false, useEffAtr: false } },
];
for (const v of satsVariants) {
  const r = satsValues(high, low, close, volume, v.opts);
  add('SATS', v.label, v.opts,
    { line: r.stLine, upper: r.upper, lower: r.lower, bull: dirBull(r.direction), tqi: r.tqi });
}
function extractSMCEvents(internalLen, swingLen) {
  const r = smcValues(high, low, close, open, internalLen, swingLen);
  const all = [...r.bosLines, ...r.chochLines];
  // 恢复扫描顺序：按 time 升序；同一根 K 线高点事件（bull）在前、低点事件在后，
  // 与 JS 主循环先处理 intHighs 后处理 intLows 的顺序一致（Array.prototype.sort 稳定）。
  all.sort((a, b) => a.time - b.time || (a.bull === b.bull ? 0 : a.bull ? -1 : 1));
  return all.map((e) => ({
    index: e.time,
    type: e.type === 'bos' ? 'BOS' : 'CHoCH',
    bullish: e.bull,
    level: e.fromPrice,
  }));
}
for (const [il, sl] of [[5, 50], [3, 20]]) {
  add('SMCEvents', `${il},${sl}`, { internalLen: il, swingLen: sl },
    { events: extractSMCEvents(il, sl) });
}
for (const p of [13, 21]) {
  const r = elderRayValues(high, low, close, p);
  add('ElderRay', `${p}`, { emaPeriod: p }, { bullPower: r.bullPower, bearPower: r.bearPower });
}

// —— 量能类 ——
add('OBV', '-', {}, { obv: obvValues(close, volume) });
for (const p of [14, 20]) {
  add('MFI', `${p}`, { period: p }, { mfi: mfiValues(high, low, close, volume, p) });
}
for (const p of [20, 10]) {
  add('CMF', `${p}`, { period: p }, { cmf: cmfValues(high, low, close, volume, p) });
}
add('ADLine', '-', {}, { ad: adValues(high, low, close, volume) });
for (const p of [13, 5]) {
  add('ForceIndex', `${p}`, { period: p }, { fi: forceIndexValues(close, volume, p) });
}
for (const [f, s] of [[3, 10], [5, 14]]) {
  add('ChaikinOsc', `${f},${s}`, { fast: f, slow: s },
    { co: chaikinOscValues(high, low, close, volume, f, s) });
}

const golden = {
  generator: 'gen_a2.mjs',
  source: 'frontend/src/components/kline/calc.ts',
  note: 'null 序列化 JS 的 null/NaN 预热位；布尔按 JS 实际；SATS 的 tqi 预热期为 0（JS outTqi 填 0）；SMCEvents.level=JS fromPrice。',
  bars: close.length,
  data: { open, high, low, close, volume },
  cases,
};

writeFileSync(join(__dirname, 'golden_a2.json'), `${JSON.stringify(golden, null, 1)}\n`);
console.log(`golden_a2.json written: ${cases.length} cases, ${close.length} bars`);
for (const c of cases) {
  if (c.fn === 'SMCEvents') {
    console.log(`  ${c.fn}(${c.label}): ${c.outputs.events.length} events`);
  }
}
