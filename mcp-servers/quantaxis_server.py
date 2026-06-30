"""
QUANTAXIS MCP Server — Chinese A-stock market data via akshare backend.

Provides 5 tools for querying A-stock information through FastMCP.
Runs via stdio transport for go-stock's MCP server integration.
"""

import pandas as pd
from mcp.server.fastmcp import FastMCP

# ---------------------------------------------------------------------------
# Library availability check — prefer akshare (well-maintained, no DB setup)
# ---------------------------------------------------------------------------

try:
    import akshare as ak

    AKSHARE_AVAILABLE = True
except ImportError:
    AKSHARE_AVAILABLE = False

mcp = FastMCP("quantaxis-server")

# ---------------------------------------------------------------------------
# Helper: check library availability
# ---------------------------------------------------------------------------

_NA_MSG = "QUANTAXIS 库未安装，请执行 pip install akshare 后重试"


def _require_akshare() -> str | None:
    """Return error string if akshare is unavailable, else None."""
    if not AKSHARE_AVAILABLE:
        return _NA_MSG
    return None


# ---------------------------------------------------------------------------
# Helper: pad A-stock code to 6 digits with optional prefix
# ---------------------------------------------------------------------------

def _normalize_code(code: str) -> str:
    """Normalize stock code to 6-digit string. Handles prefixes like SH/600000."""
    raw = code.strip()
    # If already 6 digits, return as-is
    if raw.isdigit() and len(raw) == 6:
        return raw
    # Extract digits from prefixed code (e.g. SH600000, sz000001)
    digits = "".join(ch for ch in raw if ch.isdigit())
    if len(digits) >= 6:
        return digits[:6]
    return digits.zfill(6)


# ---------------------------------------------------------------------------
# Tool 1 — Stock List
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_list() -> str:
    """获取全部A股股票代码和名称列表"""
    err = _require_akshare()
    if err:
        return err

    try:
        df = ak.stock_zh_a_spot_em()
        if df is None or df.empty:
            return "未获取到A股股票列表"

        # Keep relevant columns
        cols = ["代码", "名称"]
        available = [c for c in cols if c in df.columns]
        display = df[available].head(50).copy()
        display.index = range(1, len(display) + 1)

        lines = [
            f"## A股股票列表（共 {len(df):,} 只，显示前 50 只）",
            "",
            "```",
            display.to_string(),
            "```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取A股股票列表失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 2 — K-line Data
# ---------------------------------------------------------------------------

_FREQ_MAP = {
    "day": "daily",
    "week": "weekly",
    "month": "monthly",
}
_MIN_FREQS = {"min1", "min5", "min15", "min30", "min60"}


@mcp.tool()
def get_kline_data(code: str, start: str, end: str, freq: str = "day") -> str:
    """获取股票K线（蜡烛图）数据

    Args:
        code: 股票代码，如 "000001"
        start: 开始日期 "2025-01-01"
        end: 结束日期 "2025-12-31"
        freq: 频率，可选 day/week/month/min1/min5/min15/min30/min60，默认 day
    """
    err = _require_akshare()
    if err:
        return err

    symbol = _normalize_code(code)
    freq_lower = freq.lower().strip()

    try:
        if freq_lower in _FREQ_MAP:
            period = _FREQ_MAP[freq_lower]
            df = ak.stock_zh_a_hist(
                symbol=symbol,
                period=period,
                start_date=start,
                end_date=end,
                adjust="qfq",  # 前复权
            )
        elif freq_lower in _MIN_FREQS:
            df = ak.stock_zh_a_hist_min_em(
                symbol=symbol,
                period=freq_lower,
                start_date=start,
                end_date=end,
            )
            # Reset index so date/time become a column
            df = df.reset_index(drop=False)
        else:
            return f"无效的频率: {freq}。可选: day, week, month, min1, min5, min15, min30, min60"

        if df is None or df.empty:
            return f"未获取到 {symbol} 的K线数据（{freq}）"

        df = df.head(30)
        lines = [
            f"## {symbol} K线数据（{freq}）",
            "",
            "```",
            df.to_string(index=False),
            "```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} K线数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 3 — Stock Realtime
# ---------------------------------------------------------------------------

@mcp.tool()
def get_stock_realtime(code: str) -> str:
    """获取A股实时行情数据

    Args:
        code: 股票代码，如 "000001"
    """
    err = _require_akshare()
    if err:
        return err

    symbol = _normalize_code(code)

    try:
        df = ak.stock_zh_a_spot_em()
        if df is None or df.empty:
            return "未获取到实时行情数据"

        # Filter by code — column name may vary
        code_col = "代码" if "代码" in df.columns else df.columns[0]
        match = df[df[code_col].astype(str).str.contains(symbol)]
        if match.empty:
            return f"未找到 {symbol} 的实时行情数据，请检查股票代码"

        row = match.iloc[0]
        # Build a clean key-value output from known columns
        field_map = {
            "代码": "股票代码",
            "名称": "股票名称",
            "最新价": "最新价",
            "涨跌幅": "涨跌幅",
            "涨跌额": "涨跌额",
            "成交量": "成交量",
            "成交额": "成交额",
            "振幅": "振幅",
            "最高": "最高价",
            "最低": "最低价",
            "今开": "今开",
            "昨收": "昨收",
            "换手率": "换手率",
            "市盈率-动态": "市盈率(动态)",
            "市净率": "市净率",
            "总市值": "总市值",
            "流通市值": "流通市值",
            "量比": "量比",
            "60日涨跌幅": "60日涨跌幅",
        }

        pairs = []
        for eng, cname in field_map.items():
            if eng in row:
                val = row[eng]
                if pd.notna(val):
                    # Format numeric values
                    if isinstance(val, (int, float)):
                        if abs(val) >= 1e8:
                            val = f"{val / 1e8:.2f}亿"
                        elif abs(val) >= 1e4:
                            val = f"{val / 1e4:.2f}万"
                        elif isinstance(val, float):
                            val = f"{val:.2f}"
                    pairs.append(f"| **{cname}** | {val} |")

        lines = [
            f"## {row.get('名称', symbol)} 实时行情",
            "",
            "| 指标 | 值 |",
            "|------|-----|",
        ] + pairs
        return "\n".join(lines)
    except Exception as e:
        return f"获取 {symbol} 实时行情失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 4 — Market Index
# ---------------------------------------------------------------------------

_MAJOR_INDICES = {
    "sh": "上证指数",
    "sz": "深证成指",
    "cyb": "创业板指",
    "hs300": "沪深300",
    "zz500": "中证500",
    "kc50": "科创50",
}


@mcp.tool()
def get_market_index(code: str = "") -> str:
    """获取A股主要市场指数数据

    Args:
        code: 指数代码或名称关键字（可选）。为空时返回主要指数概览
    """
    err = _require_akshare()
    if err:
        return err

    try:
        df = ak.stock_zh_index_spot_em()
        if df is None or df.empty:
            return "未获取到指数数据"

        # Determine columns
        code_col = "代码" if "代码" in df.columns else df.columns[0]
        name_col = "名称" if "名称" in df.columns else df.columns[1]

        raw_code = code.strip()

        if raw_code:
            # Filter by code or name match
            mask = (
                df[code_col].astype(str).str.contains(raw_code)
                | df[name_col].astype(str).str.contains(raw_code)
            )
            filtered = df[mask]
            if filtered.empty:
                known = ", ".join(f"{k}({v})" for k, v in _MAJOR_INDICES.items())
                return (
                    f"未找到匹配 '{raw_code}' 的指数。\n"
                    f"常见指数关键词: {known}\n"
                    f"或直接输入指数代码部分数字进行搜索"
                )
            display = filtered.head(10)
        else:
            # Show major indices
            matched = df[df[name_col].isin(_MAJOR_INDICES.values())]
            if not matched.empty:
                display = matched
            else:
                display = df.head(10)

        cols = ["代码", "名称", "最新价", "涨跌幅", "涨跌额", "成交量", "成交额"]
        available = [c for c in cols if c in display.columns]
        result = display[available].head(20).copy()
        result.index = range(1, len(result) + 1)

        lines = [
            "## 市场指数",
            "",
            "```",
            result.to_string(),
            "```",
        ]
        return "\n".join(lines)
    except Exception as e:
        return f"获取市场指数失败: {str(e)}"


# ---------------------------------------------------------------------------
# Tool 5 — Financial Data
# ---------------------------------------------------------------------------

@mcp.tool()
def get_financial_data(code: str, report_date: str = "") -> str:
    """获取A股财务报告摘要数据

    Args:
        code: 股票代码，如 "000001"
        report_date: 报告期（可选），如 "2024-12-31"。为空时返回最新一期
    """
    err = _require_akshare()
    if err:
        return err

    symbol = _normalize_code(code)

    try:
        # Try financial abstract first
        df = ak.stock_financial_abstract(symbol=symbol)
        if df is None or df.empty:
            return f"未获取到 {symbol} 的财务数据"

        # Filter by report date if provided
        if report_date:
            date_cols = [c for c in df.columns if "期" in c or "日期" in c or "date" in c.lower()]
            if date_cols:
                date_col = date_cols[0]
                mask = df[date_col].astype(str).str.contains(report_date[:7])  # match YYYY-MM
                filtered = df[mask]
                if not filtered.empty:
                    df = filtered
                else:
                    # Try year-only match
                    mask = df[date_col].astype(str).str.contains(report_date[:4])
                    filtered = df[mask]
                    if not filtered.empty:
                        df = filtered

        df = df.head(10)
        # Transpose for easier reading when few rows
        if len(df) <= 5:
            lines = [
                f"## {symbol} 财务摘要",
                "",
            ]
            for _, row in df.iterrows():
                title = str(row.iloc[0]) if len(row) > 0 else ""
                lines.append(f"### {title}" if title else "---")
                lines.append("")
                items = []
                for col_idx in range(1, min(len(row), 20)):
                    col_name = df.columns[col_idx]
                    val = row.iloc[col_idx]
                    if pd.notna(val):
                        if isinstance(val, (int, float)):
                            if abs(val) >= 1e8:
                                val = f"{val / 1e8:.2f}亿"
                            elif abs(val) >= 1e4:
                                val = f"{val / 1e4:.2f}万"
                            else:
                                val = f"{val:.2f}" if isinstance(val, float) else str(val)
                        items.append(f"| **{col_name}** | {val} |")
                if items:
                    lines.append("| 指标 | 值 |")
                    lines.append("|------|-----|")
                    lines.extend(items)
                    lines.append("")
            return "\n".join(lines) if len(lines) > 2 else f"未获取到 {symbol} 的财务数据"

        # Fallback: show table
        lines = [
            f"## {symbol} 财务摘要",
            "",
            "```",
            df.to_string(index=False),
            "```",
        ]
        return "\n".join(lines)
    except ImportError:
        return "akshare 未安装，无法获取财务数据。请执行 pip install akshare"
    except AttributeError:
        # stock_financial_abstract may not exist in all akshare versions
        return f"当前 akshare 版本不支持通用财务摘要接口，请更新 akshare: pip install --upgrade akshare"
    except Exception as e:
        return f"获取 {symbol} 财务数据失败: {str(e)}"


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    mcp.run(transport="stdio")
