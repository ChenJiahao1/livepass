# agents LangGraph Harness 恢复设计

## 目标

本次设计目标是把 `agents` 主链路恢复为备份分支中的 `coordinator + supervisor + specialist + graph` 架构，但以当前 `main` 的能力接入形态为准：

- `agents` 主编排正式回到 `LangGraph`
- 角色拆分恢复为 `coordinator`、`supervisor`、5 个 `specialist`
- `supervisor` 作为 harness，统一掌控状态推进、确认门禁、失败回退与转人工
- Go 服务内继续启动 MCP Server
- Python 侧继续通过 MCP Client 调用能力
- `POST /agent/chat` 的对外 contract 尽量保持当前 `main` 不变
- `conversation_id -> thread_id`，使用 LangGraph checkpoint 作为主状态存储

## 本期范围

本期包含：

- 恢复 `agents/app/agents/` 目录与角色拆分
- 恢复 `agents/app/graph.py`
- 恢复 `agents/app/state.py`
- 恢复 `agents/app/session/checkpointer.py`
- 恢复 `agents/prompts/` 下的主流程 prompts
- 恢复 5 个 specialist：
  - `order`
  - `refund`
  - `activity`
  - `handoff`
  - `knowledge`
- 调整 API 主链路，从 `ParentAgent` 切到 `LangGraph graph`
- 保持当前 MCP registry / client / provider 接入方式

本期不包含：

- 不一次性删除 `ParentAgent`、`TaskCard`、`SubagentRuntime`
- 不重写 Go 侧 MCP Server 组织方式
- 不扩新增业务域，仅恢复上述 5 个 specialist
- 不改变网关侧 `/agent/chat` 透传方式
- 不重写现有外部响应字段命名

也就是说，本期先恢复正确的编排骨架和状态模型，不推翻现有能力接入层，也不做无关重构。

## 背景

当前 `main` 的 `agents` 已经具备：

- Python `agents` 服务入口
- Go 侧 MCP provider 与 Python MCP client 的能力连接
- 订单、退款、活动、转人工、知识问答等基础能力

但当前热路径已经收敛为：

- `ParentAgent -> TaskCard -> SubagentRuntime -> ToolBroker -> MCP Provider`

这个形态的问题不是“不能用”，而是它把本应显式建模的编排状态收进了 prompt 与通用 agent loop：

- 父层虽然承担了编排职责，但状态推进不是 LangGraph 原生图状态
- `SubagentRuntime` 仍然依赖通用 `create_agent(...)` 的执行循环
- 写操作前确认、跨 specialist 的状态推进、失败回退都更依赖 prompt 约束，而不是 graph 节点与状态机

仓库已有备份分支 `backup/main-20260404-183604`，其中 `agents` 曾采用：

- `FastAPI + LangGraph + MCP + Redis`
- `coordinator + supervisor + specialist + graph`

这与当前目标更一致。此次调整不是简单回退到旧实现，而是：

- 恢复旧分层
- 保留当前更成熟的 MCP 接入形态
- 把 LangGraph 重新设为唯一主编排

## 设计原则

### 原则一：LangGraph 是唯一主编排

主链路应由 LangGraph 负责状态推进、分支选择和会话恢复，而不是继续由父层 prompt 隐式控制。

### 原则二：supervisor 掌控流程，specialist 只做受控执行

`specialist` 可以调用 tool，也可以借助 LLM 生成域内话术，但不拥有跨域调度权。跨 specialist 的流程控制必须收敛在 `supervisor`。

### 原则三：对外 contract 不变，内部运行时切换

`POST /agent/chat` 的路径、基础请求响应语义、`conversationId`、`reply`、`needHandoff`、`trace` 等主要字段尽量保持兼容，避免网关与验收脚本联动破坏。

### 原则四：MCP 接入层不回退

Go 服务继续在 `go-zero` 内启动 MCP Server；Python 侧继续使用当前 MCP Client / Registry 调用工具，不恢复到旧的专用 RPC-only 接入方式。

### 原则五：Checkpoint 是唯一主状态源

`conversation_id` 应映射为 LangGraph `thread_id`，由 checkpointer 负责状态恢复。旧的 session store 若仍需保留，也只作为兼容辅助，而不是主事实源。

## 方案对比

### 方案 A：继续沿用当前 `ParentAgent` 主链路，只局部补 LangGraph

优点：

- 改动面最小
- 短期内最稳

缺点：

- 会同时存在两套编排心智模型
- `supervisor harness` 不会真正成立
- 后续仍要再次迁移

不推荐。

### 方案 B：恢复 LangGraph 热路径，但保留当前 MCP 接入层

优点：

- 结构上回到 `coordinator + supervisor + specialist + graph`
- 保留当前成熟的 MCP 组织方式
- 对外 contract 可以保持稳定
- 可以分阶段退出旧主链路

缺点：

- 迁移期内仓库会短暂存在两套编排代码

这是推荐方案。

### 方案 C：完整回摆到备份分支，再重新嫁接当前能力层

优点：

- 结构最整齐

缺点：

- 改动面最大
- 风险最高
- 容易破坏当前 API、测试和 MCP 接入

不推荐。

## 目录与运行时设计

本期建议恢复下面的目录结构：

```text
agents/
├── app/
│   ├── agents/
│   │   ├── base.py
│   │   ├── coordinator.py
│   │   ├── supervisor.py
│   │   ├── order.py
│   │   ├── refund.py
│   │   ├── activity.py
│   │   ├── handoff.py
│   │   └── knowledge.py
│   ├── graph.py
│   ├── state.py
│   └── session/
│       └── checkpointer.py
└── prompts/
    ├── coordinator/system.md
    ├── supervisor/system.md
    ├── order/system.md
    ├── refund/system.md
    ├── activity/system.md
    └── handoff/system.md
```

主链路如下：

1. `/agent/chat` 接收请求
2. API 层将 `conversation_id` 映射为 LangGraph `thread_id`
3. Graph 从 `checkpointer` 恢复历史状态
4. `coordinator` 判断本轮是直接回复、澄清还是进入业务编排
5. `supervisor` 决定下一跳 specialist 或结束
6. specialist 通过当前 MCP client 调 Go / Python MCP provider
7. specialist 返回结构化结果给 `supervisor`
8. `supervisor` 决定结束、继续、等待确认或转人工
9. Graph 结束后，API 层把结果包装成当前 `/agent/chat` 的兼容响应

## 角色职责边界

### coordinator

职责：

- 会话入口分流
- 判断是：
  - 直接答复
  - 追问澄清
  - 进入业务编排
- 识别明显的 `knowledge` 类问题

不负责：

- 不直接执行业务 tool
- 不直接执行退款、查单、转人工
- 不决定复杂多步业务流的下一跳

### supervisor

职责：

- 作为全局 harness
- 依据当前状态选择 specialist
- 控制多步业务流
- 判断是否结束、是否继续、是否转人工
- 执行确认态门禁
- 处理失败回退和兜底

不负责：

- 不直接查询业务事实
- 不直接调底层业务工具

### specialist

职责：

- 在单一业务域内完成受控执行
- 调用当前 graph 授权的工具
- 返回结构化结果给 `supervisor`

约束：

- 不拥有跨域调度权
- 不自行切换 specialist
- 输出必须稳定、可审计、便于 `supervisor` 决策

## Specialist 设计

第一批恢复这 5 个 specialist：

### `order`

- 查询最近订单列表
- 查询订单客服视角详情
- 必要时帮助锁定 `selected_order_id`

### `refund`

- 退款预览
- 在确认后提交退款

### `activity`

- 节目、场次、票档、规则、限购、入场、退票相关只读咨询

### `handoff`

- 创建人工客服工单

### `knowledge`

- 明星基础百科类问答
- 与实时新闻、八卦、热搜显式区分

## 状态模型设计

建议将 `ConversationState` 分为跨轮持久状态和当前轮临时状态。

### 跨轮持久状态

- `messages`
- `last_intent`
- `selected_program_id`
- `selected_order_id`
- `current_user_id`
- `last_refund_preview`
- `pending_confirmation`
- `pending_action`
- `need_handoff`

其中 `last_refund_preview` 至少包含：

- `order_id`
- `allow_refund`
- `refund_amount`
- `reject_reason`

### 当前轮临时状态

- `route`
- `coordinator_action`
- `next_agent`
- `business_ready`
- `delegated`
- `reply`
- `final_reply`
- `specialist_result`
- `status`
- `trace`
- `current_agent`

### specialist_result 统一结构

建议统一包含：

- `agent`
- `completed`
- `need_handoff`
- `result_summary`
- `selected_order_id`
- `payload`

这样 `supervisor` 决策依赖结构化字段，而不是依赖解析 specialist 生成的自然语言。

## 关键状态规则

### 规则一：写操作不能只靠用户文本直接触发

例如用户说“帮我退款”，如果没有 `last_refund_preview`，不能直接提交退款，必须先走：

- 订单锁定
- 退款预览
- 确认态建立
- 退款提交

### 规则二：多订单场景必须显式选单

如果当前账号下有多笔订单，必须先展示列表并等待用户选择，不能由 LLM 猜测订单。

### 规则三：current_user_id 只来自外部可信上下文

`current_user_id` 必须来自网关透传或运行时上下文，不允许模型推断。

### 规则四：need_handoff 是会话级状态

一旦进入人工处理路径，相关状态需要能跨轮保留，避免重复承诺或重复创建工单。

## 主流程设计

### 订单查询链路

1. `coordinator` 判断为业务请求
2. `supervisor` 路由到 `order`
3. `order`：
   - 已有 `selected_order_id`：查订单详情
   - 无 `selected_order_id`：按 `current_user_id` 查最近订单
4. 返回结果：
   - 0 笔订单：直接结束
   - 1 笔订单：锁定订单，可直接返回详情
   - 多笔订单：展示订单列表，等待用户下轮选单

### 退款链路

1. 用户提出退款请求
2. `supervisor` 判断是否已有 `selected_order_id`
3. 无订单号时，先走 `order`
4. 有订单但无 `last_refund_preview` 时，走 `refund` 预览
5. 用户明确确认后，`supervisor` 才允许走 `refund` 提交
6. 失败、争议、越权场景转 `handoff`

### 活动咨询链路

1. 用户咨询活动、票档、规则类问题
2. `supervisor` 路由到 `activity`
3. `activity` 调用只读 MCP 工具
4. 通常本轮直接结束；如缺少关键上下文，则返回澄清提示

### 转人工链路

1. 用户明确要求人工，或业务链路判断自动处理不合适
2. `supervisor` 路由到 `handoff`
3. `handoff` 创建工单
4. 成功则结束，并返回 `need_handoff=true`

### 知识问答链路

1. 用户提问明星基础百科
2. 路由到 `knowledge`
3. 若为实时新闻、八卦、热搜，返回能力边界提示，而不是冒充实时事实

## API 与状态存储设计

### API contract

本期要求：

- 保持 `POST /agent/chat`
- 保持当前主要请求参数与响应字段兼容
- 保持与网关、验收脚本、现有 API 测试的兼容性

### Checkpointer

本期采用备份分支 `session/checkpointer.py` 的思路：

- 使用 LangGraph checkpoint 作为主状态存储
- `conversation_id` 映射为 `thread_id`
- Redis 为 graph 提供跨轮状态恢复

旧的 `session/store.py` 若仍有兼容价值，可以暂时保留，但不再作为主状态源。

## 迁移策略

### 阶段一：恢复 graph 骨架

- 恢复 `state.py`
- 恢复 `prompts.py` 与 `prompts/*`
- 恢复 `coordinator.py`
- 恢复 `supervisor.py`
- 恢复 `graph.py`
- 恢复 `session/checkpointer.py`
- 让 `/agent/chat` 能切到 graph 主链路

目标是先打通：

- API
- graph
- checkpoint
- coordinator / supervisor

### 阶段二：接回 5 个 specialist

- 恢复 `order`
- 恢复 `refund`
- 恢复 `handoff`
- 恢复 `activity`
- 恢复 `knowledge`

同时保持当前 MCP registry / client 接入方式，不回退到底层 RPC-only 模型。

### 阶段三：完成会话切换

- 正式以 checkpoint 为主状态源
- 完成 `conversation_id -> thread_id` 绑定
- 补齐跨轮恢复测试

### 阶段四：退出旧主链路

待 graph 主链路与测试稳定后，再逐步退出：

- `orchestrator/parent_agent.py`
- `runtime/subagent_runtime.py`
- `tasking/task_card.py`
- 以及只服务于旧主链路的测试

本阶段不要求立即删除，只要求它们退出热路径。

## 测试策略

第一批测试建议覆盖：

- graph 组装与节点路由测试
- coordinator / supervisor 结构化决策测试
- `/agent/chat` contract 兼容测试

第二批测试建议覆盖：

- `order` / `refund` / `handoff` 主流程测试
- checkpoint 恢复测试
- 多轮选单与确认态测试

第三批测试建议覆盖：

- `activity` / `knowledge` specialist 测试
- e2e contract 测试
- README / docs 一致性测试

## 风险与取舍

### 风险一：迁移期双轨代码并存

短期内会同时存在：

- 旧的 `ParentAgent` 主链路代码
- 新的 LangGraph 主链路代码

这是可接受代价，前提是旧链路退出热路径，避免双主链并行。

### 风险二：旧 specialist 与当前 MCP registry 不完全兼容

备份分支 specialist 的工具查找方式可能与当前 registry 细节不完全一致，因此迁移时应优先保留当前 registry 抽象，在 specialist 层做适配。

### 风险三：状态字段命名与现有 API 测试耦合

对外响应应优先兼容当前 contract；必要时允许 graph 内部状态字段与 API 输出字段分离。

## 结论

推荐采用“恢复 LangGraph 热路径，但保留当前 MCP 接入层”的方案。

最终目标不是简单回到旧代码，而是恢复正确的 harness 架构：

- `coordinator` 负责入口分流
- `supervisor` 负责流程控制
- `specialist` 负责单域受控执行
- `graph + checkpoint` 负责主状态与跨轮恢复

这样既能符合 LangGraph 的原生使用方式，也能保留当前 `damai-go` 已经形成的 `go-zero MCP Server + Python MCP Client` 能力边界。
