"""
Auto-Research MCP Server — automated stock research & market intelligence.

Provides 6 tools for web-powered stock research through FastMCP:
  - get_market_news           Latest financial/market news
  - get_company_overview      Comprehensive company overview
  - get_industry_analysis     Industry/sector research data
  - calculate_technical_indicators  SMA, RSI, MACD, Bollinger Bands
  - get_earnings_summary      Earnings/financial performance summary
  - search_company_news       Company-specific news search

Runs via stdio transport for go-stock's MCP server integration.
"""

import json
from datetime import datetime, timedelta
from typing import Optional

import pandas as pd
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("auto-research-server")

# ---------------------------------------------------------------------------
# Optional dependency checks — graceful fallbacks when libraries absent
# ---------------------------------------------------------------------------

YFINANCE_AVAILABLE = False
try:
    import yfinance as yf

    YFINANCE_AVAILABLE = True
except ImportError:
    yf = None  # type: ignore[assignment]

BS4_AVAILABLE = False
try:
    from bs4 import BeautifulSoup

    BS4_AVAILABLE = True
except ImportError:
    BeautifulSoup = None  # type: ignore[assignment]

REQUESTS_AVAILABLE = False
try:
    import requests

    REQUESTS_AVAILABLE = True
except ImportError:
    requests = None  # type: ignore[assignment]

NUMPY_AVAILABLE = False
try:
    import numpy as np

    NUMPY_AVAILABLE = True
except ImportError:
    np = None  # type: ignore[assignment]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_YF_NA = "yfinance 库未安装，请执行 pip install yfinance 后重试"


def _require_yfinance() -> str | None:
    """Return error string if yfinance unavailable, else None."""
    return None if YFINANCE_AVAILABLE else _YF_NA


def _get_ticker(symbol: str) -> object:
    """Create a validated yfinance Ticker object."""
    return yf.Ticker(symbol.strip().upper())  # type: ignore[union-attr]


def _check_info(info: dict, symbol: str) -> str | None:
    """Return error message if info is unusable, else None."""
    if not info:
        return f"无法获取 {symbol} 的数据，请检查股票代码是否正确"
    if info.get("regularMarketPrice") is None and info.get("currentPrice") is None:
        return f"无法获取 {symbol} 的数据，请检查股票代码是否正确"
    return None


def _safe(info: dict, *keys: str, default="N/A") -> str:
    """Traverse nested keys returning formatted value or default."""
    val: object = info
    for k in keys:
        if not isinstance(val, dict):
            return default
        val = val.get(k, {})
    if val is None or val == {} or (isinstance(val, float) and pd.isna(val)):
        return default
    if isinstance(val, float):
        return f"{val:,.2f}"
    return str(val)


def _fmt_market_cap(info: dict) -> str:
    """Format market cap in human-readable units."""
    mc = info.get("marketCap")
    if mc is None or (isinstance(mc, float) and pd.isna(mc)):
        return "N/A"
    if mc >= 1e12:
        return f"${mc / 1e12:,.2f}T"
    if mc >= 1e9:
        return f"${mc / 1e9:,.2f}B"
    if mc >= 1e6:
        return f"${mc / 1e6:,.2f}M"
    return f"${mc:,.0f}"


def _parse_date(ts) -> str:
    """Convert timestamp to date string."""
    try:
        if isinstance(ts, (int, float)):
            return datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M")
        return str(ts)
    except (ValueError, OSError):
        return str(ts)


def _now_str() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


# ---------------------------------------------------------------------------
# Tool 1 — Market News
# ---------------------------------------------------------------------------

@mcp.tool()
def get_market_news(keywords: str, max_results: int = 5) -> str:
    """获取最新金融/市场新闻

    Args:
        keywords: 逗号分隔的关键词，如 "AI,stock market,technology"
        max_results: 返回结果数量上限（默认 5）
    """
    if max_results < 1:
        return "max_results 必须大于 0"
    kw_list = [k.strip() for k in keywords.split(",") if k.strip()]
    if not kw_list:
        return "请提供至少一个关键词"
    news_items = []

    # Strategy 1 — yfinance Search.news
    if YFINANCE_AVAILABLE:
        try:
            search = yf.Search(" ".join(kw_list[:3]))  # type: ignore[union-attr]
            raw = getattr(search, "news", [])
            if raw:
                for item in raw[:max_results]:
                    title = item.get("title", "")
                    pub_date = _parse_date(item.get("providerPublishTime", ""))
                    link = item.get("link", "")
                    source = item.get("publisher", "")
                    news_items.append(
                        f"- **{title}**  [{source}]({link})  ({pub_date})"
                    )
        except Exception:
            pass

    # Strategy 2 — requests + free RSS feed (Yahoo Finance RSS)
    if not news_items and REQUESTS_AVAILABLE:
        try:
            # Yahoo Finance RSS for market news
            query = "+".join(kw_list)
            rss_url = f"https://finance.yahoo.com/rss/headline?s={query}"
            resp = requests.get(rss_url, timeout=10, headers={
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
            })
            if resp.status_code == 200 and BS4_AVAILABLE:
                soup = BeautifulSoup(resp.content, "lxml")
                for item in soup.find_all("item")[:max_results]:
                    title = item.find("title")
                    link = item.find("link")
                    pub_date = item.find("pubDate")
                    news_items.append(
                        f"- **{title.text if title else ''}**  "
                        f"({pub_date.text if pub_date else ''})"
                    )
        except Exception:
            pass

    # Strategy 3 — fallback with curated market topics
    if not news_items:
        return (
            f"### 📰 市场新闻: {', '.join(kw_list)}\n\n"
            f"_当前未获取到匹配新闻，请稍后重试或尝试其他关键词。_\n"
            f"查询时间: {_now_str()}"
        )

    formatted = (
        f"### 📰 市场新闻: {', '.join(kw_list)}\n\n"
        + "\n".join(news_items[:max_results])
    )
    return formatted


# ---------------------------------------------------------------------------
# Tool 2 — Company Overview
# ---------------------------------------------------------------------------

@mcp.tool()
def get_company_overview(symbol: str, exchange: str = "") -> str:
    """获取公司综合概览（描述、行业、关键指标等）

    Args:
        symbol: 股票代码，如 "AAPL"
        exchange: 交易所（可选），"US"/"HK"/"SH"/"SZ"
    """
    err = _require_yfinance()
    if err:
        return err

    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err2 = _check_info(info, symbol)
        if err2:
            return err2

        # Company description
        desc = info.get("longBusinessSummary", "暂无公司描述")
        if len(desc) > 500:
            desc = desc[:500] + "…"

        # Recent news
        news_section = ""
        try:
            search = yf.Search(symbol)  # type: ignore[union-attr]
            raw = getattr(search, "news", [])
            if raw:
                news_items = []
                for item in raw[:5]:
                    title = item.get("title", "")
                    link = item.get("link", "")
                    news_items.append(f"- [{title}]({link})")
                if news_items:
                    news_section = "\n**近期新闻**\n" + "\n".join(news_items)
        except Exception:
            pass

        exchange_label = exchange.upper() if exchange else _safe(info, "exchange")
        lines = [
            f"## {info.get('longName', info.get('shortName', symbol))} ({symbol.upper()})",
            f"",
            f"| 字段 | 值 |",
            f"|------|-----|",
            f"| **交易所** | {exchange_label} |",
            f"| **行业** | {_safe(info, 'sector')} |",
            f"| **细分行业** | {_safe(info, 'industry')} |",
            f"| **市值** | {_fmt_market_cap(info)} |",
            f"| **当前价格** | ${_safe(info, 'currentPrice', 'regularMarketPrice')} |",
            f"| **52周最高** | ${_safe(info, 'fiftyTwoWeekHigh')} |",
            f"| **52周最低** | ${_safe(info, 'fiftyTwoWeekLow')} |",
            f"| **市盈率 (PE)** | {_safe(info, 'trailingPE', 'forwardPE')} |",
            f"| **市净率 (PB)** | {_safe(info, 'priceToBook')} |",
            f"| **股息率** | {_safe(info, 'dividendYield')} |",
            f"| **员工数** | {_safe(info, 'fullTimeEmployees')} |",
            f"| **国家** | {_safe(info, 'country')} |",
            f"",
            f"### 公司简介",
            f"{desc}",
            f"",
            f"### 关键财务指标",
            f"| 指标 | 值 |",
            f"|------|-----|",
            f"| **总收入 (TTM)** | {_safe(info, 'totalRevenue')} |",
            f"| **净利润 (TTM)** | {_safe(info, 'netIncomeToCommon', 'netIncome')} |",
            f"| **基本每股收益** | {_safe(info, 'trailingEps')} |",
            f"| **营收增长率** | {_safe(info, 'revenueGrowth')} |",
            f"| **毛利率** | {_safe(info, 'grossMargins')} |",
            f"| **营业利润率** | {_safe(info, 'operatingMargins')} |",
            f"| **ROE** | {_safe(info, 'returnOnEquity')} |",
            f"| **负债总额** | {_safe(info, 'totalDebt')} |",
            f"| **自由现金流** | {_safe(info, 'freeCashflow')} |",
        ]
        if news_section:
            lines.extend(["", news_section])

        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 公司概览失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 3 — Industry Analysis
# ---------------------------------------------------------------------------

# Known sector keywords -> industry description mapping
_SECTOR_INFO: dict[str, tuple[str, list[str]]] = {
    "technology": (
        "科技行业涵盖软件开发、硬件制造、半导体、互联网服务等细分领域。"
        "该行业具有创新驱动、高研发投入、快速迭代的特点。",
        ["AAPL", "MSFT", "GOOGL", "AMZN", "META", "NVDA", "TSM", "CRM", "ADBE", "ORCL"],
    ),
    "finance": (
        "金融行业包括银行、保险、证券、资产管理等细分领域。"
        "该行业受宏观经济、利率政策和监管环境影响较大。",
        ["JPM", "BAC", "GS", "MS", "WFC", "C", "BLK", "AXP", "V", "MA"],
    ),
    "healthcare": (
        "医疗健康行业包括制药、生物技术、医疗器械、医疗服务等细分领域。"
        "该行业具有强监管、高壁垒、研发周期长的特点。",
        ["JNJ", "PFE", "UNH", "MRK", "ABBV", "NVO", "LLY", "TMO", "MDT", "AMGN"],
    ),
    "energy": (
        "能源行业包括石油天然气、可再生能源、核电、新能源等细分领域。"
        "该行业与全球能源价格和政策高度相关。",
        ["XOM", "CVX", "COP", "SLB", "EOG", "NEE", "DUK", "BP", "SHEL", "TTE"],
    ),
    "consumer": (
        "消费行业包括日用消费品、奢侈品、零售、餐饮、电商等细分领域。"
        "该行业受消费者信心、可支配收入和消费趋势影响。",
        ["PG", "KO", "PEP", "WMT", "COST", "MCD", "SBUX", "NKE", "DIS", "AMZN"],
    ),
    "real estate": (
        "房地产行业涵盖住宅开发、商业地产、REITs、物业管理等细分领域。"
        "该行业受利率、城市化进程和宏观经济周期影响显著。",
        ["PLD", "AMT", "CCI", "EQIX", "SPG", "PSA", "O", "DLR", "AVB", "EQR"],
    ),
    "automotive": (
        "汽车行业包括传统汽车制造、新能源汽车、自动驾驶、零部件等细分领域。"
        "当前正处于电动化、智能化、网联化的转型期。",
        ["TSLA", "TM", "F", "GM", "VWAGY", "BMWYY", "MBGAF", "RIVN", "LCID", "LI"],
    ),
    "semiconductor": (
        "半导体行业是科技产业的基础，涵盖芯片设计、制造、封装测试、设备等环节。"
        "该行业具有周期性、高资本支出、技术密集型的特点。",
        ["NVDA", "TSM", "INTC", "AMD", "QCOM", "AVGO", "ASML", "MU", "TXN", "AMAT"],
    ),
}


@mcp.tool()
def get_industry_analysis(industry: str) -> str:
    """获取行业/板块研究数据

    Args:
        industry: 行业名称（英文或中文），如 "technology" / "科技"
    """
    kw = industry.strip().lower()

    # Try to match known sector
    matched_sector = None
    matched_stocks: list[str] = []
    sector_desc = ""

    # Direct key match
    if kw in _SECTOR_INFO:
        sector_desc, matched_stocks = _SECTOR_INFO[kw]
        matched_sector = kw

    # Fuzzy match against keys and Chinese aliases
    if not matched_sector:
        cn_aliases = {
            "科技": "technology", "it": "technology", "互联网": "technology",
            "金融": "finance", "银行": "finance", "保险": "finance",
            "医疗": "healthcare", "医药": "healthcare", "生物": "healthcare",
            "能源": "energy", "新能源": "energy", "石油": "energy",
            "消费": "consumer", "零售": "consumer", "电商": "consumer",
            "房地产": "real estate", "地产": "real estate",
            "汽车": "automotive", "新能源车": "automotive",
            "半导体": "semiconductor", "芯片": "semiconductor",
        }
        for alias, sector_key in cn_aliases.items():
            if alias in kw or kw in alias:
                sector_desc, matched_stocks = _SECTOR_INFO[sector_key]
                matched_sector = sector_key
                break

    if not matched_sector:
        # Return list of available industries
        available = list(_SECTOR_INFO.keys())
        return (
            f"未找到 \"{industry}\" 的行业数据\n\n"
            f"支持的行业关键词: {', '.join(available)}\n"
            f"(也支持中文: 科技/金融/医疗/能源/消费/地产/汽车/半导体)"
        )

    # Fetch real-time data for representative stocks if yfinance available
    stock_section = ""
    if YFINANCE_AVAILABLE:
        try:
            rows = []
            for sym in matched_stocks:
                try:
                    t = _get_ticker(sym)
                    info = t.info
                    price = info.get("currentPrice") or info.get("regularMarketPrice")
                    change = info.get("regularMarketChangePercent")
                    mc = info.get("marketCap")
                    if price:
                        chg_str = f"{change:+.2f}%" if change is not None else "N/A"
                        mc_str = _fmt_market_cap(info) if mc else "N/A"
                        rows.append(
                            f"| **{sym}** | ${price:,.2f} | {chg_str} | {mc_str} |"
                        )
                except Exception:
                    rows.append(
                        f"| **{sym}** | N/A | N/A | N/A |"
                    )
            if rows:
                stock_section = (
                    f"\n### 行业代表公司\n"
                    f"| 代码 | 最新价 | 涨跌幅 | 市值 |\n"
                    f"|------|--------|--------|------|\n"
                    + "\n".join(rows)
                )
        except Exception:
            stock_section = (
                f"\n### 行业代表公司\n"
                + ", ".join(matched_stocks)
            )

    # Determine sector display name
    sector_display = matched_sector.replace("_", " ").title()

    result = (
        f"### 🏭 行业研究: {sector_display}\n\n"
        f"**行业概览**\n{sector_desc}\n"
        f"{stock_section}\n\n"
        f"---\n*数据更新时间: {_now_str()}*"
    )
    return result


# ---------------------------------------------------------------------------
# Tool 4 — Technical Indicators
# ---------------------------------------------------------------------------

@mcp.tool()
def calculate_technical_indicators(symbol: str, period: str = "3mo") -> str:
    """计算股票技术指标（SMA、RSI、MACD、布林带）

    Args:
        symbol: 股票代码，如 "AAPL"
        period: 数据周期，可选 1d/5d/1mo/3mo/6mo/1y/2y/5y/10y/max，默认 3mo
    """
    err = _require_yfinance()
    if err:
        return err
    if not NUMPY_AVAILABLE:
        return "numpy 库未安装，请执行 pip install numpy 后重试"

    valid_periods = {"1d", "5d", "1mo", "3mo", "6mo", "1y", "2y", "5y", "10y", "max"}
    if period not in valid_periods:
        return f"无效周期: {period}。可选: {', '.join(sorted(valid_periods))}"

    try:
        stock = _get_ticker(symbol)
        hist = stock.history(period=period)
        if hist.empty:
            return f"未获取到 {symbol} 在 {period} 内的价格数据"

        close = hist["Close"]
        last_price = float(close.iloc[-1])
        n = len(close)

        # SMA
        sma20 = float(close.rolling(20).mean().iloc[-1]) if n >= 20 else None
        sma50 = float(close.rolling(50).mean().iloc[-1]) if n >= 50 else None

        # RSI(14)
        rsi_val: float | None = None
        if n >= 14:
            delta = close.diff()
            gain = delta.where(delta > 0, 0.0).rolling(14).mean()
            loss = (-delta.where(delta < 0, 0.0)).rolling(14).mean()
            rs = gain / loss
            rsi_series = 100.0 - (100.0 / (1.0 + rs))
            rsi_val = float(rsi_series.iloc[-1])

        # MACD
        ema12 = close.ewm(span=12).mean()
        ema26 = close.ewm(span=26).mean()
        macd_line = ema12 - ema26
        signal_line = macd_line.ewm(span=9).mean()
        macd_val = float(macd_line.iloc[-1])
        signal_val = float(signal_line.iloc[-1])
        hist_val = macd_val - signal_val

        # Bollinger Bands
        bb_mid: float | None = None
        bb_upper: float | None = None
        bb_lower: float | None = None
        if n >= 20:
            bb_mid = float(close.rolling(20).mean().iloc[-1])
            bb_std = float(close.rolling(20).std().iloc[-1])
            bb_upper = bb_mid + 2.0 * bb_std
            bb_lower = bb_mid - 2.0 * bb_std

        # Sentiment labels
        sma20_label = ""
        if sma20 is not None:
            sma20_label = "📈 多头" if last_price > sma20 else "📉 空头"
        sma50_label = ""
        if sma50 is not None:
            sma50_label = "📈 多头" if last_price > sma50 else "📉 空头"

        rsi_label = ""
        if rsi_val is not None:
            if rsi_val > 70:
                rsi_label = "⚠️ 超买"
            elif rsi_val < 30:
                rsi_label = "⚠️ 超卖"
            else:
                rsi_label = "✅ 中性"

        macd_label = "📈 金叉" if macd_val > signal_val else "📉 死叉"

        lines = [
            f"### 📊 {symbol.upper()} 技术指标分析",
            f"",
            f"**基础数据**",
            f"- 最新收盘价: ${last_price:.2f}",
            f"- 数据周期: {period}",
            f"- 数据点数: {n}",
            f"",
            f"**移动平均线 (MA)**",
            f"| 指标 | 值 | 信号 |",
            f"|------|-----|------|",
        ]
        if sma20 is not None:
            lines.append(f"| SMA(20) | ${sma20:.2f} | {sma20_label} |")
        else:
            lines.append(f"| SMA(20) | N/A (需≥20个数据点) | — |")

        if sma50 is not None:
            lines.append(f"| SMA(50) | ${sma50:.2f} | {sma50_label} |")
        else:
            lines.append(f"| SMA(50) | N/A (需≥50个数据点) | — |")

        lines.extend([
            f"",
            f"**RSI(14)**",
            f"- 值: {rsi_val:.2f}" if rsi_val is not None else "- 值: N/A (需≥14个数据点)",
        ])
        if rsi_label:
            lines[-1] += f"  — {rsi_label}"

        lines.extend([
            f"",
            f"**MACD**",
            f"| 指标 | 值 |",
            f"|------|-----|",
            f"| MACD | {macd_val:.4f} |",
            f"| Signal | {signal_val:.4f} |",
            f"| Histogram | {hist_val:.4f} |",
            f"| 信号 | {macd_label} |",
            f"",
            f"**布林带 (Bollinger Bands)**",
            f"| 轨道 | 值 |",
            f"|------|-----|",
        ])
        if bb_upper is not None:
            lines.append(f"| 上轨 | ${bb_upper:.2f} |")
            lines.append(f"| 中轨 | ${bb_mid:.2f} |")
            lines.append(f"| 下轨 | ${bb_lower:.2f} |")
            if last_price > bb_upper:
                lines.append(f"| 位置 | 🔴 价格突破上轨，可能超买 |")
            elif last_price < bb_lower:
                lines.append(f"| 位置 | 🟢 价格跌破下轨，可能超卖 |")
            else:
                lines.append(f"| 位置 | ✅ 价格在轨道内运行 |")
        else:
            lines.append("| N/A (需≥20个数据点) | — |")

        lines.extend([
            f"",
            f"---",
            f"*数据更新时间: {_now_str()}*",
        ])

        return "\n".join(lines)
    except Exception as e:
        return f"计算 {symbol} 技术指标失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 5 — Earnings Summary
# ---------------------------------------------------------------------------

@mcp.tool()
def get_earnings_summary(symbol: str, year: Optional[int] = None) -> str:
    """获取公司财报摘要（营收、利润、利润率对比）

    Args:
        symbol: 股票代码，如 "AAPL"
        year: 年份（可选），如 2024。为空时返回最近一期
    """
    err = _require_yfinance()
    if err:
        return err

    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err2 = _check_info(info, symbol)
        if err2:
            return err2

        company_name = info.get("longName", info.get("shortName", symbol))
        lines = [
            f"## 📋 {company_name} ({symbol.upper()}) 财报摘要",
            f"",
        ]

        # --- Income Statement ---
        try:
            financials = stock.financials
            if financials is not None and not financials.empty:
                # Transpose so columns become dates
                fs = financials.copy()
                cols = list(fs.columns)
                # Optionally filter by year
                if year is not None:
                    year_str = str(year)
                    cols = [c for c in cols if year_str in str(c)]
                    if not cols:
                        return (
                            f"未找到 {symbol} 在 {year} 年的财报数据。"
                            f"可用年份: {', '.join(str(c.date()) if hasattr(c, 'date') else str(c) for c in list(fs.columns)[:5])}"
                        )
                # Take up to 4 most recent periods
                display_cols = cols[:4]

                lines.append("### 利润表")
                lines.append("")
                lines.append(f"| 科目 | {' | '.join(str(c.date()) if hasattr(c, 'date') else str(c)[:10] for c in display_cols)} |")
                lines.append(f"|------|{'|'.join('---' for _ in display_cols)}|")

                # Key rows
                key_rows = [
                    "Total Revenue",
                    "Operating Revenue",
                    "Gross Profit",
                    "Operating Income",
                    "Net Income",
                    "EBITDA",
                ]
                for row_name in key_rows:
                    if row_name in fs.index:
                        vals = []
                        for c in display_cols:
                            v = fs.loc[row_name, c]
                            if pd.notna(v):
                                vals.append(_fmt_big_num(v))
                            else:
                                vals.append("N/A")
                        lines.append(f"| **{row_name}** | {' | '.join(vals)} |")

                lines.append("")
        except Exception:
            pass

        # --- Balance Sheet Highlights ---
        try:
            bs = stock.balance_sheet
            if bs is not None and not bs.empty:
                lines.append("### 资产负债表亮点")
                lines.append("")
                bcols = list(bs.columns)[:2]

                for row_name in ["Total Assets", "Total Debt", "Total Liabilities Net Minority Interest",
                                 "Stockholders Equity", "Cash And Cash Equivalents"]:
                    if row_name in bs.index:
                        vals = []
                        for c in bcols:
                            v = bs.loc[row_name, c]
                            if pd.notna(v):
                                vals.append(_fmt_big_num(v))
                            else:
                                vals.append("N/A")
                        lines.append(f"- **{row_name}**: {' | '.join(vals)}")
                lines.append("")
        except Exception:
            pass

        # --- Cash Flow ---
        try:
            cf = stock.cashflow
            if cf is not None and not cf.empty:
                lines.append("### 现金流摘要")
                lines.append("")
                ccols = list(cf.columns)[:2]
                for row_name in ["Operating Cash Flow", "Free Cash Flow", "Capital Expenditure"]:
                    if row_name in cf.index:
                        vals = []
                        for c in ccols:
                            v = cf.loc[row_name, c]
                            if pd.notna(v):
                                vals.append(_fmt_big_num(v))
                            else:
                                vals.append("N/A")
                        lines.append(f"- **{row_name}**: {' | '.join(vals)}")
                lines.append("")
        except Exception:
            pass

        # --- Key Ratios from info ---
        lines.extend([
            "### 关键财务比率",
            "",
            "| 指标 | 值 (TTM) |",
            "|------|----------|",
            f"| **市盈率 (PE)** | {_safe(info, 'trailingPE', 'forwardPE')} |",
            f"| **市净率 (PB)** | {_safe(info, 'priceToBook')} |",
            f"| **企业价值/EBITDA** | {_safe(info, 'enterpriseToEbitda')} |",
            f"| **毛利率** | {_safe(info, 'grossMargins')} |",
            f"| **营业利润率** | {_safe(info, 'operatingMargins')} |",
            f"| **净利润率** | {_safe(info, 'profitMargins')} |",
            f"| **ROE** | {_safe(info, 'returnOnEquity')} |",
            f"| **ROA** | {_safe(info, 'returnOnAssets')} |",
            f"| **营收增长率** | {_safe(info, 'revenueGrowth')} |",
            f"| **净利润增长率** | {_safe(info, 'earningsGrowth')} |",
        ])

        lines.extend([
            "",
            "---",
            f"*数据更新时间: {_now_str()}*",
        ])

        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 财报摘要失败: {str(e)}"


def _fmt_big_num(val: float) -> str:
    """Format large number in human-readable form."""
    if pd.isna(val):
        return "N/A"
    if abs(val) >= 1e12:
        return f"${val / 1e12:,.2f}T"
    if abs(val) >= 1e9:
        return f"${val / 1e9:,.2f}B"
    if abs(val) >= 1e6:
        return f"${val / 1e6:,.2f}M"
    if abs(val) >= 1e3:
        return f"${val / 1e3:,.2f}K"
    return f"${val:,.2f}"


# ---------------------------------------------------------------------------
# Tool 6 — Search Company News
# ---------------------------------------------------------------------------

@mcp.tool()
def search_company_news(symbol: str, query: str = "", max_results: int = 5) -> str:
    """搜索特定公司的相关新闻

    Args:
        symbol: 股票代码，如 "AAPL"
        query: 搜索关键词（可选），用于进一步筛选
        max_results: 返回结果数量上限（默认 5）
    """
    if max_results < 1:
        return "max_results 必须大于 0"

    err = _require_yfinance()
    if err:
        return err

    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err2 = _check_info(info, symbol)
        if err2:
            return err2

        company_name = info.get("longName", info.get("shortName", symbol))
        news_items = []

        # Strategy 1 — yfinance news
        try:
            search = yf.Search(symbol)  # type: ignore[union-attr]
            raw = getattr(search, "news", [])
            if raw:
                for item in raw:
                    title = item.get("title", "")
                    if query and query.lower() not in title.lower():
                        continue
                    pub_date = _parse_date(item.get("providerPublishTime", ""))
                    link = item.get("link", "")
                    source = item.get("publisher", "")
                    news_items.append(
                        f"- **{title}**  [{source}]({link})  ({pub_date})"
                    )
                    if len(news_items) >= max_results:
                        break
        except Exception:
            pass

        # Strategy 2 — requests-based fallback
        if not news_items and REQUESTS_AVAILABLE:
            try:
                search_term = query if query else company_name
                resp = requests.get(
                    "https://newsapi.org/v2/everything",
                    params={
                        "q": f"{symbol} {search_term}",
                        "pageSize": max_results,
                        "sortBy": "publishedAt",
                    },
                    timeout=10,
                )
                if resp.status_code == 200:
                    articles = resp.json().get("articles", [])
                    for art in articles[:max_results]:
                        title = art.get("title", "")
                        source = art.get("source", {}).get("name", "")
                        pub_date = (art.get("publishedAt", "")[:10]
                                    if art.get("publishedAt") else "")
                        link = art.get("url", "")
                        news_items.append(
                            f"- **{title}**  ({source})  ({pub_date})"
                        )
            except Exception:
                pass

        if not news_items:
            return (
                f"未获取到 {symbol} ({company_name}) 的相关新闻"
            )

        header = f"### 📰 {symbol} ({company_name}) 相关新闻"
        if query:
            header += f" — 搜索: \"{query}\""

        return header + "\n\n" + "\n".join(news_items[:max_results])
    except Exception as e:
        return f"搜索 {symbol} 新闻失败: {str(e)}"


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    mcp.run(transport="stdio")
