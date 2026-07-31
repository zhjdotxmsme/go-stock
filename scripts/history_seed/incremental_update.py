#!/usr/bin/env python3
"""Incremental update A-share daily K-line data via Baostock.

Detects the latest trade_date per stock in kline_bars, then downloads
only missing dates from [last+1day, yesterday].  Idempotent — safe to
run daily via cron / Task Scheduler.

Usage:
    python incremental_update.py
    python incremental_update.py --db-path ~/.go-stock/data/go-stock.db
    python incremental_update.py --days-back 30
    python incremental_update.py --dry-run --limit 10
    python incremental_update.py --source seed
"""

import argparse
import logging
import os
import sqlite3
import sys
import time
from datetime import datetime, timedelta
from pathlib import Path

import baostock as bs
from tqdm import tqdm

logger = logging.getLogger("incremental_update")

BAOSTOCK_RATE_LIMIT_SEC = 0.25  # 4 queries/sec
BATCH_SIZE = 500
DEFAULT_PERIOD = "day"
DEFAULT_SOURCE = "seed"
DEFAULT_ADJUSTED = True


# ---------------------------------------------------------------------------
# DB path resolution (mirrors baostock_seed.py)
# ---------------------------------------------------------------------------

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


# ---------------------------------------------------------------------------
# Code conversion helpers
# ---------------------------------------------------------------------------

def baostock_code(db_code: str) -> str:
    """db_code '601916.SH' → baostock 'sh.601916'"""
    if "." in db_code:
        sym, exch = db_code.split(".", 1)
        return exch.lower() + "." + sym
    return db_code[:2] + "." + db_code[2:]


def go_stock_code(bs_code: str) -> str:
    """bs_code 'sh.601916' → go-stock '601916'"""
    if "." in bs_code:
        return bs_code.split(".")[1]
    return bs_code.replace(".", "")


# ---------------------------------------------------------------------------
# DB queries
# ---------------------------------------------------------------------------

def load_stock_codes(conn: sqlite3.Connection) -> list[dict]:
    cursor = conn.execute(
        "SELECT secucode, sec_uri_tycode, sec_uri_tynameabbr FROM all_stock_info "
        "WHERE (secucode LIKE '%.SH' OR secucode LIKE '%.SZ') "
        "ORDER BY secucode"
    )
    cols = [desc[0] for desc in cursor.description]
    return [dict(zip(cols, row)) for row in cursor.fetchall()]


def get_latest_trade_dates(
    conn: sqlite3.Connection, source: str,
) -> dict[str, str]:
    """Return {stock_code: latest_trade_date} for bars matching source."""
    cursor = conn.execute(
        "SELECT stock_code, MAX(trade_date) FROM kline_bars "
        "WHERE period = 'day' AND source = ? "
        "GROUP BY stock_code",
        (source,),
    )
    return {row[0]: row[1] for row in cursor.fetchall()}


# ---------------------------------------------------------------------------
# Baostock download
# ---------------------------------------------------------------------------

def _fmt_date(d: str) -> str:
    d = d.replace("-", "")
    return f"{d[:4]}-{d[4:6]}-{d[6:8]}"


def download_kline(code: str, start_date: str, end_date: str) -> list[tuple]:
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
        raise RuntimeError(f"Baostock query {code} returned None")
    if rs.error_code != "0":
        raise RuntimeError(f"Baostock query {code} failed: {rs.error_msg}")
    rows = []
    while rs.next():
        row = rs.get_row_data()
        if row[0] is None or row[0].strip() == "":
            continue
        rows.append(tuple(row))
    return rows


# ---------------------------------------------------------------------------
# Build & write K-line rows
# ---------------------------------------------------------------------------

def build_kline_rows(rows: list[tuple], source: str) -> list[tuple]:
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
                source,
                now,
                now,
            ))
        except (ValueError, TypeError, IndexError):
            pass
    return result


def batch_upsert_kline(conn: sqlite3.Connection, rows: list[tuple], dry_run: bool):
    if dry_run:
        return
    cur = conn.cursor()
    for i in range(0, len(rows), BATCH_SIZE):
        batch = rows[i : i + BATCH_SIZE]
        cur.executemany(
            """INSERT OR IGNORE INTO kline_bars
               (stock_code, period, trade_date, adjusted,
                open, high, low, close, volume, amount,
                source, created_at, updated_at)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            batch,
        )
    conn.commit()


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run(
    db_path: str,
    days_back: int,
    source: str,
    limit: int | None = None,
    dry_run: bool = False,
):
    end_date = (datetime.now() - timedelta(days=1)).strftime("%Y%m%d")
    start_date = (datetime.now() - timedelta(days=days_back)).strftime("%Y%m%d")

    logger.info("SQLite: %s", db_path)
    logger.info("Range: %s ~ %s (source=%s)", start_date, end_date, source)

    conn = sqlite3.connect(db_path)
    conn.execute("PRAGMA journal_mode=WAL")
    conn.execute("PRAGMA synchronous=OFF")

    stocks = load_stock_codes(conn)
    latest = get_latest_trade_dates(conn, source)
    logger.info("Loaded %d A-share stocks, %d have %s data", len(stocks), len(latest), source)

    if limit and limit < len(stocks):
        stocks = stocks[:limit]
        logger.info("Limited to first %d stocks", limit)

    if dry_run:
        logger.info("DRY RUN — no writes")

    skipped = 0
    completed = 0
    errors: list[str] = []

    for stock in tqdm(stocks, desc="Incremental", unit="stock"):
        db_code = stock["secucode"] or stock["sec_uri_tycode"]
        if not db_code:
            skipped += 1
            continue

        last_date = latest.get(db_code)
        if not last_date:
            logger.debug("Skipping %s — no seed data yet (run baostock_seed.py first)", db_code)
            skipped += 1
            continue

        # Normalize to YYYYMMDD so lexical comparison is valid
        # (trade_date is stored as YYYY-MM-DD, end_date as YYYYMMDD)
        last_date_norm = last_date.replace("-", "")

        # If last_date already covers the target range, skip
        if last_date_norm >= end_date:
            skipped += 1
            continue

        # Stocks stale longer than days_back are skipped rather than
        # partially filled, so no permanent gaps are left behind.
        # Rerun with a larger --days-back (or baostock_seed.py) to backfill.
        dt = datetime.strptime(last_date_norm, "%Y%m%d") + timedelta(days=1)
        fetch_start = dt.strftime("%Y%m%d")
        if fetch_start < start_date:
            logger.warning(
                "Skipping %s — stale since %s (beyond --days-back %d); "
                "rerun with --days-back %d+ to backfill",
                db_code, last_date, days_back,
                (datetime.now() - dt).days + 1,
            )
            skipped += 1
            continue

        bs_code = baostock_code(db_code)
        try:
            raw = download_kline(bs_code, fetch_start, end_date)
            if not raw:
                logger.debug("No new data for %s (%s)", db_code, bs_code)
                skipped += 1
                continue

            kline_rows = build_kline_rows(raw, source)
            if kline_rows:
                batch_upsert_kline(conn, kline_rows, dry_run)
                inserted = len(kline_rows)
                if not dry_run:
                    tqdm.write(f"  {db_code}: +{inserted} bars ({raw[0][0]} ~ {raw[-1][0]})")
                completed += 1
            else:
                skipped += 1
        except Exception as e:
            errors.append(f"{db_code}: {e}")
            logger.error("Failed %s (%s): %s", db_code, bs_code, e)

        time.sleep(BAOSTOCK_RATE_LIMIT_SEC)

    conn.close()

    tqdm.write("")
    logger.info("=" * 50)
    logger.info("Summary:")
    logger.info("  Updated  : %d", completed)
    logger.info("  Skipped  : %d", skipped)
    logger.info("  Errors   : %d", len(errors))
    if errors:
        logger.info("  Error details:")
        for err in errors[:10]:
            logger.info("    - %s", err)
        if len(errors) > 10:
            logger.info("    ... and %d more", len(errors) - 10)
    if dry_run:
        logger.info("  (dry-run)")
    logger.info("=" * 50)


def main():
    parser = argparse.ArgumentParser(
        description="Incremental update A-share daily K-line data via Baostock"
    )
    parser.add_argument("--db-path", help="Path to go-stock SQLite DB (default: auto-detect)")
    parser.add_argument(
        "--days-back", type=int, default=7,
        help="How many days back to scan for missing data (default: 7)",
    )
    parser.add_argument(
        "--source", default=DEFAULT_SOURCE,
        help="Source label in kline_bars (default: seed)",
    )
    parser.add_argument("--limit", type=int, help="Max stocks to process")
    parser.add_argument("--dry-run", action="store_true", help="Check only, no writes")
    parser.add_argument("-v", "--verbose", action="store_true", help="Debug logging")
    parser.add_argument("-q", "--quiet", action="store_true", help="Errors only")
    args = parser.parse_args()

    level = logging.DEBUG if args.verbose else (logging.ERROR if args.quiet else logging.INFO)
    logging.basicConfig(level=level, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")

    db_path = resolve_db_path(args.db_path)
    if not os.path.isfile(db_path):
        logger.error("DB not found: %s", db_path)
        logger.error("Run go-stock first to initialize the database.")
        sys.exit(1)

    lg = bs.login()
    if lg.error_code != "0":
        logger.error("Baostock login failed: %s", lg.error_msg)
        sys.exit(1)
    logger.info("Baostock login OK")

    try:
        run(
            db_path=db_path,
            days_back=args.days_back,
            source=args.source,
            limit=args.limit,
            dry_run=args.dry_run,
        )
    finally:
        bs.logout()
        logger.info("Baostock logged out")


if __name__ == "__main__":
    main()
