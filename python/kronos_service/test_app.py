# Kronos 推理服务接口测试：不依赖真实模型（模型缺失时自动 skip 核心断言）。
# 运行：python -m pytest test_app.py -v
import importlib.util
import os
import sys

from fastapi.testclient import TestClient

sys.path.insert(0, os.path.dirname(__file__))
spec = importlib.util.spec_from_file_location("app", os.path.join(os.path.dirname(__file__), "app.py"))
app_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(app_module)

client = TestClient(app_module.app)


def _make_candles(n=60, base=10.0):
    candles = []
    price = base
    for i in range(n):
        o = price
        c = price * (1 + (0.01 if i % 3 else -0.008))
        h = max(o, c) * 1.01
        low = min(o, c) * 0.99
        candles.append(
            {
                "date": f"2025-01-{(i % 28) + 1:02d}",
                "open": round(o, 2),
                "high": round(h, 2),
                "low": round(low, 2),
                "close": round(c, 2),
                "volume": 100000 + i,
                "amount": 0.0,
            }
        )
        price = c
    return candles


def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"


def test_predict_validation():
    # K线数量不足 30 → 422
    r = client.post("/predict", json={"candles": _make_candles(10)})
    assert r.status_code == 422


def test_predict_with_model():
    try:
        import torch  # noqa: F401
        from model import Kronos  # noqa: F401
    except Exception:
        print("Kronos 环境未安装，跳过真实预测测试")
        return

    r = client.post(
        "/predict",
        json={"candles": _make_candles(60), "predLen": 5, "sampleCount": 1, "device": "cpu"},
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert len(body["bars"]) == 5
    bar = body["bars"][0]
    for key in ("date", "open", "high", "low", "close"):
        assert key in bar
    summary = body["summary"]
    for key in ("lastClose", "predEnd", "predHigh", "predLow", "direction", "confidence"):
        assert key in summary
    assert summary["predHigh"] >= summary["predLow"] > 0
    assert summary["direction"] in ("up", "down")
