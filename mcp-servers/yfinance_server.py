"""
yfinance MCP Server — stock market data via yfinance library.

Provides 7 tools for querying stock information through FastMCP.
Runs via stdio transport for go-stock's MCP server integration.
"""

import pandas as pd
import yfinance as yf
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("yfinance-server")

_PERIOD_OPTIONS = {"1d", "5d", "1mo", "3mo", "6mo", "1y", "2y", "5y", "10y", "max"}


def _get_ticker(symbol: str) -> yf.Ticker:
    """Create a validated yfinance Ticker object."""
    return yf.Ticker(symbol.strip().upper())


def _check_info(info: dict, symbol: str) -> str | None:
    """Return error message if info is unusable, else None."""
    if not info:
        return f"无法获取 {symbol} 的数据，请检查股票代码是否正确"
    # If regularMarketPrice is missing it's likely an invalid/untradeable symbol
    if info.get("regularMarketPrice") is None and info.get("currentPrice") is None:
        return f"无法获取 {symbol} 的数据，请检查股票代码是否正确"
    return None


# ---------------------------------------------------------------------------
# Helper: safely extract a nested dict value
# ---------------------------------------------------------------------------

def _safe(info: dict, *keys: str, default="N/A") -> str:
    """Traverse nested keys returning formatted value or default."""
    val: object = info
    for k in keys:
        if not isinstance(val, dict):
            return default
        val = val.get(k, {})  # type: ignore[arg-type]
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


# ---------------------------------------------------------------------------
# Tool 1 — Stock Info
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_info(symbol: str) -> str:
    """获取股票基本信息（名称、行业、市值等）"""
    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err = _check_info(info, symbol)
        if err:
            return err

        lines = [
            f"## {info.get('longName', info.get('shortName', symbol))}",
            f"",
            f"| 字段 | 值 |",
            f"|------|-----|",
            f"| **股票代码** | {symbol.upper()} |",
            f"| **交易所** | {_safe(info, 'exchange')} |",
            f"| **公司名称** | {_safe(info, 'longName')} |",
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
            f"| **网站** | {_safe(info, 'website')} |",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 基本信息失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 2 — Stock Price
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_price(symbol: str) -> str:
    """获取股票最新价格数据"""
    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err = _check_info(info, symbol)
        if err:
            return err

        price = info.get("currentPrice") or info.get("regularMarketPrice")
        prev_close = info.get("previousClose") or info.get("regularMarketPreviousClose")
        change = (price - prev_close) if (price and prev_close) else None
        pct = (change / prev_close * 100) if (change is not None and prev_close) else None
        vol = info.get("volume") or info.get("regularMarketVolume")

        lines = [
            f"## {symbol.upper()} 实时价格",
            f"",
            f"| 指标 | 值 |",
            f"|------|-----|",
            f"| **当前价格** | ${price:,.2f}" if price else "| **当前价格** | N/A",
            f"| **涨跌额** | {change:+,.2f}" if change is not None else "| **涨跌额** | N/A",
            f"| **涨跌幅** | {pct:+.2f}%" if pct is not None else "| **涨跌幅** | N/A",
            f"| **昨收** | ${prev_close:,.2f}" if prev_close else "| **昨收** | N/A",
            f"| **开盘** | ${_safe(info, 'regularMarketOpen')}" if info.get("regularMarketOpen") else "| **开盘** | N/A",
            f"| **当日最高** | ${_safe(info, 'regularMarketDayHigh')}" if info.get("regularMarketDayHigh") else "| **当日最高** | N/A",
            f"| **当日最低** | ${_safe(info, 'regularMarketDayLow')}" if info.get("regularMarketDayLow") else "| **当日最低** | N/A",
            f"| **成交量** | {int(vol):,}" if vol else "| **成交量** | N/A",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 价格数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 3 — Stock History
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_history(symbol: str, period: str = "1mo") -> str:
    """获取股票历史价格数据"""
    if period not in _PERIOD_OPTIONS:
        return f"无效的时间范围: {period}。可选: {', '.join(sorted(_PERIOD_OPTIONS))}"

    try:
        stock = _get_ticker(symbol)
        hist = stock.history(period=period)
        if hist.empty:
            return f"未获取到 {symbol} 在 {period} 时间范围内的历史数据"

        # Keep relevant columns and limit display
        display = hist[["Open", "High", "Low", "Close", "Volume"]].copy()
        display.index = display.index.strftime("%Y-%m-%d")
        display["Volume"] = display["Volume"].apply(lambda v: f"{int(v):,}")

        lines = [
            f"## {symbol.upper()} 历史价格（{period}）",
            f"",
            f"```",
            display.head(20).to_string(),
            f"```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 历史数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 4 — Dividends
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_dividends(symbol: str, limit: int = 10) -> str:
    """获取股票历史分红数据"""
    if limit < 1:
        return "limit 必须大于 0"

    try:
        stock = _get_ticker(symbol)
        div = stock.dividends
        if div.empty:
            return f"{symbol.upper()} 无分红数据或不分红"

        div = div.tail(limit)
        df = div.reset_index()
        df.columns = ["日期", "分红金额"]
        df["日期"] = df["日期"].dt.strftime("%Y-%m-%d")
        df["分红金额"] = df["分红金额"].apply(lambda x: f"${x:.4f}")

        lines = [
            f"## {symbol.upper()} 分红记录（最近 {limit} 次）",
            f"",
            f"```",
            df.to_string(index=False),
            f"```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 分红数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 5 — Financials
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_financials(symbol: str) -> str:
    """获取股票关键财务数据（利润表摘要）"""
    try:
        stock = _get_ticker(symbol)
        info = stock.info
        err = _check_info(info, symbol)
        if err:
            return err

        lines = [
            f"## {symbol.upper()} 财务摘要",
            f"",
            f"| 指标 | 值 (TTM) |",
            f"|------|----------|",
            f"| **总收入** | {_safe(info, 'totalRevenue', default='N/A')} |",
            f"| **净利润** | {_safe(info, 'netIncomeToCommon', 'netIncome', default='N/A')} |",
            f"| **基本每股收益** | {_safe(info, 'trailingEps', 'forwardEps')} |",
            f"| **每股净资产** | {_safe(info, 'bookValue')} |",
            f"| **毛利率** | {_safe(info, 'grossMargins')} |",
            f"| **营业利润率** | {_safe(info, 'operatingMargins')} |",
            f"| **ROE** | {_safe(info, 'returnOnEquity')} |",
            f"| **ROA** | {_safe(info, 'returnOnAssets')} |",
            f"| **营收增长率** | {_safe(info, 'revenueGrowth')} |",
            f"| **净利润增长率** | {_safe(info, 'earningsGrowth')} |",
            f"| **负债总额** | {_safe(info, 'totalDebt')} |",
            f"| **现金及等价物** | {_safe(info, 'totalCash')} |",
            f"| **自由现金流** | {_safe(info, 'freeCashflow')} |",
            f"| **市盈率 (PE)** | {_safe(info, 'trailingPE', 'forwardPE')} |",
            f"| **市净率 (PB)** | {_safe(info, 'priceToBook')} |",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 财务数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 6 — Analyst Recommendations
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_analyst_recommendations(symbol: str) -> str:
    """获取股票分析师评级"""
    try:
        stock = _get_ticker(symbol)
        recs = stock.recommendations
        if recs is None or recs.empty:
            return f"{symbol.upper()} 无分析师评级数据"

        recs = recs.tail(10)
        df = recs.reset_index()
        if "Firm" in df.columns:
            df = df[["Date", "Firm", "To Grade"]]
        elif "Action" in df.columns:
            df = df[["Date", "Action"]]
        else:
            df = recs.tail(10).reset_index()

        # Format Date column
        if "Date" in df.columns:
            df["Date"] = pd.to_datetime(df["Date"]).dt.strftime("%Y-%m-%d")

        lines = [
            f"## {symbol.upper()} 分析师评级（最近 10 条）",
            f"",
            f"```",
            df.to_string(index=False),
            f"```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 分析师评级失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 7 — Major Holders
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_major_holders(symbol: str) -> str:
    """获取股票主要机构持有者"""
    try:
        stock = _get_ticker(symbol)
        holders = stock.major_holders
        inst = stock.institutional_holders

        lines = [
            f"## {symbol.upper()} 主要持有人",
            f"",
        ]

        if holders is not None and not holders.empty:
            lines.append("### 持有比例概览")
            lines.append("")
            lines.append("```")
            lines.append(holders.to_string(index=False))
            lines.append("```")

        if inst is not None and not inst.empty:
            top = inst.head(10)
            lines.append("")
            lines.append(f"### 机构持有人（前 {len(top)} 名）")
            lines.append("")
            lines.append("```")
            lines.append(top.to_string(index=False))
            lines.append("```")

        if not lines[2:]:  # Only header lines present — no data
            return f"{symbol.upper()} 无持有人数据"

        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 持有人数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    mcp.run(transport="stdio")
