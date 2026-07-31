# A-Share Historical K-Line Seed & Incremental Update Scripts

## Overview

Two scripts for populating go-stock's `kline_bars` table with A-share daily K-line data via [Baostock](http://baostock.com/):

| Script | Purpose | When to run |
|--------|---------|-------------|
| `baostock_seed.py` | Full initial seed (2010~yesterday) | Once, first setup |
| `incremental_update.py` | Daily incremental update | Every trading day after close |

## Requirements

```bash
pip install baostock tqdm
```

## Usage

### 1. Full seed (first time only)

```bash
python scripts/history_seed/baostock_seed.py
```

### 2. Daily incremental update

```bash
# Default: scan last 7 days, auto-detect DB
python scripts/history_seed/incremental_update.py

# Specify DB path
python scripts/history_seed/incremental_update.py --db-path ~/.go-stock/data/go-stock.db

# Dry run — check what would be updated
python scripts/history_seed/incremental_update.py --dry-run --limit 10

# Scan last 30 days (e.g. after a long break)
python scripts/history_seed/incremental_update.py --days-back 30

# Quiet mode — errors only
python scripts/history_seed/incremental_update.py -q
```

### 3. Schedule daily (Windows Task Scheduler)

```xml
Trigger: Daily at 18:00 (after market close)
Action: python E:\path\to\scripts\history_seed\incremental_update.py -q
```

## Options

### baostock_seed.py

| Flag | Default | Description |
|------|---------|-------------|
| `--db-path` | auto-detect | Path to go-stock SQLite database |
| `--start-date` | 20100101 | Start date (YYYYMMDD) |
| `--end-date` | yesterday | End date (YYYYMMDD) |
| `--limit` | all | Max stocks to process |
| `--dry-run` | false | Check only, no writes |
| `-v` | false | Debug logging |
| `-q` | false | Errors only |

### incremental_update.py

| Flag | Default | Description |
|------|---------|-------------|
| `--db-path` | auto-detect | Path to go-stock SQLite database |
| `--days-back` | 7 | How many days back to scan for missing data (stocks stale longer than this are skipped; rerun with a larger value to backfill long gaps) |
| `--source` | seed | Source label in kline_bars |
| `--limit` | all | Max stocks to process |
| `--dry-run` | false | Check only, no writes |
| `-v` | false | Debug logging |
| `-q` | false | Errors only |

## Database Path Discovery

Priority: `--db-path` CLI arg > `$DATA_PATH` env var > platform defaults:

- **Linux**: `~/.go-stock/data/go-stock.db` or `~/.config/go-stock/data/go-stock.db`
- **Windows**: `%APPDATA%/go-stock/data/go-stock.db`
- **macOS**: `~/Library/Application Support/go-stock/data/go-stock.db`

## Data Flow

```
Baostock API → batch_upsert_kline (INSERT OR IGNORE) → kline_bars table
                   ↓
            Source='seed' set for all records
                   ↓
         Backtest engine reads from local cache
```

## How Incremental Update Works

1. Query `MAX(trade_date)` per stock from `kline_bars WHERE source='seed'`
2. For each stock, if `last_date < yesterday`, download `[last_date+1, yesterday]`
3. `INSERT OR IGNORE` — idempotent, safe to re-run
4. Skips stocks with no seed data (run `baostock_seed.py` first)

## Notes

- **Rate limiting**: 250ms between queries (240/min), under Baostock's 300/min limit
- **Run go-stock first** to initialize the database before running any script
- **Incremental script requires seed data to exist** — run `baostock_seed.py` first
