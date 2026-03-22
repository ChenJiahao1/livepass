# agents `/agent/chat` 接入设计

## 1. 背景

当前仓库 `damai-go` 已具备 `user`、`program`、`order`、`pay` 等核心服务，并通过 `gateway-api` 作为统一 HTTP 入口。另有一个位于仓库外的 Python CLI Demo：`/home/chenjiahao/code/project/customer`，其核心能力包括：

- LangGraph 多 Agent 编排
- coordinator / supervisor / specialist 分工
- 基于 tool-calling 的业务工具调用
- 面向票务客服的活动咨询、订单查询、退款、转人工流程

该 Demo 当前仍是 CLI 形态，且底层依赖本地 mock MCP server 与 mock 数据，不适合直接作为正式能力接入。

本设计目标是将其产品化为 `damai-go` 体系内的正式客服能力：

- 对外通过 `gateway-api` 暴露统一接口
- 对内由独立 `agents` Python 服务承接编排与会话
- 业务数据改为调用 `damai-go` 真实服务
- 首期使用普通 HTTP，后续可平滑扩展到 SSE

## 2. 目标与非目标

### 2.1 目标

- 新增正式客服接口：`POST /agent/chat`
- 要求用户已登录后才能使用该接口
- 支持多轮会话，客户端通过 `conversationId` 续接上下文
- 首期覆盖 4 类能力：
  - 活动咨询
  - 订单查询
  - 退款预检与退款发起
  - 转人工
- 保留 Python 多 Agent 编排能力，不重写为 Go
- 将 Demo 中的 mock MCP/数据调用替换为真实业务调用

### 2.2 非目标

- 首期不做 SSE / 流式输出
- 首期不做明星百科、通用知识问答
- 首期不做对公网直连的 `agents` 服务
- 首期不在 Python 侧实现 etcd 服务发现
- 首期不做历史会话列表与会话检索接口

## 3. 核心决策

### 3.1 对外入口继续走 `gateway-api`

`agents` 不直接暴露公网入口。客户端统一访问 `gateway-api` 暴露的 `/agent/chat`。

这样做的原因：

- 与现有对外 API 入口保持一致
- 统一复用网关鉴权能力
- 前端/客户端不需要感知 `agents` 的语言栈差异
- 后续限流、审计、埋点都可继续挂在网关

### 3.2 `agents` 作为根级 Python 服务存在

项目约束已明确 `agents` 是预留的 Python 独立组件，不纳入 `go-zero` 服务目录规范。因此正式接入时，`agents` 应作为仓库根级目录存在，而不是被塞入 `services/*`。

### 3.3 `agents` 对内直连 `*-rpc`

正式内部链路采用：

```text
Client
  -> gateway-api
    -> /agent/chat
      -> agents
        -> user-rpc / program-rpc / order-rpc
```

不采用 `agents -> gateway-api -> *-api` 的原因：

- 避免内部链路重复穿网关
- 少一跳，减少超时与链路复杂度
- 客服服务消费内部领域契约，而不是复用面向前端的 HTTP 聚合层

### 3.4 首期 Compose 场景不做 Python etcd 服务发现

当前部署形态以 Docker Compose 为主。首期 `agents` 通过 Compose 内部服务名直连 gRPC 服务，例如：

```env
USER_RPC_TARGET=user-rpc:8080
PROGRAM_RPC_TARGET=program-rpc:8083
ORDER_RPC_TARGET=order-rpc:8082
```

后续若迁移到更适合基于 DNS 的 gRPC 负载均衡环境，可将 target 切换为 `dns:///...` 形式，但本设计首期不在 Python 侧补 etcd 服务发现。

## 4. 总体架构

### 4.1 逻辑架构

```text
Client
  -> gateway-api
    -> agents API
      -> session store (Redis)
      -> LangGraph orchestrator
      -> activity/order/refund/handoff tools
      -> user-rpc / program-rpc / order-rpc
```

### 4.2 组件职责

- `gateway-api`
  - 暴露 `/agent/chat`
  - 校验用户登录态
  - 透传可信用户上下文至 `agents`
- `agents`
  - HTTP 接口接入
  - 会话读取与写回
  - LangGraph 多 Agent 编排
  - 调用业务工具与领域服务
  - 结构化生成最终回复
- Redis
  - 存储多轮会话
  - 保存槽位状态、滚动摘要、最近若干轮消息
- `user-rpc`
  - 提供用户基础信息与观演人相关能力
- `program-rpc`
  - 提供活动详情、票档、预下单信息
- `order-rpc`
  - 提供订单查询、退款预检、退款执行、客服视图能力

## 5. 对外接口设计

### 5.1 `POST /agent/chat`

对外统一由 `gateway-api` 暴露。

请求：

```json
{
  "conversationId": "optional",
  "message": "帮我看一下这个订单能不能退",
  "clientMessageId": "optional"
}
```

字段说明：

- `conversationId`
  - 首轮可不传
  - 后续多轮由客户端回传
- `message`
  - 当前用户输入
- `clientMessageId`
  - 可选，便于客户端幂等或前端链路追踪

响应：

```json
{
  "conversationId": "c_01HQ...",
  "reply": "订单 123456789 当前满足退款条件，已为你提交退款申请。",
  "status": "completed",
  "intent": "refund",
  "currentAgent": "refund",
  "needHandoff": false,
  "handoffTicketId": "",
  "suggestions": [
    "查看订单详情",
    "继续咨询其他订单"
  ]
}
```

建议响应字段：

- `conversationId`
- `reply`
- `status`
  - `completed`
  - `need_user_input`
  - `handoff_queued`
- `intent`
  - `activity`
  - `order`
  - `refund`
  - `handoff`
- `currentAgent`
- `needHandoff`
- `handoffTicketId`
- `suggestions`

### 5.2 鉴权与头透传

`gateway-api` 应将 `/agent/` 路径纳入鉴权范围。

鉴权成功后，网关向 `agents` 透传可信上下文，例如：

- `X-User-Id`
- `X-Request-Id`
- `X-Conversation-Id`（可选）

`agents` 不自行校验 JWT，不信任客户端自填 `userId`，仅信任网关透传结果。

## 6. 会话模型

### 6.1 会话主键

- 服务端生成 `conversationId`
- 客户端后续带回 `conversationId`
- Redis 会话记录必须同时绑定 `userId`

同一 `conversationId` 若被其他用户使用，应直接拒绝，避免串读会话。

### 6.2 会话内容

Redis 内建议保存：

- 最近 `N` 轮消息
- 滚动摘要
- 当前槽位状态
  - `selectedOrderNumber`
  - `selectedProgramId`
  - `lastIntent`
  - `needHandoff`
  - `pendingQuestion`
- 最近一次执行摘要

### 6.3 TTL

- 默认 TTL：7 天
- 每次会话写入自动续期

### 6.4 已转人工场景

会话内保留：

- `handoffTicketId`
- handoff 摘要
- 当前未解决原因

便于客服接续和后续审计。

## 7. `agents` 服务内部设计

### 7.1 建议目录

```text
agents/
├── app/
│   ├── api/
│   ├── orchestrator/
│   ├── session/
│   ├── tools/
│   ├── clients/
│   │   └── rpc/
│   ├── domain/
│   ├── handoff/
│   ├── prompts/
│   └── config.py
├── tests/
└── pyproject.toml
```

### 7.2 入口实现建议

`agents` 使用 Python HTTP 服务承接实际逻辑，推荐使用：

- FastAPI
- uvicorn

原因：

- 与当前 async 编排兼容
- 后续扩展 SSE 不需要换框架
- 接入请求校验、超时和依赖注入较直接

### 7.3 编排层演进

保留现有 Demo 的核心编排思路：

- coordinator
- supervisor
- specialist agents

但将 CLI 与 mock 数据替换为正式服务接入：

- CLI 仅保留为开发调试入口
- HTTP API 成为正式入口
- 会话不再使用内存 saver，而是落 Redis

## 8. 工具层替换方案

### 8.1 保留 tool-calling，移除 mock MCP server

首期不再保留 `uv run mock-mcp-server --toolset ...` 这种 stdio 子进程式 MCP 调用。

改为：

- 保留 toolset 概念
- 在 `agents` 进程内直接注册 LangChain tool
- tool 的底层实现改为调用真实 gRPC client

### 8.2 推荐 toolset

#### `activity`

- `search_programs`
- `get_program_detail`
- `get_program_preorder`

#### `order`

- `list_orders`
- `get_order_service_view`

#### `refund`

- `preview_refund_order`
- `submit_refund_order`

#### `handoff`

- `create_handoff_ticket`

### 8.3 工具到底层 RPC 的映射

- `search_programs`
  - `program-rpc.PagePrograms`
  - 需要时可辅以 `ListHomePrograms`
- `get_program_detail`
  - `program-rpc.GetProgramDetail`
- `get_program_preorder`
  - `program-rpc.GetProgramPreorder`
- `list_orders`
  - `order-rpc.ListOrders`
- `get_order_service_view`
  - 新增 `order-rpc.GetOrderServiceView`
- `preview_refund_order`
  - 新增 `order-rpc.PreviewRefundOrder`
- `submit_refund_order`
  - 现有 `order-rpc.RefundOrder`

## 9. 领域补口设计

### 9.1 原因

当前 `customer` Demo 的订单与退款视图是 demo 级字段模型，不应被直接搬入正式实现。

正式实现中，不应让 `agents` 自己拼接以下规则：

- 退款资格判定
- 支付状态解释
- 票券状态拼装
- 客服视角订单状态收敛

这些应仍然属于订单域。

### 9.2 `order-rpc` 新增接口建议

#### `GetOrderServiceView`

用途：为客服/AI 提供适合直接表达的订单视图。

建议返回：

- `orderNumber`
- `orderStatus`
- `payStatus`
- `ticketStatus`
- `programTitle`
- `programShowTime`
- `ticketCount`
- `orderPrice`
- `canRefund`
- `refundBlockedReason`

#### `PreviewRefundOrder`

用途：只做退款预检，不执行退款。

建议返回：

- `orderNumber`
- `allowRefund`
- `refundAmount`
- `refundPercent`
- `rejectReason`

### 9.3 领域职责边界

- `agents`
  - 做对话编排
  - 做槽位管理
  - 做用户提示和追问
- `order-rpc`
  - 做订单客服视图
  - 做退款资格预检
  - 做退款真正执行

## 10. 错误处理与降级

### 10.1 错误分类

#### 用户输入不足

例如：

- 没提供订单号
- 问题上下文不足
- 需要用户明确选择订单

处理方式：

- 返回 `need_user_input`
- 不视为系统失败
- 不转人工

#### 业务拒绝

例如：

- 订单不存在
- 无权访问该订单
- 当前订单不可退

处理方式：

- 返回正常业务回复
- 不转人工

#### 系统失败

例如：

- Redis 超时
- gRPC 超时
- LLM 调用失败
- tool 执行异常

处理方式：

- 对用户返回统一降级提示
- 标记 `needHandoff=true`
- 生成并保存 handoff 摘要

### 10.2 统一降级文案

系统不可恢复失败时，建议统一收口为：

> 当前处理失败，已转人工继续处理。

避免向用户暴露底层 RPC 或模型报错细节。

### 10.3 超时建议

- `gateway-api -> agents`
  - 总超时高于单次内部 RPC 超时
- `agents -> rpc`
  - 每次调用独立短超时
- `agents -> LLM`
  - 独立超时，避免无限等待

## 11. 测试策略

### 11.1 `services/order-rpc/tests/`

新增客服相关 RPC 集成测试：

- `GetOrderServiceView`
- `PreviewRefundOrder`

### 11.2 `services/gateway-api/tests/`

新增网关测试：

- `/agent/chat` 受鉴权保护
- 网关转发至 `agents`
- 头透传正确

### 11.3 `agents/tests/`

新增 Python 侧测试：

- 会话状态读写
- 多轮会话拼接
- tool 调用适配
- handoff 摘要生成
- 编排流程测试

### 11.4 跨服务验收

放在根级 `tests/` 或 `scripts/acceptance/`：

- 网关 -> agents -> order-rpc 查询订单
- 网关 -> agents -> program-rpc 咨询活动
- 网关 -> agents -> order-rpc 退款预检
- 网关 -> agents -> order-rpc 发起退款
- 系统异常触发转人工

## 12. 实施顺序

### 阶段 1：补领域接口

- 为 `order-rpc` 增加客服视图与退款预检 RPC
- 完成对应 proto、logic、server、测试

### 阶段 2：落 `agents` 服务骨架

- 新建根级 `agents/`
- 落 FastAPI 入口
- 接 Redis 会话模型
- 接 gRPC clients

### 阶段 3：迁移编排与工具层

- 保留现有 LangGraph 编排
- 将 mock MCP 工具替换为真实 tool
- 接活动/订单/退款/转人工 4 条链路

### 阶段 4：接入网关

- `gateway-api` 增加 `/agent/chat`
- `/agent/` 纳入鉴权
- 配置内网转发至 `agents`

### 阶段 5：补验收与治理

- 补全跨服务验收
- 补 trace / timeout / request id
- 评估后续 SSE 演进

## 13. 后续演进

后续可在不推翻本设计的前提下继续扩展：

- 新增 `POST /agent/chat/stream` 做 SSE
- 引入更完整的会话查询接口
- 将 gRPC target 切换为 `dns:///...`
- 扩展更多客服场景
- 接入知识库能力

## 14. 结论

正式接入方案应是：

- 对外由 `gateway-api` 提供 `/agent/chat`
- 对内由根级 Python `agents` 服务承接会话与编排
- `agents` 直连 `user-rpc`、`program-rpc`、`order-rpc`
- 多轮会话使用 Redis 持久化
- `order-rpc` 补齐客服视图与退款预检能力

这样既保留了现有 Demo 在 Agent 编排上的价值，又避免将 demo 级入口和 mock 数据直接带入生产主链路。
