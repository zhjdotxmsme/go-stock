import * as Runtime from '@wailsio/runtime';
import * as echarts from 'echarts';

import { StockHandler, AgentHandler } from './bindings/go-stock/backend/handler';
import { MobileService } from './bindings/go-stock/mobile';

const Events = Runtime.Events;

const $ = (id) => document.getElementById(id);

// ---- Tab 切换 ----
document.querySelectorAll('nav.tabbar button').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('nav.tabbar button').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('.page').forEach((p) => p.classList.remove('active'));
    btn.classList.add('active');
    $('page-' + btn.dataset.page).classList.add('active');
    $('page-title').innerText = btn.dataset.page === 'market' ? '行情' : 'AI 问答';
  });
});

// ---- 行情页:自选列表 + K线 ----
let chart = null;

function fmtPct(p) {
  if (p === undefined || p === null || isNaN(p)) return '';
  const v = Number(p);
  return (v >= 0 ? '+' : '') + v.toFixed(2) + '%';
}

async function loadFollowList() {
  const box = $('follow-list');
  try {
    const list = (await StockHandler.GetFollowList(0)) || [];
    if (!list.length) {
      box.innerHTML = '<div class="empty">暂无自选股(可在桌面端添加)</div>';
      return;
    }
    box.innerHTML = '';
    for (const s of list) {
      const item = document.createElement('div');
      item.className = 'stock-item';
      const pct = fmtPct(s.ChangePercent);
      const cls = Number(s.ChangePercent) >= 0 ? 'up' : 'down';
      item.innerHTML = `
        <div><div class="s-name">${s.Name ?? ''}</div><div class="s-code">${s.StockCode ?? ''}</div></div>
        <div class="s-price"><div class="${cls}">${s.Price ?? '--'}</div><div class="${cls}">${pct}</div></div>`;
      item.addEventListener('click', () => loadKLine(s.StockCode, s.Name));
      box.appendChild(item);
    }
  } catch (e) {
    box.innerHTML = '<div class="empty">自选列表加载失败: ' + (e?.message || e) + '</div>';
  }
}

async function loadKLine(code, name) {
  if (!code) return;
  $('market-status').innerText = '加载K线中...';
  try {
    const klines = (await StockHandler.GetStockKLine(code, name || '', 120)) || [];
    if (!klines.length) {
      $('market-status').innerText = '未获取到K线数据';
      return;
    }
    const dates = klines.map((k) => k.Date);
    const kdata = klines.map((k) => [k.Open, k.Close, k.Low, k.High]);
    const volumes = klines.map((k) => k.Volume);
    $('quote-name').innerText = (name || '') + ' ' + code;
    const last = klines[klines.length - 1];
    const pct = fmtPct(last?.ChangePercent);
    $('quote-change').innerText = (last?.Close ?? '--') + '  ' + pct;
    $('quote-change').className = 'quote-change ' + (Number(last?.ChangePercent) >= 0 ? 'up' : 'down');

    if (!chart) chart = echarts.init($('kline-chart'));
    chart.setOption({
      animation: false,
      grid: [{ left: 50, right: 12, top: 10, height: '58%' }, { left: 50, right: 12, top: '74%', height: '18%' }],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#888', fontSize: 10 } },
        { type: 'category', gridIndex: 1, data: dates, axisLabel: { show: false }, axisLine: { show: false } },
      ],
      yAxis: [
        { scale: true, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#888', fontSize: 10 }, splitLine: { lineStyle: { color: '#222' } } },
        { gridIndex: 1, axisLabel: { show: false }, splitLine: { show: false } },
      ],
      dataZoom: [{ type: 'inside', xAxisIndex: [0, 1], start: 60, end: 100 }],
      tooltip: { trigger: 'axis', axisPointer: { type: 'cross' }, backgroundColor: '#1e1e1e', borderColor: '#333', textStyle: { color: '#ddd', fontSize: 11 } },
      series: [
        {
          type: 'candlestick', name: 'K线', data: kdata,
          itemStyle: { color: '#e05656', color0: '#21b978', borderColor: '#e05656', borderColor0: '#21b978' },
        },
        { type: 'bar', name: '成交量', xAxisIndex: 1, yAxisIndex: 1, data: volumes, itemStyle: { color: '#3a5a8c' } },
      ],
    });
    $('market-status').innerText = '';
  } catch (e) {
    $('market-status').innerText = 'K线加载失败: ' + (e?.message || e);
  }
}

$('btn-load-kline').addEventListener('click', () => loadKLine($('stock-code').value.trim(), ''));

// ---- AI 问答页 ----
let aiBusy = false;
let streaming = null; // 当前流式 DOM 节点
let fullText = '';

async function loadAiConfigs() {
  try {
    const configs = (await MobileService.ListAiConfigs()) || [];
    const sel = $('ai-config');
    sel.innerHTML = '';
    if (!configs.length) {
      const opt = document.createElement('option');
      opt.textContent = '未配置 AI(请在桌面端配置)';
      opt.value = '0';
      sel.appendChild(opt);
      return;
    }
    for (const c of configs) {
      const opt = document.createElement('option');
      opt.value = c.id;
      opt.textContent = `${c.name} (${c.modelName})`;
      sel.appendChild(opt);
    }
  } catch (e) {
    $('ai-status').innerText = 'AI 配置加载失败: ' + (e?.message || e);
  }
}

function appendUserMsg(text) {
  const div = document.createElement('div');
  div.className = 'chat-msg user';
  div.textContent = text;
  $('chat-box').appendChild(div);
}

function ensureStreamingNode() {
  if (!streaming) {
    streaming = document.createElement('div');
    streaming.className = 'chat-msg';
    const reasoning = document.createElement('div');
    reasoning.className = 'reasoning';
    const content = document.createElement('div');
    content.className = 'content';
    streaming.appendChild(reasoning);
    streaming.appendChild(content);
    streaming._reasoning = reasoning;
    streaming._content = content;
    $('chat-box').appendChild(streaming);
  }
  return streaming;
}

function handleAgentMessage(payload) {
  // v3 事件负载可能是 {data: ...} 包装,做一次解包
  const msg = payload && typeof payload === 'object' && 'data' in payload ? payload.data : payload;
  if (msg === 'agent-DONE') {
    finishStream('完成');
    return;
  }
  if (!msg || typeof msg !== 'object') return;
  const node = ensureStreamingNode();
  if (msg.role === 'user') return; // 不回显用户消息
  if (msg.reasoning_content) node._reasoning.textContent = msg.reasoning_content;
  if (msg.content) {
    if (msg.content === 'agent-DONE') { finishStream('完成'); return; }
    fullText += msg.content;
    node._content.textContent = fullText;
    $('chat-box').scrollTop = $('chat-box').scrollHeight;
  }
}

let streamOff = null;
function ensureStreamSubscribed() {
  if (streamOff) return;
  streamOff = Events.On('agent-message', handleAgentMessage);
}

function finishStream(text) {
  aiBusy = false;
  streaming = null;
  fullText = '';
  $('ai-status').innerText = text;
  $('btn-ask').disabled = false;
}

$('btn-ask').addEventListener('click', async () => {
  const question = $('ai-question').value.trim();
  const aiConfigId = Number($('ai-config').value || 0);
  if (!question || aiBusy) return;
  if (!aiConfigId) {
    $('ai-status').innerText = '请先在桌面端配置 AI 后重试';
    return;
  }
  aiBusy = true;
  fullText = '';
  streaming = null;
  $('ai-status').innerText = 'AI 思考中...';
  $('btn-ask').disabled = true;
  appendUserMsg(question);
  ensureStreamSubscribed();
  try {
    // ChatWithAgent(question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionID)
    await AgentHandler.ChatWithAgent(question, aiConfigId, null, false, 0, false, 'quick', 'mobile');
  } catch (e) {
    finishStream('请求失败: ' + (e?.message || e));
  }
});

$('btn-abort').addEventListener('click', async () => {
  try { await AgentHandler.AbortChatWithAgent(); } catch { /* ignore */ }
  finishStream('已中断');
});

// ---- 启动 ----
loadFollowList();
loadAiConfigs();
