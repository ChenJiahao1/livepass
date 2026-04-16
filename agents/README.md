# agents

`agents` 是 `damai-go` 的 Python 智能客服组件，当前提供面向 assistant-ui external-store 的 `Thread / Message / Run` API。

当前运行时基线：

- Python 3.12
- LangGraph 1.1.6
- FastAPI + LangGraph + MCP + Redis + MySQL

## 入口

```bash
uv run uvicorn app.main:app --reload
```

当前 HTTP API：

- `POST /agent/threads`
- `GET /agent/threads`
- `GET /agent/threads/{threadId}`
- `PATCH /agent/threads/{threadId}`
- `GET /agent/threads/{threadId}/messages`
- `POST /agent/runs`
- `GET /agent/runs/{runId}`
- `GET /agent/runs/{runId}/events`
- `POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume`
- `POST /agent/runs/{runId}/cancel`

`POST /agent/runs` 仅接收当前线程下本轮输入，核心请求体字段为 `threadId`、`input.parts` 与 `metadata`。

## 关键环境变量

```bash
OPENAI_API_KEY=
OPENAI_BASE_URL=
OPENAI_MODEL=gpt-4.1-mini
LIGHTRAG_BASE_URL=http://127.0.0.1:9621
LIGHTRAG_API_KEY=
REDIS_URL=redis://127.0.0.1:6379/0
AGENTS_MYSQL_HOST=127.0.0.1
AGENTS_MYSQL_PORT=3306
AGENTS_MYSQL_USER=root
AGENTS_MYSQL_PASSWORD=123456
AGENTS_MYSQL_DATABASE=damai_agents
AGENTS_MYSQL_CHARSET=utf8mb4
ACTIVITY_MCP_ENDPOINT=http://127.0.0.1:9083/message
ORDER_MCP_ENDPOINT=http://127.0.0.1:9082/message
```

## 运行时说明

- 业务工具通过 Go MCP server 提供：`activity` 走 `program-mcp`，`order/refund` 走 `order-mcp`。
- `handoff` 当前不再通过 MCP 执行，仅在编排层保留 TODO 占位。
- LangGraph checkpoint 仍写入 Redis，但只作为内部运行状态，不对外暴露。
- 退款 HITL 中断走图内 `interrupt()` / `Command(resume=...)` 恢复链路，不再额外维护 executor 手写退款分支。
- 线程、消息、运行读模型写入 MySQL `damai_agents`。
- Redis ownership 已切换为 `threadId -> userId`。
- 已移除旧 chat demo 接口，不再提供兼容层。
- 历史消息通过 `GET /agent/threads/{threadId}/messages` 查询；活动态可通过 `GET /agent/runs/{runId}/events?after=<sequenceNo>` 的 `after 游标回放历史事件` 并续接增量事件。
- `POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume` 与 `POST /agent/runs/{runId}/cancel` 在同一请求重复提交时保持安全，`resume / cancel 接口按同一请求做幂等处理`。

## 本地联调

```bash
# Go MCP servers
go run ./services/order-rpc/cmd/order_mcp_server -f services/order-rpc/etc/order-mcp.yaml
go run ./services/program-rpc/cmd/program_mcp_server -f services/program-rpc/etc/program-mcp.yaml

# agents API
uv run uvicorn app.main:app --reload
```

## 测试

```bash
uv run pytest tests/test_api.py tests/test_run_contract_api.py tests/test_run_stream_service.py tests/test_run_executor.py tests/test_run_resume_cancel_api.py tests/test_e2e_contract.py tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py tests/test_session_store.py tests/test_docs.py tests/test_smoke.py tests/test_config.py -v
```
