/**
 * 关注代码归一化回归测试（无第三方依赖，node 直接跑）
 *
 *   node scripts/check-stock-code.mjs
 *
 * 背景：后端 StockDataApi.Follow 只接受 shXXXXXX/szXXXXXX/bjXXXXXX/hkXXXXX/usXXXX，
 * 实测 Follow("600519")、Follow("格力") 都返回「关注失败」。而 SelectStock.vue 的
 * AI 选股结果行没有 MARKET_SHORT_NAME，旧代码 row.MARKET_SHORT_NAME.toLowerCase()
 * 直接抛 TypeError（Promise 链之外），点击「关注」完全没有反应。
 * 这里锁定这两个入口的归一化行为。
 */
import { resolveFollowCode, resolveRowFollowCode } from '../src/utils/stockCode.js'

const followCases = [
  // 输入, 期望
  ['600519', 'sh600519'],
  ['600519.SH', 'sh600519'],
  ['sh600519', 'sh600519'],
  ['SH600519', 'sh600519'],
  ['601398.sh', 'sh601398'],
  ['000651.SZ', 'sz000651'],
  ['000651', 'sz000651'],
  ['300750', 'sz300750'],
  ['688981', 'sh688981'],
  ['430047', 'bj430047'],
  ['830799', 'bj830799'],
  ['00700.HK', 'hk00700'],
  ['格力电器 - 000651.SZ', 'sz000651'],
  ['AAPL.US', 'usAAPL'],
  ['gb_AAPL', 'usAAPL'],
  // 以下是必须被拒绝的输入（原样发送会被后端判为「关注失败」）
  ['格力', ''],
  ['随便写点什么', ''],
  ['600519.SH格力', ''],
  ['600519', 'sh600519'],
  ['', ''],
  [null, ''],
  [undefined, ''],
]

const rowCases = [
  [{ SECURITY_CODE: '600519', MARKET_SHORT_NAME: 'SH' }, 'sh600519'],
  [{ SECURITY_CODE: '000651', MARKET_SHORT_NAME: 'sz' }, 'sz000651'],
  [{ SECURITY_CODE: '00700', MARKET_SHORT_NAME: 'HK' }, 'hk00700'],
  [{ SECURITY_CODE: 'AAPL', MARKET_SHORT_NAME: 'US' }, 'usAAPL'],
  [{ SECURITY_CODE: '600519' }, 'sh600519'],
  [{ SECURITY_CODE: '000651.SZ' }, 'sz000651'],
  [{ SECURITY_CODE: '600519', MARKET_SHORT_NAME: 'SSE' }, 'sh600519'],
  // AI 配置选股结果行：无 MARKET_SHORT_NAME（旧代码在这里崩）
  [{ SECURITY_CODE: '300750', SECURITY_SHORT_NAME: '宁德时代' }, 'sz300750'],
  [{ SECURITY_CODE: '' }, ''],
  [null, ''],
]

let failed = 0

function check(label, got, want) {
  const ok = got === want
  if (!ok) failed++
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${label} = ${JSON.stringify(got)}${ok ? '' : ` (want ${JSON.stringify(want)})`}`)
}

for (const [input, want] of followCases) {
  check(`resolveFollowCode(${JSON.stringify(input)})`, resolveFollowCode(input), want)
}
for (const [row, want] of rowCases) {
  check(`resolveRowFollowCode(${JSON.stringify(row)})`, resolveRowFollowCode(row), want)
}

console.log(failed === 0 ? `\nALL PASS (${followCases.length + rowCases.length} cases)` : `\n${failed} FAILURES`)
process.exit(failed === 0 ? 0 : 1)
