# Kronos K线预测推理服务（go-stock 可选组件）
# 基于清华开源 Kronos 模型：https://github.com/shiyu-coder/Kronos
# 由 go-stock 进程托管拉起，仅监听 127.0.0.1。独立手动运行：python app.py --port 8765
import argparse
import threading
from contextlib import asynccontextmanager

import pandas as pd
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

# 全局模型句柄（懒加载，首次 /predict 时初始化）
_model_lock = threading.Lock()
_predictor = None
_predictor_key = None  # "model_name|device" 变化时重建


def _get_predictor(model_name: str, device: str):
    """懒加载 KronosPredictor；同配置复用，配置变化时重建。"""
    global _predictor, _predictor_key
    key = f"{model_name}|{device}"
    if _predictor is not None and _predictor_key == key:
        return _predictor
    with _model_lock:
        if _predictor is not None and _predictor_key == key:
            return _predictor
        from model import Kronos, KronosTokenizer, KronosPredictor  # 官方实现随仓库附带

        tokenizer_name = "NeoQuasar/Kronos-Tokenizer-base"
        tok = KronosTokenizer.from_pretrained(tokenizer_name)
        model = Kronos.from_pretrained(model_name)
        actual_device = device
        if actual_device == "auto":
            import torch

            actual_device = "cuda:0" if torch.cuda.is_available() else "cpu"
        _predictor = KronosPredictor(model, tok, device=actual_device, max_context=512)
        _predictor_key = key
    return _predictor


class Candle(BaseModel):
    date: str
    open: float
    high: float
    low: float
    close: float
    volume: float = 0.0
    amount: float = 0.0


class PredictRequest(BaseModel):
    # 历史日K（升序，>=30 根，<=512 根）
    candles: list[Candle] = Field(min_length=30, max_length=512)
    predLen: int = Field(default=5, ge=1, le=120)
    T: float = Field(default=1.0, ge=0.1, le=2.0)
    topP: float = Field(default=0.9, gt=0.0, le=1.0)
    sampleCount: int = Field(default=4, ge=1, le=16)
    model: str = "NeoQuasar/Kronos-small"
    device: str = "auto"
    # 历史切点（回测模式）：只使用该日期（含）之前的K线做预测
    asOfDate: str | None = None


class PredictResponse(BaseModel):
    bars: list[Candle]
    summary: dict


class BatchItem(BaseModel):
    code: str
    candles: list[Candle] = Field(min_length=30, max_length=512)


class BatchPredictRequest(BaseModel):
    items: list[BatchItem] = Field(min_length=1, max_length=100)
    predLen: int = Field(default=5, ge=1, le=120)
    T: float = Field(default=1.0, ge=0.1, le=2.0)
    topP: float = Field(default=0.9, gt=0.0, le=1.0)
    sampleCount: int = Field(default=4, ge=1, le=16)
    model: str = "NeoQuasar/Kronos-small"
    device: str = "auto"


class BatchPredictResponse(BaseModel):
    results: dict[str, PredictResponse | None]  # code -> 预测（失败为 None）
    errors: dict[str, str]  # code -> 失败原因


def _to_dataframe(candles: list[Candle]) -> pd.DataFrame:
    df = pd.DataFrame([c.model_dump() for c in candles])
    df["timestamps"] = pd.to_datetime(df["date"])
    df = df.set_index("timestamps")
    return df[["open", "high", "low", "close", "volume", "amount"]]


def _next_trade_dates(last_date: pd.Timestamp, pred_len: int) -> pd.DatetimeIndex:
    """生成未来交易日时间戳（仅跳过周末；节假日由前端展示层面弱化影响）。"""
    return pd.bdate_range(last_date + pd.Timedelta(days=1), periods=pred_len)


def _run_predict(req: PredictRequest) -> PredictResponse:
    try:
        predictor = _get_predictor(req.model, req.device)
    except Exception as e:  # 模型下载失败/环境缺失
        raise HTTPException(status_code=503, detail=f"MODEL_LOAD_FAILED: {e}") from e

    df = _to_dataframe(req.candles)
    # 历史切点：截断到 asOfDate（含）之前
    if req.asOfDate:
        cutoff = pd.to_datetime(req.asOfDate)
        df = df[df.index <= cutoff]
        if len(df) < 30:
            raise HTTPException(status_code=422, detail="INSUFFICIENT_DATA_AT_CUTOFF")
    last_date = df.index[-1]
    y_timestamp = _next_trade_dates(last_date, req.predLen)

    try:
        pred = predictor.predict(
            df=df,
            x_timestamp=pd.Series(df.index),
            y_timestamp=pd.Series(y_timestamp),
            pred_len=req.predLen,
            T=req.T,
            top_p=req.topP,
            sample_count=req.sampleCount,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"PREDICT_FAILED: {e}") from e

    bars = []
    for ts, row in pred.iterrows():
        bars.append(
            Candle(
                date=ts.strftime("%Y-%m-%d"),
                open=round(float(row["open"]), 3),
                high=round(float(row["high"]), 3),
                low=round(float(row["low"]), 3),
                close=round(float(row["close"]), 3),
                volume=float(row.get("volume", 0) or 0),
                amount=float(row.get("amount", 0) or 0),
            )
        )

    closes = [b.close for b in bars]
    highs = [b.high for b in bars]
    lows = [b.low for b in bars]
    last_close = float(df["close"].iloc[-1])  # 切点截断后的最后一根收盘
    pred_end = closes[-1]
    direction = "up" if pred_end >= last_close else "down"
    change_pct = (pred_end - last_close) / last_close * 100 if last_close > 0 else 0.0
    # 置信度：多次采样离散度换算（std/mean 越小越可信），映射到 0~100
    import statistics

    if len(closes) > 1 and statistics.mean(closes) > 0:
        cv = statistics.pstdev(closes) / statistics.mean(closes)
    else:
        cv = 0.0
    confidence = max(0.0, min(100.0, 100.0 * (1.0 - min(cv * 10.0, 1.0))))

    return PredictResponse(
        bars=bars,
        summary={
            "lastClose": last_close,
            "predEnd": pred_end,
            "predHigh": max(highs),
            "predLow": min(lows),
            "changePct": round(change_pct, 2),
            "direction": direction,
            "confidence": round(confidence, 1),
            "predLen": req.predLen,
        },
    )


@asynccontextmanager
async def _lifespan(app: FastAPI):
    yield
    # 进程退出由宿主(go-stock)管理，这里无需额外清理


app = FastAPI(title="go-stock Kronos Service", lifespan=_lifespan)


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/predict", response_model=PredictResponse)
def predict(req: PredictRequest):
    return _run_predict(req)


@app.post("/predict_batch", response_model=BatchPredictResponse)
def predict_batch(req: BatchPredictRequest):
    """批量预测：GPU 上逐只推理（predict_batch 要求各序列等长，暂不强制）；
    单只失败不影响其他，错误单独返回。"""
    results: dict[str, PredictResponse | None] = {}
    errors: dict[str, str] = {}
    for item in req.items:
        try:
            sub = PredictRequest(
                candles=item.candles,
                predLen=req.predLen,
                T=req.T,
                topP=req.topP,
                sampleCount=req.sampleCount,
                model=req.model,
                device=req.device,
            )
            results[item.code] = _run_predict(sub)
        except HTTPException as e:
            results[item.code] = None
            errors[item.code] = str(e.detail)
        except Exception as e:
            results[item.code] = None
            errors[item.code] = str(e)
    return BatchPredictResponse(results=results, errors=errors)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=8765)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()
    import uvicorn

    uvicorn.run(app, host=args.host, port=args.port, log_level="warning")
