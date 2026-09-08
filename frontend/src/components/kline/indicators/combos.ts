/**
 * 组合指标预设：点击后只开启该组指标（其余自动关闭），图面更干净。
 * 全部按「日线短线」调校：持仓约 2 天 ~ 3 周，主看日线周期；
 * 不适用于分钟级超短线（噪声大、假突破多），也不适用于周线/月线超长线（信号滞后）。
 * keys 与 useIndicatorToggles 中的指标 key 一一对应。
 */

export const commonCombos = [
  {
    key: 'classic',
    label: '经典入门',
    keys: ['ma', 'boll', 'macd', 'kdj'],
    tip: 'MA + BOLL + MACD + KDJ\n最通用的入门四件套，覆盖趋势/位置/买卖点/时机\n✅ MA看方向：站上MA20偏多，跌破偏空\n✅ BOLL看位置：触上轨不追、触下轨不杀\n✅ MACD定买卖：零轴上金叉买、零轴下死叉卖\n✅ KDJ找时点：20以下金叉比高位金叉可靠得多\n用法：日线四个指标同向时信号最强；相互矛盾时以MACD为准',
  },
  {
    key: 'trendSwing',
    label: '趋势波段',
    keys: ['ema', 'macd', 'adx'],
    tip: 'EMA + MACD + ADX\n做日线上升趋势中的波段，持仓约3~10天\n✅ EMA12>EMA21 = 短线趋势向上，只做多不做空\n✅ ADX>25 才说明有趋势，<20 时观望不做\n✅ MACD柱体放大=趋势加速，持续缩小=减速警惕\n用法：ADX上穿25 + EMA多头排列 + MACD零轴上金叉 = 最佳波段买点；EMA12下穿EMA21离场',
  },
  {
    key: 'rangeDip',
    label: '震荡低吸',
    keys: ['boll', 'kdj', 'rsi'],
    tip: 'BOLL + KDJ + RSI\n专做震荡市的低吸高抛，持仓约2~5天\n✅ 股价触BOLL下轨 + KDJ在20以下金叉 = 低吸点\n✅ 股价触BOLL上轨 + RSI>70拐头 = 高抛点\n⚠️ 仅限横盘震荡行情！单边下跌中用会接飞刀，单边上涨中会卖飞\n用法：先看BOLL是否横向收口确认震荡，再按上下轨操作；BOLL开口立即停用本组合',
  },
  {
    key: 'volPrice',
    label: '量价确认',
    keys: ['obv', 'mfi', 'cmf'],
    tip: 'OBV + MFI + CMF\n用资金与成交量验证价格信号的真伪\n✅ 上涨必须放量：OBV同步上行的上涨才可信\n✅ MFI>80 资金过热防回调，<20 资金冰点看反弹\n✅ CMF>0 资金净流入，<0 净流出\n✅ 价涨但OBV走平/下降 = 量价背离，上涨乏力准备减仓\n用法：日线突破关键位时，三个指标至少两个配合才算有效突破',
  },
]

export const advancedCombos = [
  {
    key: 'ttmBreak',
    label: 'TTM挤压突破',
    keys: ['ttmSqueeze', 'boll', 'keltner'],
    tip: 'TTM Squeeze + BOLL + Keltner\n捕捉横盘蓄力后的爆发性突破，持仓约3~10天\n✅ BOLL收窄进入Keltner通道内 = 挤压（黄点），波动率极低，大行情正在酝酿\n✅ 黄点转绿点 = 挤压释放，趋势正式启动\n✅ 动量柱>0 向上突破做多，<0 向下突破回避\n用法：日线上黄点连续出现5根以上重点盯守，转绿首日且放量=最佳入场；止损设在挤压区间下沿',
  },
  {
    key: 'ichimokuTrend',
    label: '一目均衡趋势',
    keys: ['ichimoku', 'adx'],
    tip: '一目均衡表 + ADX\n日本机构经典趋势系统，过滤震荡只做单边\n✅ 价格在云层上方=多头市场，只做多；下方=只观望\n✅ 转换线上穿基准线 + 价格在云上方 = 强买入信号\n✅ 云层越厚 = 支撑/压力越强，薄云容易被突破\n✅ ADX>25 确认趋势有效，过滤掉云层附近的假突破\n用法：日线放量突破云层且ADX走强时介入；收盘跌回云层内止损',
  },
  {
    key: 'alligatorAO',
    label: '鳄鱼动量',
    keys: ['alligator', 'ao', 'fractal'],
    tip: '鳄鱼线 + AO + 分形\nBill Williams 经典系统：等趋势张嘴再进场\n✅ 鳄鱼三线纠缠 = 沉睡期，坚决不操作\n✅ 三线开始发散张嘴 = 趋势启动，顺势入场\n✅ AO柱与价格同向放大 = 动量确认，可加仓\n✅ 向上突破最近的顶分形 = 精确入场点\n用法：日线张嘴初期介入，三线重新收口缠绕时离场；纠缠期频繁操作是本系统最大禁忌',
  },
  {
    key: 'elderTriple',
    label: 'Elder三重过滤',
    keys: ['ema', 'forceIndex', 'elderRay'],
    tip: 'EMA + ForceIndex + ElderRay\nElder 三重滤网：顺势+等回调+抓衰竭，持仓约3~8天\n✅ 第一重：EMA方向定趋势，只顺不逆（EMA向上只做多）\n✅ 第二重：上升趋势中ForceIndex回落=回调到位，准备买\n✅ 第三重：BearPower<0且开始收窄=空头衰竭，精确买入\n用法：日线EMA向上时，耐心等回调中BearPower从低点回升再买入；EMA拐头向下立即止损离场',
  },
  {
    key: 'satsAdaptive',
    label: '自适应趋势',
    keys: ['sats', 'atr', 'adx'],
    tip: 'SATS + ATR + ADX\n自适应趋势系统：趋势强时灵敏、震荡时迟钝，自动调节\n✅ SATS红线=持股看多，绿线=空仓观望，变色即切换\n✅ TQI趋势质量高时通道自动收窄，锁定利润更紧\n✅ ADX>25 确认趋势成立，震荡期不追信号\n✅ 止损=入场价-2×ATR，随价格上涨同步上移\n用法：日线SATS转红+ADX走强买入，用2倍ATR移动止损让利润奔跑，转绿无条件离场',
  },
  {
    key: 'smcStructure',
    label: '聪明钱结构',
    keys: ['smc', 'macd', 'signalRatio'],
    tip: 'SMC + MACD + 信号比\n跟踪机构资金的市场结构，做结构突破与反转\n✅ BOS蓝点=结构突破，趋势大概率延续\n✅ CHoCH黄点=特征转变，趋势可能反转，高度警惕\n✅ MACD与突破方向同向 = 动能配合，胜率更高\n✅ 信号比净信号>0且放大 = 多指标共振看多\n用法：日线CHoCH出现后等回踩入场，BOS确认后加仓；跌破结构低点(摆动低点)止损',
  },
]

export const allCombos = [...commonCombos, ...advancedCombos]
