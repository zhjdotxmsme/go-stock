#!/usr/bin/env python3
"""Generate A-share historical daily K-line seed data using Baostock.

Downloads daily K-lines from 2010-01-01 to yesterday for all A-share stocks
and writes directly into go-stock's kline_bars SQLite table (Source='seed').

Usage:
    python baostock_seed.py
    python baostock_seed.py --db-path ~/.go-stock/data/go-stock.db
    python baostock_seed.py --limit 50 --dry-run
    python baostock_seed.py --start-date 20150101 --end-date 20241231
"""

import argparse
import logging
import os
import re
import sqlite3
import sys
import time
from datetime import datetime, timedelta
from pathlib import Path

import baostock as bs
from tqdm import tqdm

logger = logging.getLogger("baostock_seed")
KLINE_BATCH_SIZE = 10000
BAOSTOCK_RATE_LIMIT_SEC = 0.25  # 4 queries/sec, well under 300/min
DEFAULT_PERIOD = "day"
DEFAULT_SOURCE = "seed"
DEFAULT_ADJUSTED = True

def resolve_db_path(cli_path: str | None) -> str:
    if cli_path:
        return os.path.abspath(os.path.expanduser(cli_path))
    env_path = os.environ.get("DATA_PATH")
    if env_path:
        return os.path.abspath(os.path.expanduser(env_path))
    home = Path.home()
    candidates = [
        home / ".go-stock" / "data" / "go-stock.db",
        home / ".config" / "go-stock" / "data" / "go-stock.db",
    ]
    if sys.platform == "win32":
        appdata = Path(os.environ.get("APPDATA", ""))
        if appdata:
            candidates.insert(0, appdata / "go-stock" / "data" / "go-stock.db")
    elif sys.platform == "darwin":
        candidates.insert(0, home / "Library" / "Application Support" / "go-stock" / "data" / "go-stock.db")
    for candidate in candidates:
        if candidate.exists():
            return str(candidate.resolve())
    logger.warning("No existing go-stock DB found, using default path: %s", candidates[0])
    candidates[0].parent.mkdir(parents=True, exist_ok=True)
    return str(candidates[0].resolve())


def baostock_code(db_code: str) -> str:
    # db_code is "601916.SH" or "000800.SZ" → baostock wants "sh.601916"
    if "." in db_code:
        sym, exch = db_code.split(".", 1)
        return exch.lower() + "." + sym
    return db_code[:2] + "." + db_code[2:]


def go_stock_code(bs_code: str) -> str:
    # bs_code is "sh.601916" → go-stock 标准格式 "sh601916"
    # （与 backtest.NormalizeStockCode / kline_bars 其他来源一致）
    return bs_code.replace(".", "")


def count_existing_bars(conn: sqlite3.Connection) -> dict[str, int]:
    cursor = conn.execute(
        "SELECT stock_code, COUNT(*) FROM kline_bars WHERE source = 'seed' GROUP BY stock_code"
    )
    return {row[0]: row[1] for row in cursor.fetchall()}


def load_stock_codes(conn: sqlite3.Connection) -> list[dict]:
    cursor = conn.execute(
        "SELECT secucode, sec_uri_tycode, sec_uri_tynameabbr FROM all_stock_info "
        "WHERE (secucode LIKE '%.SH' OR secucode LIKE '%.SZ') "
        "ORDER BY secucode"
    )
    cols = [desc[0] for desc in cursor.description]
    return [dict(zip(cols, row)) for row in cursor.fetchall()]


def _fmt_date(d: str) -> str:
    d = d.replace("-", "")
    return f"{d[:4]}-{d[4:6]}-{d[6:8]}"


def download_kline(
    code: str, start_date: str, end_date: str
) -> list[tuple]:
    params = {
        "code": code,
        "fields": "date,code,open,high,low,close,preclose,volume,amount,pctChg",
        "start_date": _fmt_date(start_date),
        "end_date": _fmt_date(end_date),
        "frequency": "d",
        "adjustflag": "2",
    }
    rs = bs.query_history_k_data_plus(**params)
    if rs is None:
        raise RuntimeError(f"Baostock query {code} returned None (invalid params?)")
    if rs.error_code != "0":
        raise RuntimeError(f"Baostock query {code} failed: {rs.error_msg}")
    rows = []
    while rs.next():
        row = rs.get_row_data()
        if row[0] is None or row[0].strip() == "":
            continue
        rows.append(tuple(row))
    if len(rows) < KLINE_BATCH_SIZE:
        return rows
    more = len(rows)
    page_index = 2
    while more >= KLINE_BATCH_SIZE:
        rs = bs.query_history_k_data_plus(**{**params, "pageNumber": page_index})
        if rs.error_code != "0":
            raise RuntimeError(f"Baostock query {code} page {page_index} failed: {rs.error_msg}")
        page_rows = []
        while rs.next():
            row = rs.get_row_data()
            if row[0] is None or row[0].strip() == "":
                continue
            page_rows.append(tuple(row))
        if not page_rows:
            break
        rows.extend(page_rows)
        more = len(page_rows)
        page_index += 1
    return rows


def build_kline_rows(rows: list[tuple]) -> list[tuple]:
    now = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")
    result = []
    for r in rows:
        code = go_stock_code(r[1])
        try:
            result.append((
                code,
                DEFAULT_PERIOD,
                r[0],
                DEFAULT_ADJUSTED,
                float(r[2]),
                float(r[3]),
                float(r[4]),
                float(r[5]),
                int(round(float(r[7]))),
                float(r[8]),
                DEFAULT_SOURCE,
                now,
                now,
            ))
        except (ValueError, TypeError, IndexError) as e:
            logger.debug("Skipping row %s due to parse error: %s", r, e)
    return result


def batch_upsert_kline(conn: sqlite3.Connection, rows: list[tuple], dry_run: bool, batch_size: int = 500):
    cur = conn.cursor()
    for i in range(0, len(rows), batch_size):
        batch = rows[i : i + batch_size]
        if dry_run:
            continue
        cur.executemany(
            """INSERT OR IGNORE INTO kline_bars
               (stock_code, period, trade_date, adjusted,
                open, high, low, close, volume, amount,
                source, created_at, updated_at)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            batch,
        )
    conn.commit()


def run(
    db_path: str,
    start_date: str,
    end_date: str,
    limit: int | None = None,
    dry_run: bool = False,
    verbose: bool = False,
):
    logger.info("Connecting to SQLite: %s", db_path)
    conn = sqlite3.connect(db_path)
    conn.execute("PRAGMA journal_mode=WAL")
    conn.execute("PRAGMA synchronous=OFF")

    existing = count_existing_bars(conn)
    stocks = load_stock_codes(conn)
    logger.info("Loaded %d A-share stocks from all_stock_info", len(stocks))

    if limit and limit < len(stocks):
        stocks = stocks[:limit]
        logger.info("Limited to first %d stocks", limit)

    if dry_run:
        logger.info("DRY RUN mode — no data will be written to the database")

    skipped = 0
    completed = 0
    errors: list[str] = []

    for stock in tqdm(stocks, desc="Downloading", unit="stock"):
        db_code = stock["secucode"] or stock["sec_uri_tycode"]
        if not db_code:
            skipped += 1
            continue
        if db_code in existing and existing[db_code] > 0:
            logger.debug("Skipping %s — already has %d bars", db_code, existing[db_code])
            skipped += 1
            continue

        bs_code = baostock_code(db_code)
        try:
            raw = download_kline(bs_code, start_date, end_date)
            if not raw:
                logger.debug("No data returned for %s (%s)", db_code, bs_code)
                skipped += 1
                continue
            kline_rows = build_kline_rows(raw)
            if kline_rows:
                batch_upsert_kline(conn, kline_rows, dry_run)
                inserted = len(kline_rows)
                if not dry_run:
                    tqdm.write(f"  {db_code}: {inserted} rows ({raw[0][0]} ~ {raw[-1][0]})")
                completed += 1
            else:
                skipped += 1
        except Exception as e:
            errors.append(f"{db_code}: {e}")
            logger.error("Failed to download %s (%s): %s", db_code, bs_code, e)

        time.sleep(BAOSTOCK_RATE_LIMIT_SEC)

    conn.close()

    tqdm.write("")
    logger.info("=" * 50)
    logger.info("Summary:")
    logger.info("  Completed : %d", completed)
    logger.info("  Skipped   : %d", skipped)
    logger.info("  Errors    : %d", len(errors))
    if errors:
        logger.info("  Error details:")
        for err in errors[:10]:
            logger.info("    - %s", err)
        if len(errors) > 10:
            logger.info("    ... and %d more", len(errors) - 10)
    if dry_run:
        logger.info("  (dry-run — no data written)")
    logger.info("=" * 50)


def main():
    parser = argparse.ArgumentParser(
        description="Generate A-share daily K-line seed data into go-stock SQLite DB"
    )
    parser.add_argument(
        "--db-path",
        help="Path to go-stock SQLite database (default: auto-detect from DATA_PATH or ~/.go-stock)",
    )
    parser.add_argument(
        "--start-date",
        default="20100101",
        help="Start date YYYYMMDD (default: 20100101)",
    )
    parser.add_argument(
        "--end-date",
        default=(datetime.now() - timedelta(days=1)).strftime("%Y%m%d"),
        help="End date YYYYMMDD (default: yesterday)",
    )
    parser.add_argument(
        "--limit",
        type=int,
        help="Max number of stocks to process (for testing)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Connect to DB and enumerate stocks, but do not write any data",
    )
    parser.add_argument(
        "-v", "--verbose",
        action="store_true",
        help="Enable debug-level logging",
    )
    parser.add_argument(
        "-q", "--quiet",
        action="store_true",
        help="Suppress info logging, show only errors",
    )
    args = parser.parse_args()

    level = logging.DEBUG if args.verbose else (logging.ERROR if args.quiet else logging.INFO)
    logging.basicConfig(
        level=level,
        format="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%H:%M:%S",
    )

    db_path = resolve_db_path(args.db_path)
    logger.info("Database path: %s", db_path)
    logger.info(
        "Date range: %s ~ %s", args.start_date,
        args.end_date or (datetime.now() - timedelta(days=1)).strftime("%Y%m%d"),
    )

    if not os.path.isfile(db_path):
        logger.error("Database file not found: %s", db_path)
        logger.error("Please run go-stock first to initialize the database, then re-run this script.")
        sys.exit(1)

    lg = bs.login()
    if lg.error_code != "0":
        logger.error("Baostock login failed: %s", lg.error_msg)
        sys.exit(1)
    logger.info("Baostock login successful")

    try:
        run(
            db_path=db_path,
            start_date=args.start_date,
            end_date=args.end_date or (datetime.now() - timedelta(days=1)).strftime("%Y%m%d"),
            limit=args.limit,
            dry_run=args.dry_run,
            verbose=args.verbose,
        )
    finally:
        bs.logout()
        logger.info("Baostock logged out")


if __name__ == "__main__":
    main()
