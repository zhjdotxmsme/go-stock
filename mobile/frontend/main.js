// go-stock mobile frontend — 4 feature groups:
//   行情(自选+K线/分时+全球指数+商品+快讯)
//   AI  (Agent 对话 + 个股流式分析)
//   选股(每日选股 + 复盘 + 回测)
//   持仓(持仓概览 + 交易记录 + 基金 + 异动)
import * as Runtime from '@wailsio/runtime';
import * as echarts from 'echarts';

import {
  StockHandler, MarketHandler, CommodityHandler, NewsHandler,
  AgentHandler, DailyPickHandler, TradingRecordHandler, FundHandler, StockChangeHandler,
} from './bindings/go-stock/backend/handler';
import { Service as BacktestService } from './bindings/go-stock/backend/data/backtest';
import { DailyPickBacktestService } from './bindings/go-stock/backend/service';
import { MobileService } from './bindings/go-stock/mobile';

const { Events } = Runtime;
const $ = (id) => document.getElementById(id);

// ================================ 工具 ================================
const fmtNum = (n) => { if (n === undefined || n === null || isNaN(+n)) return '--'; return (+n).toFixed(2); };
const fmtPct = (n) => { if (n === undefined || n === null || isNaN(+n)) return '--'; const v = +n; return (v >= 0 ? '+' : '') + v.toFixed(2) + '%'; };
const cls = (n) => (n === undefined || n === null || isNaN(+n)) ? 'neutral' : (+n >= 0 ? 'up' : 'down');
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const today = () => new Date().toISOString().slice(0, 10);
const pick = (obj, keys, def) => { for (const k of keys) { if (obj && obj[k] !== undefined) return obj[k]; } return def; };

// 事件订阅(去重)：v3 的 Events.On 每次调用都会新建一个订阅，避免重复
const _subs = [];
function on(event, handler) {
  for (const s of _subs) if (s.event === event) return;
  const off = Events.On(event, (e) => {
    try { handler(e && e.data !== undefined ? e.data : e); } catch (_) {}
  });
  _subs.push({ event, off });
}

// ================================ Tab 切换 ================================
const TITLES = { market: '行情', ai: 'AI', pick: '选股', pos: '持仓' };
function switchTab(name) {
  const btn = document.querySelector(`nav.tabbar button[data-page="${name}"]`);
  if (btn) btn.click();
}
document.querySelectorAll('nav.tabbar button').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('nav.tabbar button').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('main .page').forEach((p) => p.classList.remove('active'));
    btn.classList.add('active');
    $('page-' + btn.dataset.page).classList.add('active');
    $('page-title').textContent = TITLES[btn.dataset.page];
    // 进入新 tab 时按需刷新(避免行情/持仓过期)
    if (btn.dataset.page === 'market') reloadMarket();
    if (btn.dataset.page === 'pick') { loadLatestPicks(); loadDailyPickStats(); }
    if (btn.dataset.page === 'pos') reloadPosition();
  });
});

// ================================ 页1：行情 ================================
let klineChart = null;
let cmdChart = null;
let curKlt = '101';
document.querySelectorAll('#kline-period .chip').forEach((c) => c.addEventListener('click', () => {
  document.querySelectorAll('#kline-period .chip').forEach((x) => x.classList.remove('active'));
  c.classList.add('active');
  curKlt = c.dataset.klt;
  loadKline($('stock-code').value.trim());
}));

async function loadMarketStat() {
  try {
    const s = await MarketHandler.GetTodayMarketStatistic();
    if (s && typeof s === 'object') {
      $('mkt-up').textContent = s.UpCount ?? s.upCount ?? '0';
      $('mkt-down').textContent = s.DownCount ?? s.downCount ?? '0';
      $('mkt-limitup').textContent = s.LimitUp ?? s.limitUp ?? '0';
      $('mkt-limitdown').textContent = s.LimitDown ?? s.limitDown ?? '0';
    }
  } catch (_) { /* 静默——不影响其它卡片 */ }
}

async function loadGlobalIdx() {
  const box = $('global-idx');
  box.className = 'empty'; box.innerHTML = '加载中...';
  try {
    let arr = null;
    try { arr = await MarketHandler.GlobalStockIndexes(); } catch (_) {}
    if (arr && !Array.isArray(arr) && typeof arr === 'object') arr = Object.values(arr);
    if (arr && arr.length) {
      box.className = '';
      box.innerHTML = arr.slice(0, 30).map((it) => {
        const name = pick(it, ['Name', 'name', 'code'], '--');
        const zxj = pick(it, ['Zxj', 'zxj'], '--');
        const zdf = pick(it, ['Zdf', 'zdf'], '');
        return `<div class="kv"><span class="k">${esc(name)}</span><span class="${cls(zdf)}">${esc(zxj)} ${fmtPct(zdf)}</span></div>`;
      }).join('');
      return;
    }
    // 结构化取不到 → 回退到 Readable 字符串
    const s = await MarketHandler.GlobalStockIndexesReadable();
    const txt = typeof s === 'string' && s ? s : '暂无全球指数（首次运行会后台爬取，稍后刷新可看）';
    box.className = '';
    box.innerHTML = '<pre class="code">' + esc(txt) + '</pre>';
  } catch (e) { box.className = 'empty'; box.innerHTML = '加载失败: ' + esc(e && e.message || e); }
}

async function loadKline(code) {
  if (!code) return;
  $('market-status').classList.remove('err');
  $('market-status').textContent = '加载K线中...';
  try {
    let kl;
    try {
      const r = await StockHandler.GetStockKLineWithFallback(code, '', curKlt, 200);
      kl = r && (r.Data ?? r.data) ? (r.Data ?? r.data) : r;
    } catch (_) {
      kl = await StockHandler.GetStockKLine(code, '', 120);
    }
    if (!kl || !kl.length) { $('market-status').textContent = '未获取到K线数据'; return; }
    const dates = kl.map((k) => pick(k, ['Date', 'date', 'Timestamp'], ''));
    const kdata = kl.map((k) => [
      +(pick(k, ['Open', 'open'], 0)),
      +(pick(k, ['Close', 'close'], 0)),
      +(pick(k, ['Low', 'low'], 0)),
      +(pick(k, ['High', 'high'], 0)),
    ]);
    const vols = kl.map((k) => +(pick(k, ['Volume', 'volume'], 0)));
    const last = kl[kl.length - 1];
    const lastPx = +(pick(last, ['Close', 'close'], 0));
    const pxField = pick(last, ['ChangePercent', 'changePercent', 'Percent', 'percent'], 0);
    $('quote-name').textContent = code;
    $('quote-change').textContent = fmtNum(lastPx) + '  ' + fmtPct(pxField);
    $('quote-change').className = 'quote-change ' + cls(pxField);
    if (!klineChart) klineChart = echarts.init($('kline-chart'));
    klineChart.setOption({
      animation: false,
      grid: [
        { left: 55, right: 12, top: 10, height: '58%' },
        { left: 55, right: 12, top: '74%', height: '18%' },
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#888', fontSize: 10 } },
        { type: 'category', gridIndex: 1, data: dates, axisLabel: { show: false }, axisLine: { show: false } },
      ],
      yAxis: [
        { scale: true, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#888', fontSize: 10 }, splitLine: { lineStyle: { color: '#2b374d' } } },
        { gridIndex: 1, axisLabel: { show: false }, splitLine: { show: false } },
      ],
      dataZoom: [{ type: 'inside', xAxisIndex: [0, 1], start: 60, end: 100 }],
      tooltip: { trigger: 'axis', axisPointer: { type: 'cross' }, backgroundColor: '#1a2130', borderColor: '#2b374d', textStyle: { color: '#e6ecf5', fontSize: 11 } },
      series: [
        { type: 'candlestick', name: 'K线', data: kdata,
          itemStyle: { color: '#ff5a6a', color0: '#21b978', borderColor: '#ff5a6a', borderColor0: '#21b978' } },
        { type: 'bar', name: '成交量', xAxisIndex: 1, yAxisIndex: 1, data: vols, itemStyle: { color: '#4a90e2' } },
      ],
    }, true);
    $('market-status').textContent = '';
  } catch (e) {
    $('market-status').classList.add('err');
    $('market-status').textContent = 'K线加载失败: ' + (e && e.message || e);
  }
}
$('btn-load-kline').addEventListener('click', () => loadKline($('stock-code').value.trim()));

async function loadFollowList() {
  const box = $('follow-list');
  box.innerHTML = '<div class="empty">加载自选股...</div>';
  try {
    const list = (await StockHandler.GetFollowList(0)) || [];
    if (!list.length) { box.innerHTML = '<div class="empty">尚未关注（点下方"关注当前"把查询的代码加进来）</div>'; return; }
    box.innerHTML = '';
    for (const s of list) {
      const code = pick(s, ['StockCode', 'stockCode']);
      const name = pick(s, ['Name', 'name'], '');
      const price = pick(s, ['Price', 'price'], '');
      const pct = pick(s, ['ChangePercent', 'changePercent'], '');
      const item = document.createElement('div');
      item.className = 'stock-row';
      item.innerHTML = `
        <div><div class="s-name">${esc(name)}</div><div class="s-code">${esc(code)}</div></div>
        <div class="s-price"><div class="${cls(pct)}">${price ? esc(fmtNum(price)) : '--'}</div><div class="${cls(pct)}">${fmtPct(pct)}</div></div>`;
      item.addEventListener('click', () => { $('stock-code').value = code; loadKline(code); $('kline-chart').scrollIntoView({ behavior: 'smooth', block: 'center' }); });
      box.appendChild(item);
    }
  } catch (e) {
    box.innerHTML = '<div class="empty">自选加载失败: ' + esc(e && e.message || e) + '</div>';
  }
}
$('btn-refresh-fl').addEventListener('click', loadFollowList);
$('btn-add-follow').addEventListener('click', async () => {
  const c = $('stock-code').value.trim(); if (!c) return;
  try { await StockHandler.Follow(c); loadFollowList(); }
  catch (e) { $('market-status').classList.add('err'); $('market-status').textContent = '关注失败: ' + e; }
});
$('btn-unfollow').addEventListener('click', async () => {
  const c = $('stock-code').value.trim(); if (!c) return;
  try { await StockHandler.UnFollow(c); loadFollowList(); }
  catch (e) { $('market-status').classList.add('err'); $('market-status').textContent = '取关失败: ' + e; }
});

async function loadCommodity() {
  const code = $('cmd-code').value.trim();
  if (!code) return;
  const period = $('cmd-period').value;
  $('cmd-quote').textContent = '加载中...';
  try {
    let q = null;
    try { q = await CommodityHandler.GetCommodityQuoteIntl(code); } catch (_) { q = await CommodityHandler.GetCommodityQuote(code); }
    const name = pick(q, ['Name', 'name'], code);
    const price = pick(q, ['Price', 'price'], '');
    const pct = pick(q, ['ChangePercent', 'changePercent'], '');
    $('cmd-quote').innerHTML = `<b>${esc(name)}</b>  <span class="${cls(pct)}">${fmtNum(price)} ${fmtPct(pct)}</span>`;
    let kl = null;
    try { kl = await CommodityHandler.GetCommodityKLineIntl(code, period, 120); }
    catch (_) { kl = await CommodityHandler.GetCommodityKLine(code, period, 120); }
    if (kl && kl.length) {
      const dates = kl.map((k) => pick(k, ['Date', 'date'], ''));
      const closes = kl.map((k) => +(pick(k, ['Close', 'close'], 0)));
      if (!cmdChart) cmdChart = echarts.init($('commodity-chart'));
      cmdChart.setOption({
        animation: false,
        xAxis: { type: 'category', data: dates, axisLabel: { color: '#888', fontSize: 10 } },
        yAxis: { scale: true, axisLabel: { color: '#888', fontSize: 10 }, splitLine: { lineStyle: { color: '#2b374d' } } },
        dataZoom: [{ type: 'inside', start: 50, end: 100 }],
        series: [{ type: 'line', data: closes, smooth: true, lineStyle: { color: '#4a90e2', width: 1.5 },
          areaStyle: { color: 'rgba(74,144,226,0.08)' } }],
      }, true);
    }
  } catch (e) { $('cmd-quote').innerHTML = '商品查询失败: ' + esc(e && e.message || e); }
}
$('btn-cmd').addEventListener('click', loadCommodity);

async function loadNews(limit = 30) {
  const src = $('news-src').value;
  $('btn-news').disabled = true;
  try {
    let list = null;
    try { list = await NewsHandler.GetTelegraphList(src); } catch (_) {}
    if (!list || !list.length) {
      // 回退：用今日热题替代
      const hot = await MarketHandler.HotTopic(15);
      if (hot && (hot.length || (typeof hot === 'object' && Object.keys(hot).length))) {
        const arr2 = Array.isArray(hot) ? hot : Object.values(hot);
        $('news-list').innerHTML = arr2.slice(0, 15).map((h) => {
          const title = pick(h, ['Title', 'title', 'Question', 'question', 'name'], '--');
          const heat = pick(h, ['Heat', 'heat', 'heatValue', 'value'], '');
          return `<div style="padding:8px 0;border-bottom:1px dashed var(--line)">${esc(title)} ${heat ? '<span class="tag">' + esc(heat) + '</span>' : ''}</div>`;
        }).join('');
        return;
      }
      $('news-list').innerHTML = '<div class="empty">暂无快讯</div>';
      return;
    }
    const arr2 = Array.isArray(list) ? list : Object.values(list);
    $('news-list').innerHTML = arr2.slice(0, limit).map((n) => {
      // n 可能是 string 或 {content/title/time...}
      const c = typeof n === 'string' ? n : pick(n, ['Content', 'content', 'Title', 'title', 'summary'], '…');
      const t = pick(n, ['Time', 'time', 'Timestamp', 'timestamp'], '') ? ' <span class="tag">' + esc(pick(n, ['Time', 'time', 'Timestamp', 'timestamp'], '')) + '</span>' : '';
      return `<div style="padding:8px 0;border-bottom:1px dashed var(--line)">${esc(c)}${t}</div>`;
    }).join('');
  } catch (e) { $('news-list').innerHTML = '<div class="empty">加载失败: ' + esc(e) + '</div>'; }
  finally { $('btn-news').disabled = false; }
}
$('btn-news').addEventListener('click', () => loadNews());

// 实时推送：行情更新 / 自选刷新 / 快讯推送
on('stock_price', () => loadFollowList());
on('refreshFollowList', () => loadFollowList());
on('newTelegraph', () => { if ($('news-list').innerHTML !== '<div class="empty">暂无快讯</div>') loadNews(); });

function reloadMarket() {
  loadMarketStat(); loadGlobalIdx(); loadFollowList(); loadNews();
}

// ================================ 页2：AI ================================
document.querySelectorAll('#ai-mode .chip').forEach((c) => c.addEventListener('click', () => {
  document.querySelectorAll('#ai-mode .chip').forEach((x) => x.classList.remove('active'));
  c.classList.add('active');
}));
let aiBusy = false;
async function loadAiConfigs() {
  const sel = $('ai-config');
  try {
    const list = (await MobileService.ListAiConfigs()) || [];
    sel.innerHTML = '';
    if (!list.length) {
      const o = document.createElement('option'); o.value = '0';
      o.textContent = '未配置 AI（去桌面端"AI 配置"里加一条，再刷新）';
      sel.appendChild(o); return;
    }
    for (const c of list) {
      const o = document.createElement('option'); o.value = c.id;
      o.textContent = `${c.name}  (${c.modelName})`;
      sel.appendChild(o);
    }
  } catch (e) { sel.innerHTML = '<option value="0">AI 配置加载失败</option>'; }
}
$('btn-ai-refresh').addEventListener('click', loadAiConfigs);

function appendChat(text, isUser = false) {
  const div = document.createElement('div');
  div.className = 'chat-msg' + (isUser ? ' user' : '');
  div.textContent = text;
  $('chat-box').appendChild(div);
  $('chat-box').scrollTop = $('chat-box').scrollHeight;
  return div;
}

$('btn-ask').addEventListener('click', async () => {
  const q = $('ai-question').value.trim();
  if (!q || aiBusy) return;
  const cid = Number($('ai-config').value || 0);
  if (!cid) { $('ai-status').textContent = '请先在桌面端"AI 配置"里加一条 AI，再"刷新"。'; return; }
  aiBusy = true; $('btn-ask').disabled = true;
  $('ai-question').value = '';
  appendChat(q, true);
  const stream = appendChat('');
  let text = '';
  on('agent-message', (msg) => {
    if (msg === 'agent-DONE' || (msg && msg.content === 'agent-DONE')) {
      if (!text) stream.textContent = '(完成，无内容)';
      aiBusy = false; $('btn-ask').disabled = false;
      $('ai-status').textContent = '完成';
      return;
    }
    if (!msg || typeof msg !== 'object') return;
    if (msg.role === 'user') return;
    if (msg.content) { text += msg.content; stream.textContent = text; $('chat-box').scrollTop = $('chat-box').scrollHeight; }
    if (msg.reasoning_content) {
      const r = stream.querySelector('.reasoning') || (() => { const n = document.createElement('div'); n.className = 'reasoning'; stream.insertBefore(n, stream.firstChild); return n; })();
      r.textContent = (r.textContent || '') + msg.reasoning_content;
    }
  });
  try {
    const mode = (document.querySelector('#ai-mode .chip.active') || {}).dataset?.mode || 'quick';
    await AgentHandler.ChatWithAgent(q, cid, null, false, 0, mode === 'pro', mode, 'mobile-' + Date.now());
  } catch (e) {
    stream.textContent = '请求失败: ' + (e && e.message || e);
    aiBusy = false; $('btn-ask').disabled = false;
    $('ai-status').textContent = '失败';
  }
});
$('btn-abort').addEventListener('click', async () => { try { await AgentHandler.AbortChatWithAgent(); } catch (_) {} aiBusy = false; $('btn-ask').disabled = false; });

// 个股流式分析
$('btn-analyze').addEventListener('click', async () => {
  const code = $('an-stock').value.trim();
  if (!code) { $('an-status').textContent = '请填写股票代码'; return; }
  const cid = Number($('ai-config').value || 0);
  if (!cid) { $('an-status').textContent = '缺 AI 配置'; return; }
  const q = $('an-q').value.trim() || '分析这只股票';
  $('an-status').textContent = '流式分析中(几十秒)…';
  const stream = appendChat('');
  let text = '';
  const streamSid = 'an-' + Date.now();
  on('newChatStream', (msg) => {
    if (msg === 'DONE') { $('an-status').textContent = '完成'; return; }
    if (!msg || typeof msg !== 'object') return;
    if (msg.chatId && msg.chatId !== streamSid) return;
    if (msg.content) { text += msg.content; stream.textContent = text; $('chat-box').scrollTop = $('chat-box').scrollHeight; }
    if (msg.reasoning_content) text += msg.reasoning_content;
  });
  try {
    await AgentHandler.NewChatStream('', code, q, cid, null, false, false, 'quick', '');
  } catch (e) { $('an-status').textContent = '失败: ' + (e && e.message || e); }
});
$('btn-analyze-abort').addEventListener('click', async () => { try { await AgentHandler.AbortChatWithAgent(); } catch (_) {} $('an-status').textContent = '已中断'; });

// ================================ 页3：选股 ================================
if ($('pk-date-input')) { $('pk-date-input').value = today(); $('pk-date-input').max = today(); }

async function loadDailyPickStats() {
  try {
    const s = await DailyPickHandler.GetDailyPickStats();
    if (s && typeof s === 'object') {
      const t = pick(s, ['TotalCount', 'totalCount', 'count'], 0);
      const as = pick(s, ['AvgScore', 'avgScore'], '');
      const hit = pick(s, ['HitRate', 'hitRate', 'ReviewHitRate'], '');
      $('pk-count').textContent = t === '' ? '—' : t;
      $('pk-avg-score').textContent = as === '' ? '—' : fmtNum(as);
      const h = hit === '' ? '—' : ((+hit) >= 0 ? (+hit).toLocaleString('en', { style: 'percent' }) : '—');
      $('pk-hit').textContent = h;
    }
  } catch (_) {}
}

async function loadLatestPicks() {
  const topN = Number($('pk-topn').value || 10);
  const date = $('pk-date-input').value;
  let list;
  try {
    list = await DailyPickHandler.GetLatestPicks(topN);
  } catch (e1) {
    try {
      list = await DailyPickHandler.GetDailyPicks({ page: 1, pageSize: topN, tradeDate: date, startDate: '', endDate: '', Reviewed: null });
      // GetDailyPicks 可能返回 {data: []} 或 数组
      if (list && !Array.isArray(list) && Array.isArray(list.data)) list = list.data;
    } catch (e2) { list = []; }
  }
  const box = $('pk-list');
  if (!list || !list.length) { box.innerHTML = '<div class="empty">暂无选股结果（点下方"立即跑一次"或到"复盘"手动触发）</div>'; return; }
  box.innerHTML = '';
  for (const p of list) {
    const code = pick(p, ['stockCode', 'StockCode']);
    const name = pick(p, ['stockName', 'StockName']);
    const score = pick(p, ['score', 'Score']);
    const close = pick(p, ['closePrice', 'ClosePrice']);
    const chg = pick(p, ['changePercent', 'ChangePercent']);
    const strat = pick(p, ['strategyName', 'StrategyName'], '');
    const el = document.createElement('div');
    el.className = 'stock-row';
    el.innerHTML = `
      <div><div class="s-name">${esc(name)} <span class="tag">${esc(code)}</span> ${strat ? '<span class="tag">' + esc(strat) + '</span>' : ''}</div>
        <div class="s-code">评分 ${fmtNum(score)} · 收 ${fmtNum(close)} · ${fmtPct(chg)}</div></div>
      <div class="s-price"><div class="${cls(chg)}">${fmtNum(close)}</div></div>`;
    el.addEventListener('click', () => { switchTab('market'); $('stock-code').value = code; loadKline(code); });
    box.appendChild(el);
  }
}
$('btn-pk-refresh').addEventListener('click', () => { loadLatestPicks(); loadDailyPickStats(); });

$('btn-pk-run').addEventListener('click', async () => {
  const date = $('pk-date-input').value;
  const topN = Number($('pk-topn').value || 10);
  $('btn-pk-run').disabled = true;
  $('pk-status').textContent = '选股运行中…（30-90 秒）';
  try {
    const run = async (fn) => {
      try { await fn(); return true; } catch (e) { $('pk-status').textContent = '直接运行失败: ' + (e && e.message || e); return false; }
    };
    let started = false;
    if (!started) started = await run(() => DailyPickHandler.RunDailyPickAsync(date, topN));
    if (!started) await run(() => DailyPickHandler.RunDailyPick(date, topN));
    if (!started) return;
    // 轮询：每 4s 看一次 stats，最多 40s
    let tried = 0;
    const t = setInterval(async () => {
      tried++;
      try {
        const s = await DailyPickHandler.GetDailyPickStats();
        const t2 = pick(s, ['TotalCount', 'totalCount', 'count'], 0);
        if (+t2 > 0) {
          clearInterval(t);
          $('btn-pk-run').disabled = false;
          $('pk-status').textContent = '完成，已刷新';
          loadLatestPicks(); loadDailyPickStats();
          return;
        }
      } catch (_) {}
      if (tried >= 10) { clearInterval(t); $('btn-pk-run').disabled = false; $('pk-status').textContent = '仍在跑（后台继续），稍后刷新可看进度'; }
    }, 4000);
  } catch (e) {
    $('btn-pk-run').disabled = false;
    $('pk-status').textContent = '失败: ' + (e && e.message || e);
  }
});

$('btn-review-all').addEventListener('click', async () => {
  $('btn-review-all').disabled = true;
  $('pk-status').textContent = '复盘运行中（会调 LLM，可能需要 1-3 分钟）…';
  try {
    await DailyPickHandler.ReviewAllUnreviewed();
    $('pk-status').textContent = '复盘完成';
    loadLatestPicks(); loadDailyPickStats();
  } catch (e) { $('pk-status').textContent = '复盘失败: ' + (e && e.message || e); }
  finally { $('btn-review-all').disabled = false; }
});

// 回测
$('btn-bt-run').addEventListener('click', async () => {
  const date = $('pk-date-input').value;
  const topN = Number($('pk-topn').value || 10);
  $('btn-bt-run').disabled = true;
  $('bt-status').textContent = '回测运行中…';
  try {
    const r = await DailyPickBacktestService.RunBacktestForDailyPicks(date, topN, 5, -0.05, 0.10, true);
    $('bt-result-card').style.display = '';
    $('bt-result').textContent = JSON.stringify(r, null, 2);
    $('bt-status').textContent = '完成';
  } catch (e) { $('bt-status').textContent = '失败: ' + (e && e.message || e); }
  finally { $('btn-bt-run').disabled = false; }
});

$('btn-bt-view').addEventListener('click', async () => {
  const date = $('pk-date-input').value;
  $('bt-status').textContent = '加载近期回测…';
  let picks;
  try { picks = await DailyPickHandler.GetLatestPicks(10); } catch (_) { picks = []; }
  if (!picks || !picks.length) { $('bt-status').textContent = '无候选可供回测展示'; return; }
  const out = {};
  for (const p of picks) {
    const code = pick(p, ['stockCode', 'StockCode']);
    if (!code) continue;
    try { out[code] = await BacktestService.RunSingleBacktest(code, date, 0, 5, -0.05, 0.10, true); }
    catch (e) { out[code] = { error: (e && e.message) || String(e) }; }
  }
  $('bt-result-card').style.display = '';
  $('bt-result').textContent = JSON.stringify(out, null, 2);
  $('bt-status').textContent = '已加载';
});

// ================================ 页4：持仓 ================================
async function loadHoldings() {
  const box = $('pos-holdings');
  try {
    const d = await TradingRecordHandler.GetHoldingsDetail();
    if (!d) { box.innerHTML = '<div class="empty">暂无持仓数据</div>'; return; }
    const lines = [];
    const total = pick(d, ['TotalAssets', 'Total', 'totalAssets'], '');
    const cash = pick(d, ['CashAmount', 'cashAmount'], '');
    const mv = pick(d, ['MarketValue', 'marketValue'], '');
    const pct = pick(d, ['TotalPct', 'totalPct'], '');
    if (total !== '') lines.push(['总资产', fmtNum(total)]);
    if (cash !== '') lines.push(['现金', fmtNum(cash)]);
    if (mv !== '')  lines.push(['市值', fmtNum(mv)]);
    if (pct !== '') lines.push(['盈亏', fmtPct(pct)]);
    const holds = pick(d, ['Holdings', 'holdings', 'Stocks', 'stocks'], []);
    let h = '';
    for (const s of (holds || [])) {
      const code = pick(s, ['StockCode', 'stockCode']);
      const name = pick(s, ['Name', 'name'], '');
      const px = +(pick(s, ['Price', 'price', 'CurrentPrice', 'currentPrice'], 0) || 0);
      const chg = pick(s, ['ChangePercent', 'changePercent', 'Pct', 'pct'], '');
      const pnl = pick(s, ['Profit', 'profit', 'Pnl', 'pnl'], '');
      h += `<div class="kv"><span class="k">${esc(name || code)} <span class="tag">${esc(code)}</span></span><span class="${cls(chg)}">${px ? fmtNum(px) : '--'} ${fmtPct(chg)} ${pnl !== '' ? ' ' + fmtNum(pnl) : ''}</span></div>`;
    }
    box.className = '';
    box.innerHTML = (lines || []).map(([k, v]) => `<div class="kv"><span class="k">${k}</span><span>${esc(v == null ? '--' : v)}</span></div>`).join('') + (h || '');
  } catch (e) { box.innerHTML = '<div class="empty">持仓加载失败: ' + esc(e && e.message || e) + '</div>'; }
}

async function loadTradeRecords() {
  const box = $('pos-records');
  try {
    let list = null;
    try {
      list = await TradingRecordHandler.GetTradingRecordList({ Page: 1, PageSize: 10, StartDate: '', EndDate: '' });
      if (list && !Array.isArray(list) && Array.isArray(list.Data)) list = list.Data;
    } catch (_) { list = []; }
    if (!list || !list.length) { box.innerHTML = '<div class="empty">暂无交易记录（下方可加一条）</div>'; return; }
    box.innerHTML = list.slice(0, 10).map((r) => {
      const code = pick(r, ['StockCode', 'stockCode']);
      const name = pick(r, ['Name', 'name'], '');
      const side = pick(r, ['Side', 'side'], '');
      const px = +(pick(r, ['Price', 'price'], 0));
      const vol = pick(r, ['Volume', 'volume'], '');
      const date = (pick(r, ['TradeDate', 'tradeDate', 'CreatedAt', 'createdAt'], '') || '').slice(0, 10);
      const sideTag = side.toLowerCase() === 'buy' ? '买' : (side.toLowerCase() === 'sell' ? '卖' : side);
      return `<div class="kv"><span class="k">${esc(date)} ${esc(name || code)} <span class="tag">${esc(code)}</span> <span class="tag">${esc(sideTag)}</span></span>
      <span>${px ? fmtNum(px) : '--'} ${vol !== '' ? '× ' + esc(vol) : ''}</span></div>`;
    }).join('');
  } catch (e) { box.innerHTML = '<div class="empty">加载失败: ' + esc(e) + '</div>'; }
}

$('btn-nr-add').addEventListener('click', async () => {
  const code = $('nr-code').value.trim();
  const price = +$('nr-price').value;
  const side = $('nr-side').value;
  if (!code) { $('nr-status').textContent = '代码不能为空'; return; }
  if (!price) { $('nr-status').textContent = '价格不能为空'; return; }
  try {
    await TradingRecordHandler.AddTradingRecord({
      StockCode: code, Name: '', Price: price, Volume: 0, Side: side, TradeDate: today(),
    });
    $('nr-status').textContent = '已添加';
    loadTradeRecords(); loadHoldings();
  } catch (e) { $('nr-status').textContent = '失败: ' + (e && e.message || e); }
});

async function loadFunds() {
  const box = $('pos-funds');
  try {
    const list = (await FundHandler.GetFollowedFund()) || [];
    const arr = Array.isArray(list) ? list : Object.values(list);
    if (!arr || !arr.length) { box.innerHTML = '<div class="empty">暂无自选基金（下方可加）</div>'; return; }
    box.innerHTML = arr.slice(0, 30).map((f) => {
      const code = pick(f, ['Code', 'code']);
      const name = pick(f, ['Name', 'name']);
      const nv = pick(f, ['Nv', 'nv', 'UnitNv', 'unitNv', 'NetValue', 'netValue'], '');
      const pct = pick(f, ['NvChg', 'nvChg', 'NetValueChange', 'netValueChange', 'Pct', 'pct'], '');
      return `<div class="kv"><span class="k">${esc(name || code)} <span class="tag">${esc(code)}</span></span><span class="${cls(pct)}">${nv !== '' ? fmtNum(nv) : '—'} ${fmtPct(pct)}</span></div>`;
    }).join('');
  } catch (e) { box.innerHTML = '<div class="empty">加载失败: ' + esc(e) + '</div>'; }
}
$('btn-fd-follow').addEventListener('click', async () => { const c = $('fd-code').value.trim(); if (!c) return; try { await FundHandler.FollowFund(c); loadFunds(); $('fd-status').textContent = '已关注'; } catch (e) { $('fd-status').textContent = '失败: ' + e; } });
$('btn-fd-unfollow').addEventListener('click', async () => { const c = $('fd-code').value.trim(); if (!c) return; try { await FundHandler.UnFollowFund(c); loadFunds(); $('fd-status').textContent = '已取关'; } catch (e) { $('fd-status').textContent = '失败: ' + e; } });

async function loadChanges() {
  const box = $('pos-changes');
  try {
    let items = null;
    try { items = await StockChangeHandler.GetChangeRank(1, 20); } catch (_) {}
    const arr = (items && (Array.isArray(items) ? items : (Array.isArray(items.List) ? items.List : (Array.isArray(items.Data) ? items.Data : [])))) || [];
    if (!arr.length) { box.innerHTML = '<div class="empty">暂无当日异动记录</div>'; return; }
    box.innerHTML = arr.slice(0, 20).map((c) => {
      const code = pick(c, ['StockCode', 'stockCode']);
      const name = pick(c, ['Name', 'name']);
      const type = pick(c, ['ChangeType', 'changeType', 'Type', 'type'], '');
      const time = (pick(c, ['ChangeTime', 'changeTime', 'Time', 'time'], '') || '').toString().slice(11, 16);
      return `<div class="kv"><span class="k">${esc(time)} ${esc(name || code)} <span class="tag">${esc(code)}</span> <span class="tag">${esc(type)}</span></span><span></span></div>`;
    }).join('');
  } catch (e) { box.innerHTML = '<div class="empty">加载失败: ' + esc(e) + '</div>'; }
}
$('btn-ch-refresh').addEventListener('click', loadChanges);

$('btn-pos-refresh').addEventListener('click', () => { loadHoldings(); loadTradeRecords(); });
$('btn-pos-sum').addEventListener('click', async () => {
  const btn = $('btn-pos-sum');
  btn.disabled = true;
  const old = btn.textContent; btn.textContent = '运行中…';
  try {
    const cid = Number($('ai-config').value || 0);
    if (!cid) { alert('缺 AI 配置'); btn.textContent = old; return; }
    const ev = 'SummarizeHolding-' + Date.now();
    // 事件流：summarizeHoldings（若后端用） + 直接看返回
    const r = await TradingRecordHandler.SummarizeHoldings(cid, ev);
    switchTab('ai');
    appendChat('持仓 AI 摘要\n' + (r && (r.Summary ?? r.summary ?? r.Text ?? JSON.stringify(r))) , false);
  } catch (e) { alert('摘要失败: ' + (e && e.message || e)); }
  finally { btn.textContent = old; btn.disabled = false; }
});

function reloadPosition() { loadHoldings(); loadTradeRecords(); loadFunds(); loadChanges(); }

// ================================ 启动 ================================
(async () => {
  await Promise.allSettled([
    loadMarketStat(), loadGlobalIdx(), loadFollowList(), loadNews(),
    loadAiConfigs(), loadDailyPickStats(),
  ]);
})();
