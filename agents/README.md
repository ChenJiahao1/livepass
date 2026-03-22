# agents

`agents` 是根级 Python 组件，负责承接 `/agent/chat` 请求，并在后续任务中接入会话存储、编排和 RPC tools。

## 开发

```bash
uv run uvicorn app.main:app --reload
```

## 生成 gRPC stubs

```bash
bash scripts/generate_proto_stubs.sh
```
