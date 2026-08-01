# 生成黄金对照数据：需先把 free-stockdb 仓库的 pybao 目录加入 PYTHONPATH，
# 且 stockdb.exe 已启动并有数据。输出 golden_600633.json 到本目录。
import json, os
from stock_sdk import StockDBClient

c = StockDBClient()
out = {}
for fq in ("qfq", "hfq", None):
    out[f"day_{fq}"] = c.get_data("600633", start="20250101", end="20260701", fq=fq)
out["week_qfq"] = c.get_data("600633", start="20250101", end="20260701", frequency="1w", fq="qfq")
out["30m_qfq"] = c.get_data("600633", start="20260625", end="20260626", frequency="30m", fq="qfq")
with open(os.path.join(os.path.dirname(__file__), "golden_600633.json"), "w", encoding="utf-8") as fp:
    json.dump(out, fp, ensure_ascii=False)
