# A-Share Historical K-Line Seed Script

## Overview

`baostock_seed.py` initializes go-stock's local SQLite database with A-share historical daily K-line data using [Baostock](http://baostock.com/), a free Chinese stock data API.

Data is written directly into the `kline_bars` table (`Source='seed'`), enabling the backtest engine to query locally without hitting rate-limited external APIs.

## Requirements

```bash
pip install baostock tqdm
```

## Usage

```bash
# Basic — auto-detect database path
python scripts/history_seed/baostock_seed.py

# Specify database location
python scripts/history_seed/baostock_seed.py --db-path ~/.go-stock/data/go-stock.db

# Dry run — check connectivity without writing
python scripts/history_seed/baostock_seed.py --dry-run --limit 5

# Custom date range
python scripts/history_seed/baostock_seed.py --start-date 20150101 --end-date 20241231

# Process only first 50 stocks (for testing)
python scripts/history_seed/baostock_seed.py --limit 50

# Quiet mode — show errors only
python scripts/history_seed/baostock_seed.py -q
```

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `--db-path` | auto-detect | Path to go-stock SQLite database |
| `--start-date` | 20100101 | Start date (YYYYMMDD) |
| `--end-date` | yesterday | End date (YYYYMMDD) |
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

## Notes

- **Incremental**: Skips stocks already having seed data in `kline_bars`
- **Rate limiting**: 250ms between queries (240/min), under Baostock's 300/min limit
- **One-time run**: Designed for initial data seeding; daily incremental updates use Go data sources (mootdx/Tencent)
- **Run go-stock first** to initialize the database before running this script
