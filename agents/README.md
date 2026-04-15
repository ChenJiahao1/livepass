# agents

`agents` 是 `damai-go` 的 Python 智能客服组件，当前基于 `FastAPI + LangGraph + MCP + Redis` 运行。`/agent/chat` 的主链路已经恢复为 `coordinator -> supervisor -> specialist`，其中 LangGraph 负责主编排与会话状态推进，MCP provider 负责订单、退款、节目和人工转接等工具能力。

## 入口

HTTP API:

```bash
uv run uvicorn app.main:app --reload
```

默认对外提供 `POST /agent/chat`。

Go `order` MCP provider:

```bash
go run ./services/order-rpc/cmd/order_mcp_server -f services/order-rpc/etc/order-mcp.yaml
```

Python `handoff` provider:

```bash
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
ORDER_MCP_ENDPOINT=http://127.0.0.1:9082/message
ORDER_RPC_TARGET=127.0.0.1:8082
PROGRAM_RPC_TARGET=127.0.0.1:8083
USER_RPC_TARGET=127.0.0.1:8080
```

未设置 `OPENAI_API_KEY` 时，`/agent/chat` 会直接返回 `503`，不再保留无模型 fallback。

## 运行时说明

- Go `order` MCP provider 基于 go-zero 官方 `mcp` 组件，通过内部 HTTP/Streamable HTTP 提供订单查询和退款能力。
- Python `handoff` provider 继续通过本地 `damai-mcp-server` stdio 启动。
- LangGraph 状态定义在 `app/state.py`，主状态通过 `app/session/checkpointer.py` 写入 Redis；`app/session/store.py` 只保留 `conversation_id -> user_id` 的 ownership 校验。
- `coordinator` 只做 `respond / clarify / delegate` 三类判定，不直接调用工具。
- `supervisor` 只决定下一跳 specialist 或 `finish`，并在 specialist 完成后统一收口。
- 当前 specialist 包括 `order`、`refund`、`activity`、`handoff`、`knowledge`：
  - `order`：优先列单或查单详情
  - `refund`：先做退款预览，确认后才提交退款
  - `activity`：节目、场次、票档与库存咨询
  - `handoff`：创建人工接管请求并返回 `need_handoff=True`
  - `knowledge`：通过 `KnowledgeService` 处理明星基础百科，不回答实时新闻、八卦或热搜
- `MCPToolRegistry` 按 toolset 复用当前 provider 连接：`order/refund` 走 go-zero MCP，`activity/handoff` 走 Python MCP。

## 测试

```bash
uv run pytest tests/test_prompts.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_graph.py -v
uv run pytest tests/test_agents.py tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_knowledge_agent.py -v
uv run pytest tests/test_api.py tests/test_e2e_contract.py tests/test_session_store.py tests/test_docs.py tests/test_smoke.py tests/test_config.py -v
```
