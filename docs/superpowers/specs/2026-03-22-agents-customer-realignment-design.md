# damai-go agents 对齐 customer 控制流设计

## 背景

当前 `damai-go/agents` 已经具备以下正式外壳能力：

- `FastAPI` 对外暴露 `/agent/chat`
- `Redis` 维护 `conversation_id` 与会话归属
- `MCP` 按 `activity/order/refund/handoff` 组织工具边界
- `RPC` 客户端直连 `damai-go` 服务

但内部 agent runtime 与原始 `/home/chenjiahao/code/project/customer` 已经出现明显漂移，主要体现在：

- graph 被压平成 `coordinator -> supervisor -> specialist -> END`
- `prompt` 从强约束职责说明退化为概述式提示
- `ConversationState` 的字段仍保留部分名称，但语义和生命周期不再一致
- 测试主要覆盖 smoke 与 API 契约，缺少原始行为基线约束

这导致当前 `agents` 的外部形态与 `customer` 接近，但核心客服职责链已经不再等价。

## 目标

在保留 `damai-go` 现有 `FastAPI + Redis + MCP + RPC` 接入层的前提下，恢复 `customer` 的内部控制流和状态语义，使 `agents` 回到“每轮从入口进入，内部完成调度，到 `END` 结束，等待下一条用户消息”的正式客服模型。

本次对齐需要满足：

1. 恢复 `customer` 的 `coordinator -> supervisor -> specialist -> supervisor -> finish` 控制流
2. 恢复 `ConversationState` 的关键语义和字段生命周期
3. 恢复 `prompt` 的职责边界，避免 LLM 自由漂移
4. 恢复关键行为测试基线
5. 保留当前 `damai-go` 的 `FastAPI`、`Redis`、`MCP`、`RPC` 集成方式

## 不在本次范围内

- 调整 `gateway-api` 对外契约
- 把 `knowledge` 纳入 `MCP`
- 新增长期记忆、用户画像或埋点
- 变更 `MCP` toolset 的服务边界
- 引入真实人工服务系统

## 设计结论

推荐采用“回迁 `customer` 内核，保留 `damai-go` 外壳”的方案：

- 外层继续使用当前 `FastAPI`、`Redis` 会话归属、`MCP` tool registry 和真实 `RPC` 客户端
- 内层恢复 `customer` 的 graph 控制流、prompt 约束、state 语义和行为测试
- 每个 HTTP 请求都是一次独立 graph invocation
- 单轮请求内部允许 `specialist -> supervisor` 的最小回环调度
- 一旦当前轮完成，graph 即进入 `END`，系统等待下一条用户消息重新从入口进入

## 总体架构

### 1. API 与会话层

职责：

- 暴露 `POST /agent/chat`
- 校验 `X-User-Id`
- 获取或创建 `conversation_id`
- 将用户请求映射为 graph invocation
- 把结果组装为稳定响应契约

保持不变：

- `conversationId`
- `reply`
- `status`

其中：

- `need_handoff == true` 时返回 `handoff`
- 否则返回 `completed`

### 2. Agent Runtime 层

恢复 `customer` 基线：

- `CoordinatorAgent`
- `SupervisorAgent`
- `ActivityAgent`
- `OrderAgent`
- `RefundAgent`
- `HandoffAgent`
- `KnowledgeAgent`
- `LangGraph` runtime

职责划分：

- `coordinator` 只决定 `respond` / `clarify` / `delegate`
- `supervisor` 只决定 specialist 下一跳或 `finish`
- specialist 负责工具调用与结果组织
- `knowledge` 继续独立直连 `LightRAG`

### 3. Tool 接入层

保持当前正式边界：

- agent runtime 只依赖 `MCPToolRegistry`
- `MCP` server 内部继续通过真实 `RPC` client 调用 `damai-go` 服务
- `knowledge` 不纳入 `MCP`

## 控制流设计

### 请求边界

每一条 `/agent/chat` 请求都视为一轮独立对话执行：

1. API 读取或创建会话
2. 使用 `conversation_id` 作为 `thread_id`
3. graph 从 `START` 开始运行
4. graph 在当前轮内部完成调度
5. 到达 `END` 后返回回复
6. 服务等待下一条用户消息

下一条消息再次从入口进入，不会从上一个 specialist 节点中间续跑。

### 单轮内部控制流

恢复为：

`START -> coordinator -> supervisor -> specialist -> supervisor -> finish -> END`

规则：

- `coordinator=respond` 或 `clarify` 时直接结束本轮
- `coordinator=delegate` 时进入 `supervisor`
- `supervisor` 根据当前上下文选择 `activity/order/refund/handoff/knowledge/finish`
- specialist 完成后必须回到 `supervisor`
- `supervisor` 依据 `specialist_result` 判断是否 `finish`

### 不采用的方案

不保留当前“specialist 直接 `END`”的压平模型，因为它会丢失以下能力：

- specialist 完成后的显式完成态判断
- 列单后结束等待用户下一条消息的正式语义
- `knowledge`、`handoff` 等特殊 agent 的统一收尾逻辑
- 退款前先列单的可解释路由链

## 状态语义设计

### 跨轮保留字段

以下字段可跨轮保留：

- `messages`
- `last_intent`
- `selected_program_id`
- `selected_order_id`
- `current_user_id`

### 单轮临时字段

以下字段只服务当前轮，进入新一轮前必须清理：

- `route`
- `coordinator_action`
- `next_agent`
- `business_ready`
- `delegated`
- `specialist_result`
- `need_handoff`
- `trace`
- `current_agent`
- `final_reply`
- `reply`
- `status`

### 轮次准备

在每次 graph invocation 进入业务节点前增加一个轻量的 turn prepare 步骤，职责仅包括：

1. 用 runtime context 注入或覆盖 `current_user_id`
2. 清理上轮遗留的单轮字段

这样可以避免上一轮的 `specialist_result` 或 `need_handoff` 污染下一条消息。

### 字段语义

- `route`：当前轮内部业务主线，不作为长期记忆
- `last_intent`：最近一次确认过的业务意图，可跨轮复用
- `selected_order_id`：如果用户本轮显式提供新订单号，则覆盖旧值；否则可复用历史值
- `selected_program_id`：与 `selected_order_id` 同理
- `specialist_result`：仅用于当前轮 supervisor 的完成态判断

`specialist_result` 至少保留：

- `agent`
- `completed`
- `need_handoff`
- `result_summary`

## 完成态规则

### coordinator

- `respond`：当前轮直接结束
- `clarify`：当前轮直接结束
- `delegate`：进入 `supervisor`

### supervisor

`supervisor` 必须优先读取 `specialist_result`，再决定是否继续路由。完成态优先于继续调度。

建议规则：

- `knowledge` 完成后直接 `finish`
- `handoff` 完成后直接 `finish`
- `order` 在“已展示多订单列表且未选中订单号”时直接 `finish`
- `refund` 缺订单号但存在 `current_user_id` 时，优先转 `order`
- specialist 已经形成明确回复、当前只需等待用户补充下一条消息时，直接 `finish`

### 跨轮复用

如果上一轮已经写入 `selected_order_id`，下一轮用户只说“帮我退款”，允许复用该订单号继续退款流。

## Specialist 行为约束

### activity

- 只处理节目、场次、票档、时间地点等咨询
- 优先复用 `selected_program_id`
- 不处理订单、退款、人工

### order

- 只处理订单状态、支付状态、票券状态、订单列表
- 未提供订单号但已登录时优先列单
- 多订单时只列单，不自动替用户选单
- 单订单时可直接返回详情，并写入 `selected_order_id`

### refund

- 只处理退款资格预览和退款申请
- 不自行放宽退款规则
- 缺订单号时不强行执行退款
- 已有订单号时，先做预览，再在明确申请时提交

### handoff

- 只负责转人工
- 进入该 agent 后默认生成“已转人工/正在转人工”的明确回复
- 完成时 `need_handoff=true`

### knowledge

- 只处理明星基础百科
- 不处理实时新闻、八卦、最新动态
- 超出边界时统一返回能力说明

## Prompt 职责边界

### coordinator

- 唯一直接面向用户的入口
- 处理寒暄、简单 FAQ、能力边界说明
- 负责业务问题是否需要补槽位
- 不调用工具
- 不直接回答百科
- 只有在业务请求具备进入内部流程条件时才 `delegate`

### supervisor

- 只在业务流内部工作
- 不调用工具
- 不直接回答业务内容
- 只决定下一跳 specialist 或 `finish`
- 先看 `specialist_result` 再判断是否继续

### specialists

各 specialist 的 prompt 只描述本域职责和工具使用边界，不互相越界。

## 测试策略

恢复并补齐以下五组测试：

### 1. Prompt 与路由层

覆盖：

- `coordinator` 的槽位补全规则
- `supervisor` 的路由和完成态规则
- `knowledge` 路由边界
- 退款缺订单号转 `order`

### 2. Graph 层

覆盖：

- `coordinator -> supervisor -> specialist -> supervisor -> finish`
- 每轮请求从入口开始
- 多轮通过 `thread_id/conversation_id` 继承上下文
- 列单后结束，等待下一条消息
- 已选中订单号在下一轮被复用

### 3. Specialist 层

覆盖：

- `order`：无订单、单订单、多订单
- `refund`：缺订单号、可退款、不可退款、提交退款
- `handoff`：`need_handoff`
- `knowledge`：成功、越界、配置缺失、超时、异常

### 4. MCP 与 RPC 层

保留当前正式接入测试，验证：

- tool 命名
- RPC 参数映射
- 返回值归一化
- 异常处理

### 5. API 与会话层

覆盖：

- `X-User-Id` 校验
- `conversation_id` 归属校验
- 多轮会话续写
- `handoff` 状态映射
- 每次请求都从入口进入但可继承历史上下文

## 实施顺序

### Step 1. 恢复 state 与 graph 语义

- 调整 `ConversationState`
- 引入 turn prepare
- 恢复 `specialist -> supervisor` 回环

### Step 2. 恢复 prompt 约束

- 回迁 `customer` 的强约束 prompt
- 把 demo 术语统一收口到 `damai-go` 领域语义

### Step 3. 恢复 agent 行为

- 恢复 `coordinator/supervisor` 的上下文注入
- 恢复 `order/refund/knowledge/handoff` 的关键行为

### Step 4. 恢复关键测试

- 先补 graph / supervisor / specialist 失败测试
- 再补 prompt 和 API 会话测试

### Step 5. 回归现有 MCP/RPC 与 API 契约

- 确保新的 runtime 不破坏当前 `FastAPI + Redis + MCP + RPC` 接口层

## 成功标准

本次改造完成后应满足：

1. 每条消息都从入口进入，到 `END` 结束
2. 单轮内部保留 `customer` 的职责链和最小回环调度
3. `prompt` 恢复明确边界，不再依赖概述式指令
4. 多轮消息可以正确复用 `selected_order_id/current_user_id`
5. 关键行为测试能够证明系统重新对齐 `customer`

## 风险

### 风险 1：历史状态污染下一轮

处理方式：

- 明确区分跨轮字段与单轮字段
- 每轮开始前清理临时状态

### 风险 2：prompt 恢复后与当前精简实现不兼容

处理方式：

- 先补失败测试，再调整 graph 与 agent

### 风险 3：回迁 `customer` 语义时混入 demo 命名

处理方式：

- 状态字段、tool 名和文案统一使用 `damai-go` 领域术语
