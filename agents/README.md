# agents

`agents` 是 `damai-go` 的 Python 智能客服组件，当前提供面向 assistant-ui external-store 的 `Thread / Message / Run` API。

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
- `POST /agent/threads/{threadId}/messages`
- `GET /agent/threads/{threadId}/runs/{runId}`

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
ORDER_MCP_ENDPOINT=http://127.0.0.1:9082/message
ORDER_RPC_TARGET=127.0.0.1:8082
PROGRAM_RPC_TARGET=127.0.0.1:8083
USER_RPC_TARGET=127.0.0.1:8080
```

## 运行时说明

- LangGraph checkpoint 仍写入 Redis，但只作为内部运行状态，不对外暴露。
- 线程、消息、运行读模型写入 MySQL `damai_agents`。
- Redis ownership 已切换为 `threadId -> userId`。
- 已移除旧 chat demo 接口，不再提供兼容层。

## 测试

```bash
uv run pytest tests/test_api.py tests/test_e2e_contract.py tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py tests/test_session_store.py tests/test_docs.py tests/test_smoke.py tests/test_config.py -v
```
