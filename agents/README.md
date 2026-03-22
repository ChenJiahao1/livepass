# agents

`agents` 是 `damai-go` 的 Python 智能客服组件，当前基于 `FastAPI + LangGraph + MCP + Redis` 运行。

## 入口

HTTP API:

```bash
uv run uvicorn app.main:app --reload
```

默认对外提供 `POST /agent/chat`。

MCP server:

```bash
uv run damai-mcp-server --toolset order
uv run damai-mcp-server --toolset refund
uv run damai-mcp-server --toolset activity
uv run damai-mcp-server --toolset handoff
```

## 关键环境变量

```bash
OPENAI_API_KEY=
OPENAI_BASE_URL=
OPENAI_MODEL=gpt-4.1-mini
LIGHTRAG_BASE_URL=http://127.0.0.1:9621
LIGHTRAG_API_KEY=
REDIS_URL=redis://127.0.0.1:6379/0
ORDER_RPC_TARGET=127.0.0.1:8082
PROGRAM_RPC_TARGET=127.0.0.1:8083
USER_RPC_TARGET=127.0.0.1:8080
```

未设置 `OPENAI_API_KEY` 时，HTTP API 会退回到本地关键词路由，不阻塞服务启动。

## 生成 gRPC stubs

```bash
bash scripts/generate_proto_stubs.sh
```

生成目录为 `app/rpc/generated/`。

## 测试

```bash
uv run pytest tests/test_smoke.py tests/test_config.py tests/test_router.py -v
uv run pytest tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_agents.py tests/test_graph.py -v
uv run pytest tests/test_mcp_server.py tests/test_mcp_registry.py tests/test_rpc_clients.py -v
uv run pytest tests/test_session_store.py tests/test_api.py tests/test_e2e_contract.py -v
uv run pytest tests/test_docs.py -v
```

最终验收：

```bash
uv run pytest -v
uv run python -c "from app.main import create_app; app=create_app(); print(app.title)"
uv run python -c "from app.mcp_server.server import build_server; print(build_server('order'))"
```

## 已知限制

- `knowledge` 依赖外部 `LightRAG` HTTP 服务；当前实现保留接口，但本地未做在线验证。
- `MCP` toolset 当前覆盖 `activity`、`order`、`refund`、`handoff`，`knowledge` 不走 MCP。
