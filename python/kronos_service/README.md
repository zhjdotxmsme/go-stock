# go-stock Kronos 推理服务

基于 [Kronos](https://github.com/shiyu-coder/Kronos)（清华开源 K 线基础模型）的本地推理服务，
由 go-stock 设置页托管拉起（默认端口 127.0.0.1:8765），为 K 线图与持仓深度分析提供未来 K 线预测。

## 手动部署（可选，go-stock 一键初始化会自动完成）

```bash
cd python/kronos_service
python -m venv venv
venv/Scripts/pip install -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple   # Windows
# 拉取 Kronos 官方模型实现（model.py / module.py 等）
git clone https://github.com/shiyu-coder/Kronos.git kronos_repo
# Windows: mklink /D model kronos_repo\model   其他: ln -s kronos_repo/model model
python app.py --port 8765
```

模型权重首次调用时自动从 HuggingFace 下载（Kronos-small 约 100MB）；离线环境可提前下载
[NeoQuasar/Kronos-small](https://huggingface.co/NeoQuasar/Kronos-small) 放入 HF 缓存目录。

## 接口

- `GET /health` → `{"status":"ok"}`
- `POST /predict`：`{candles:[{date,open,high,low,close,volume,amount}], predLen, T, topP, sampleCount, model, device}`
  → `{bars:[未来K线], summary:{lastClose,predEnd,predHigh,predLow,changePct,direction,confidence,predLen}}`

## 测试

```bash
python -m pytest test_app.py -v
```
