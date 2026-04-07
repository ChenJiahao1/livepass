# agents

`agents` 是 `damai-go` 的 Python 智能客服组件，当前基于 `FastAPI + MCP + Redis` 运行。主链路已经收敛为 `ParentAgent -> TaskCard -> SubagentRuntime -> ToolBroker -> MCP Provider`，其中父层负责 LLM 编排，subagent 只执行单一 skill。

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
- `ParentAgent` 是真正的 LLM 编排者：负责理解用户诉求、决定直接回复/澄清/知识问答/下发 TaskCard，而不是关键词路由器。
- Python 侧 subagent 只有一份固定 system prompt；具体业务流程、步骤约束和结束条件从 `app/skills/<skill-name>/SKILL.md` 动态载入，运行时注入的是完整 `SKILL.md`。
- `SKILL.md` 按 Agent Skills / DeerFlow 兼容格式组织并做校验：当前支持 `name`、`description`、`license`、`compatibility`、`metadata`、`allowed-tools`，并兼容 DeerFlow 接受的 `version` / `author` 扩展键。
- `allowed-tools` 按规范使用空格分隔字符串；`metadata` 约束为 string-to-string 映射。
- 旧的 LangGraph `graph/coordinator/supervisor` 兼容层、旧专家代理目录和重复 `app/clients` 目录已移除。
- `TaskCard` 当前只携带单一 `skill_id`，由父层选定 skill，subagent 不再接收 `allowed_skills` 这类多选策略字段。
- `MCPToolRegistry` 按 provider 首次命中时懒加载并缓存 tool catalog；实际暴露给 subagent 的，只是当前 skill 绑定的那组 tools。
- Python 侧 `ToolBroker` 负责 tool 白名单、上下文注入、provider 调度以及对当前 task 的 tool 绑定，不承载业务规则。
- 明星基础百科问题走独立 `KnowledgeService`，不经过业务 subagent；实时新闻、八卦类问题会返回能力边界提示。
- “帮我退最近那单” 会走 `ParentAgent(LLM)` -> `order.list_recent` -> `refund.preview` 链路；业务失败时回退到 `handoff.create_ticket`。

## 测试

```bash
uv run pytest tests/test_task_card.py tests/test_provider_registry.py -v
uv run pytest tests/test_mcp_registry.py tests/test_go_provider_registry.py tests/test_tool_broker.py -v
uv run pytest tests/test_parent_agent.py tests/test_policy_engine.py tests/test_skill_resolver.py tests/test_subagent_runtime.py -v
uv run pytest tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_knowledge_agent.py tests/test_session_store.py tests/test_api.py tests/test_e2e_contract.py tests/test_docs.py tests/test_smoke.py -v
```
