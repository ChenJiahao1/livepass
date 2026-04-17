# agents Run Contract V2 最终态设计

**日期：** 2026-04-17

## 目标

为 `/home/chenjiahao/code/project/damai-go/agents` 设计一套面向开发阶段最终态的后端契约与存储模型。

本设计明确采用“最佳重构、无兼容层”原则：

- 对外正式契约切换为 `Thread / Message / Run / ToolCall`
- 消息内容统一使用 `content[]`，不再使用 `parts`
- SSE 正式收敛为 `Run` 事件流，不透传 LangGraph 原始 chunk
- MySQL 读模型与外部 DTO 字段统一命名，不保留历史过渡字段
- Redis checkpoint 继续只承载 LangGraph 内部恢复状态，不作为外部资源来源

本次设计目标包括：

- 从后端资源模型出发定义稳定契约，而不是从前端渲染结构反推
- 建立 `threadId -> runId -> outputMessageId -> toolCallId` 的清晰关系
- 支持流式文本输出、人工中断、恢复执行、取消执行与事件续接
- 为后续 `image / file` 内容类型预留扩展位，但当前只实现 `text`
- 明确 MySQL、Redis、SSE 之间的职责边界

本设计明确不做：

- 不保留 `parts` 字段兼容层
- 不保留 `assistantMessageId`、`acceptedMessage` 等旧命名
- 不向前端暴露 LangGraph `MessagesState`、`interrupt` 原始结构或 checkpoint 内容
- 不把前端 SDK 的消息 block 结构定义为后端主协议

## 背景

当前 `agents` 已经具备：

- `POST /agent/runs`
- `GET /agent/runs/{runId}`
- `GET /agent/runs/{runId}/events`
- `POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume`
- `POST /agent/runs/{runId}/cancel`

但当前模型仍有几个明显问题：

1. `Message` 对外内容字段仍叫 `parts`
2. `CreateRunRequest` 的写入模型是 `input.parts`
3. `Run` 仍使用 `assistantMessageId` 这类偏 UI/角色命名
4. MySQL 内部字段与外部 API 字段未完全统一
5. SSE 事件命名偏“投影内部实现”，语义不够最终态

这些问题会带来长期成本：

- API 文档表达不够清晰
- 前后端都要维护“消息真实语义”和“SDK block 结构”两套心智模型
- 数据库存储名词与 DTO 名词错位
- 事件模型不够利于长期审计和前端适配

因此本次设计直接进入最终态：

- 后端只暴露自己的资源模型
- 前端如果需要某种 UI SDK 结构，自己在适配层做转换
- LangGraph 继续作为内部运行时，而不是外部契约的一部分

## 设计原则

### 1. 以 Run 为唯一执行资源

一次用户输入触发一次 `Run`。

所有与执行过程有关的状态都绑定到 `Run`：

- 流式输出
- 工具调用
- 人工中断
- 恢复执行
- 取消执行
- 失败
- 事件回放

`Thread` 负责历史容器，`Message` 负责展示快照，`Run` 才负责执行语义。

### 2. 写入契约最小化，读取契约富表达

创建运行时，请求体只表达“本轮新增输入”：

- 属于哪个 `thread`
- 本轮输入内容是什么

查询与 SSE 才表达：

- 消息完整快照
- 运行过程事件
- 工具调用状态
- interrupt 信息

因此：

- 写入接口不上传整段 `messages[]`
- 写入接口也不暴露前端渲染块概念
- 读取接口和事件流负责把后端资源完整表达出来

### 3. 内容类型用 `type` 判别，不用 `mimeType`

消息内容块统一定义为：

```ts
type MessageContent =
  | { type: "text"; text: string }
  | { type: "image"; url: string; alt?: string }
  | { type: "file"; fileId?: string; url: string; name: string; size?: number }
```

原则：

- `type` 是协议层主判别字段
- 不引入 `mimeType`
- 文本默认由前端按 Markdown 渲染
- 当前后端只正式支持 `text`
- `image / file` 仅作为最终态协议扩展位

原因：

- `mimeType` 在当前阶段不会参与 UI 主分支判断
- 当前也没有完整的上传、下载、预览、对象存储协议
- 提前引入 `mimeType` 只会增加协议复杂度

### 4. SSE 是后端投影事件流，不是 LangGraph 原始 stream

LangGraph 内部可以继续使用：

- `messages`
- `updates`
- `custom`
- `interrupt()`
- `Command(resume=...)`

但对前端公开的 SSE 必须是稳定业务事件：

- `run.created`
- `message.created`
- `message.delta`
- `tool_call.waiting_human`
- `message.completed`
- `run.completed`

前端不需要理解：

- graph 节点名
- stream mode
- chunk 原始结构
- checkpoint 数据结构

### 5. MySQL 读模型与外部 DTO 字段保持一致

既然开发阶段选择最佳重构，就不再接受：

- DTO 叫 `content`
- DB 还叫 `parts_json`

也不再接受：

- DTO 叫 `outputMessageId`
- DB 还叫 `assistant_message_id`

原则是：

- 外部资源命名是什么，MySQL 读模型字段就尽量是什么
- repository 和 service 层不长期保留历史名词翻译

### 6. Redis checkpoint 只服务运行恢复

Redis checkpoint 保持内部用途：

- `threadId` 维度恢复 LangGraph 执行
- `resume` 时从中断点继续推进

它不负责：

- 对外消息历史查询
- SSE 回放
- 线程列表
- run 状态查询

对外资源与事件的权威来源是 MySQL。

## 方案对比

### 方案 A：保留 `parts`，只局部重命名

优点：

- 改动最小
- 短期可继续沿用已有测试

缺点：

- 后端仍被前端 block 模型污染
- 存储层和协议层名词继续错位
- 长期要维护“后端语义”和“前端 block 语义”两套表达

不推荐。

### 方案 B：对外完全改成 `messages[]` 输入

优点：

- 表面上更接近部分 LLM API 的 stateless 风格

缺点：

- 与当前 `thread / run / checkpoint / resume` 的有状态模型冲突
- 前端与后端会形成双重事实源
- interrupt、续跑、去重、幂等都会更复杂

不推荐。

### 方案 C：Run 中心 + `content[]` 消息模型 + SSE 事件流

优点：

- 后端边界清晰
- 与 LangGraph 有状态执行模型一致
- 存储、API、事件流语义统一
- 前端只需要适配层，不反向影响后端设计

缺点：

- 需要一次性改 DTO、repository、SQL、测试

这是推荐方案。

## 资源模型

## Thread

`Thread` 是历史会话容器，不是执行过程本身。

```ts
type Thread = {
  id: string
  title: string
  status: "active" | "archived"
  createdAt: string
  updatedAt: string
  lastMessageAt: string | null
  activeRunId: string | null
  metadata: Record<string, unknown>
}
```

字段语义：

- `id`：线程 ID
- `title`：线程标题，默认由首条用户文本截断生成
- `status`：线程状态
- `activeRunId`：当前未终态运行 ID，没有则为 `null`
- `lastMessageAt`：最后消息时间，用于排序
- `metadata`：轻量扩展字段

约束：

- `threadId` 是对外线程主键
- `threadId` 同时作为 LangGraph `configurable.thread_id`
- `threadId` 同时也是 Redis ownership / checkpoint 的公共上下文标识

## MessageContent

```ts
type MessageContent =
  | { type: "text"; text: string }
  | { type: "image"; url: string; alt?: string }
  | { type: "file"; fileId?: string; url: string; name: string; size?: number }
```

说明：

- `text`：文本内容，前端默认按 Markdown 渲染
- `image`：图片内容，当前阶段预留
- `file`：文件内容，当前阶段预留

当前后端实现约束：

- `POST /agent/runs` 只接受 `text`
- 若收到 `image/file`，返回 `400 unsupported_content_type`

## Message

`Message` 是可展示消息的完整资源快照。

```ts
type Message = {
  id: string
  threadId: string
  runId: string | null
  role: "user" | "assistant"
  status: "completed" | "streaming" | "failed" | "cancelled"
  content: MessageContent[]
  createdAt: string
  updatedAt: string
  metadata: Record<string, unknown>
}
```

说明：

- `content` 是完整内容快照，不是增量
- 用户消息通常创建即 `completed`
- assistant 输出消息可以先创建为空 `content: []` 且 `status: "streaming"`
- `updatedAt` 体现消息最终状态变更时间

## Run

`Run` 是唯一执行资源。

```ts
type Run = {
  id: string
  threadId: string
  status: "queued" | "running" | "requires_action" | "completed" | "failed" | "cancelled"
  triggerMessageId: string
  outputMessageId: string
  startedAt: string
  completedAt: string | null
  error: RunError | null
  metadata: Record<string, unknown>
}
```

说明：

- `triggerMessageId`：本轮用户输入消息
- `outputMessageId`：本轮 assistant 输出消息
- 旧的 `assistantMessageId` 正式移除
- `outputMessageId` 比 `assistantMessageId` 更符合资源语义

## RunError

```ts
type RunError = {
  code: string
  message: string
  details?: Record<string, unknown>
}
```

## ToolCall

`ToolCall` 是 run 内正式资源，不再只是临时事件。

```ts
type ToolCall = {
  id: string
  threadId: string
  runId: string
  messageId: string
  name: string
  status: "running" | "waiting_human" | "completed" | "failed" | "cancelled"
  input: Record<string, unknown>
  output: Record<string, unknown> | null
  humanRequest: HumanRequest | null
  createdAt: string
  updatedAt: string
  completedAt: string | null
  metadata: Record<string, unknown>
}
```

## HumanRequest

```ts
type HumanRequest = {
  kind: "approval" | "input"
  title: string
  description?: string
  allowedActions: Array<"approve" | "reject" | "edit">
}
```

## HTTP 契约

## 创建 Thread

```http
POST /agent/threads
```

Request:

```json
{
  "title": null,
  "metadata": {}
}
```

Response:

```json
{
  "thread": {
    "id": "thr_123",
    "title": "新会话",
    "status": "active",
    "createdAt": "2026-04-17T09:00:00Z",
    "updatedAt": "2026-04-17T09:00:00Z",
    "lastMessageAt": null,
    "activeRunId": null,
    "metadata": {}
  }
}
```

## 创建 Run

```http
POST /agent/runs
```

Request:

```json
{
  "threadId": "thr_123",
  "input": {
    "content": [
      {
        "type": "text",
        "text": "帮我查一下订单"
      }
    ]
  },
  "metadata": {}
}
```

请求原则：

- 只表达“当前线程上的本轮新增输入”
- 不上传整个 `messages[]`
- 当前阶段只接受 `text`

Response:

```json
{
  "thread": {
    "id": "thr_123",
    "title": "帮我查一下订单",
    "status": "active",
    "createdAt": "2026-04-17T09:00:00Z",
    "updatedAt": "2026-04-17T09:01:00Z",
    "lastMessageAt": "2026-04-17T09:01:00Z",
    "activeRunId": "run_123",
    "metadata": {}
  },
  "run": {
    "id": "run_123",
    "threadId": "thr_123",
    "status": "queued",
    "triggerMessageId": "msg_123",
    "outputMessageId": "msg_124",
    "startedAt": "2026-04-17T09:01:00Z",
    "completedAt": null,
    "error": null,
    "metadata": {}
  },
  "inputMessage": {
    "id": "msg_123",
    "threadId": "thr_123",
    "runId": "run_123",
    "role": "user",
    "status": "completed",
    "content": [
      {
        "type": "text",
        "text": "帮我查一下订单"
      }
    ],
    "createdAt": "2026-04-17T09:01:00Z",
    "updatedAt": "2026-04-17T09:01:00Z",
    "metadata": {}
  },
  "outputMessage": {
    "id": "msg_124",
    "threadId": "thr_123",
    "runId": "run_123",
    "role": "assistant",
    "status": "streaming",
    "content": [],
    "createdAt": "2026-04-17T09:01:00Z",
    "updatedAt": "2026-04-17T09:01:00Z",
    "metadata": {}
  }
}
```

命名调整：

- `acceptedMessage` -> `inputMessage`
- `assistantMessage` -> `outputMessage`
- `assistantMessageId` -> `outputMessageId`

## 查询 Thread 消息

```http
GET /agent/threads/{threadId}/messages
```

Response:

```json
{
  "messages": [
    {
      "id": "msg_123",
      "threadId": "thr_123",
      "runId": "run_123",
      "role": "user",
      "status": "completed",
      "content": [
        {
          "type": "text",
          "text": "帮我查一下订单"
        }
      ],
      "createdAt": "2026-04-17T09:01:00Z",
      "updatedAt": "2026-04-17T09:01:00Z",
      "metadata": {}
    },
    {
      "id": "msg_124",
      "threadId": "thr_123",
      "runId": "run_123",
      "role": "assistant",
      "status": "completed",
      "content": [
        {
          "type": "text",
          "text": "已为你查询到 2 条订单。"
        }
      ],
      "createdAt": "2026-04-17T09:01:00Z",
      "updatedAt": "2026-04-17T09:01:08Z",
      "metadata": {}
    }
  ],
  "nextCursor": null
}
```

## 查询 Run

```http
GET /agent/runs/{runId}
```

Response:

```json
{
  "run": {
    "id": "run_123",
    "threadId": "thr_123",
    "status": "requires_action",
    "triggerMessageId": "msg_123",
    "outputMessageId": "msg_124",
    "startedAt": "2026-04-17T09:01:00Z",
    "completedAt": null,
    "error": null,
    "metadata": {}
  },
  "outputMessage": {
    "id": "msg_124",
    "threadId": "thr_123",
    "runId": "run_123",
    "role": "assistant",
    "status": "streaming",
    "content": [
      {
        "type": "text",
        "text": "我需要你确认是否继续退款。"
      }
    ],
    "createdAt": "2026-04-17T09:01:00Z",
    "updatedAt": "2026-04-17T09:01:03Z",
    "metadata": {}
  },
  "activeToolCall": {
    "id": "tc_123",
    "threadId": "thr_123",
    "runId": "run_123",
    "messageId": "msg_124",
    "name": "refund_apply",
    "status": "waiting_human",
    "input": {
      "orderId": "ord_001"
    },
    "output": null,
    "humanRequest": {
      "kind": "approval",
      "title": "请确认是否发起退款",
      "description": "退款后将释放票务资源",
      "allowedActions": ["approve", "reject", "edit"]
    },
    "createdAt": "2026-04-17T09:01:03Z",
    "updatedAt": "2026-04-17T09:01:03Z",
    "completedAt": null,
    "metadata": {}
  }
}
```

说明：

- `GET /agent/runs/{runId}` 不只返回 `run`
- 同时返回当前 `outputMessage` 与 `activeToolCall`
- 这样前端在断线恢复或页面刷新后可以直接重建运行态 UI

## 恢复 ToolCall

```http
POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume
```

Request:

```json
{
  "action": "approve",
  "reason": null,
  "values": {}
}
```

约束：

- `action` 只允许 `approve | reject | edit`
- 不支持 `respond`
- `run.status` 必须是 `requires_action`
- `toolCall.status` 必须是 `waiting_human`
- `resume` 恢复同一个 `run`，不是新建 `run`

Response:

```json
{
  "run": {
    "id": "run_123",
    "threadId": "thr_123",
    "status": "running",
    "triggerMessageId": "msg_123",
    "outputMessageId": "msg_124",
    "startedAt": "2026-04-17T09:01:00Z",
    "completedAt": null,
    "error": null,
    "metadata": {}
  }
}
```

## 取消 Run

```http
POST /agent/runs/{runId}/cancel
```

行为：

- 若当前 run 为 `queued / running / requires_action`，则允许取消
- 若当前有 `waiting_human` 的 tool call，则一并置为 `cancelled`
- assistant 输出消息置为 `cancelled`
- run 终态置为 `cancelled`

## SSE 契约

## 连接方式

```http
GET /agent/runs/{runId}/events?after=0
```

SSE 外壳：

```text
id: 1
event: agent.run.event
data: {...json...}
```

约束：

- `id` 等于 `sequenceNo`
- 前端用 `runId + sequenceNo` 去重
- `after` 表示仅返回 `sequenceNo > after` 的事件
- 当前可以继续保留 `after` 机制
- 后续如需增强，可兼容标准 `Last-Event-ID`

## 统一事件外壳

```ts
type RunEventEnvelope = {
  type: string
  sequenceNo: number
  threadId: string
  runId: string
  timestamp: string
}
```

## 事件定义

### run.created

```json
{
  "type": "run.created",
  "sequenceNo": 1,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:00Z",
  "run": {
    "id": "run_123",
    "status": "queued"
  }
}
```

### message.created

```json
{
  "type": "message.created",
  "sequenceNo": 2,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:00Z",
  "message": {
    "id": "msg_124",
    "role": "assistant",
    "status": "streaming",
    "content": []
  }
}
```

### run.updated

```json
{
  "type": "run.updated",
  "sequenceNo": 3,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:00Z",
  "run": {
    "id": "run_123",
    "status": "running"
  }
}
```

### message.delta

```json
{
  "type": "message.delta",
  "sequenceNo": 4,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:01Z",
  "messageId": "msg_124",
  "delta": {
    "type": "text",
    "text": "正在"
  }
}
```

说明：

- 当前 `message.delta` 只支持 `text`
- 前端收到后把文本追加到目标消息
- 最终以 `message.completed` 的完整 `content` 快照作为收口

### tool_call.created

```json
{
  "type": "tool_call.created",
  "sequenceNo": 8,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:03Z",
  "toolCall": {
    "id": "tc_123",
    "messageId": "msg_124",
    "name": "refund_apply",
    "status": "running",
    "input": {
      "orderId": "ord_001"
    }
  }
}
```

### tool_call.waiting_human

```json
{
  "type": "tool_call.waiting_human",
  "sequenceNo": 9,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:03Z",
  "toolCall": {
    "id": "tc_123",
    "messageId": "msg_124",
    "name": "refund_apply",
    "status": "waiting_human",
    "input": {
      "orderId": "ord_001"
    },
    "humanRequest": {
      "kind": "approval",
      "title": "请确认是否发起退款",
      "description": "退款后将释放票务资源",
      "allowedActions": ["approve", "reject", "edit"]
    }
  }
}
```

说明：

- 这是前端识别 interrupt 的核心事件
- 前端不应把 SSE 结束误判为网络错误
- `run.updated(status=requires_action)` 只负责资源状态同步

### run.updated requires_action

```json
{
  "type": "run.updated",
  "sequenceNo": 10,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:01:03Z",
  "run": {
    "id": "run_123",
    "status": "requires_action"
  }
}
```

### tool_call.completed

```json
{
  "type": "tool_call.completed",
  "sequenceNo": 11,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:00Z",
  "toolCall": {
    "id": "tc_123",
    "status": "completed",
    "output": {
      "action": "approve"
    }
  }
}
```

### message.completed

```json
{
  "type": "message.completed",
  "sequenceNo": 20,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "message": {
    "id": "msg_124",
    "status": "completed",
    "content": [
      {
        "type": "text",
        "text": "退款申请已提交。"
      }
    ]
  }
}
```

### message.failed

```json
{
  "type": "message.failed",
  "sequenceNo": 20,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "message": {
    "id": "msg_124",
    "status": "failed",
    "content": [
      {
        "type": "text",
        "text": "运行失败，请稍后重试。"
      }
    ]
  }
}
```

### message.cancelled

```json
{
  "type": "message.cancelled",
  "sequenceNo": 20,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "message": {
    "id": "msg_124",
    "status": "cancelled"
  }
}
```

### run.completed

```json
{
  "type": "run.completed",
  "sequenceNo": 21,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "run": {
    "id": "run_123",
    "status": "completed"
  }
}
```

### run.failed

```json
{
  "type": "run.failed",
  "sequenceNo": 21,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "run": {
    "id": "run_123",
    "status": "failed"
  },
  "error": {
    "code": "langgraph_runtime_error",
    "message": "运行失败"
  }
}
```

### run.cancelled

```json
{
  "type": "run.cancelled",
  "sequenceNo": 21,
  "threadId": "thr_123",
  "runId": "run_123",
  "timestamp": "2026-04-17T09:02:10Z",
  "run": {
    "id": "run_123",
    "status": "cancelled"
  }
}
```

## SSE 生命周期

### 正常完成

1. `run.created`
2. `message.created`
3. `run.updated(running)`
4. `message.delta` 多次
5. `message.completed`
6. `run.completed`
7. SSE 结束

### 进入人工确认

1. `run.created`
2. `message.created`
3. `run.updated(running)`
4. `message.delta` 可选
5. `tool_call.created`
6. `tool_call.waiting_human`
7. `run.updated(requires_action)`
8. SSE 结束

### 人工确认后恢复

1. 前端提交 `POST /resume`
2. 后端恢复同一个 `runId`
3. 前端重新连接：

```http
GET /agent/runs/{runId}/events?after=<lastSequenceNo>
```

4. 后端从同一个 checkpoint 恢复执行
5. 后续继续输出 `tool_call.completed / message.delta / message.completed / run.completed`

## 存储模型

## 总体职责边界

| 存储 | 用途 | 对前端暴露 |
|---|---|---|
| `agent_threads` | 线程读模型 | 是 |
| `agent_messages` | 消息快照 | 是 |
| `agent_runs` | 运行状态 | 是 |
| `agent_tool_calls` | 工具调用状态 | 是 |
| `agent_run_events` | SSE 回放与续接 | 是，通过 SSE |
| Redis checkpoint | LangGraph 恢复执行 | 否 |

## agent_messages

建议字段：

```sql
CREATE TABLE agent_messages (
  id VARCHAR(64) PRIMARY KEY,
  thread_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  role VARCHAR(32) NOT NULL,
  content_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  run_id VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  metadata_json JSON NOT NULL
);
```

说明：

- `parts_json` 正式改为 `content_json`
- 存储完整 `MessageContent[]`
- 用户消息与输出消息都通过 `run_id` 归属到同一个 `run`

## agent_runs

建议字段：

```sql
CREATE TABLE agent_runs (
  id VARCHAR(64) PRIMARY KEY,
  thread_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  trigger_message_id VARCHAR(64) NOT NULL,
  output_message_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  started_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  error_json JSON NULL,
  metadata_json JSON NOT NULL
);
```

说明：

- `assistant_message_id` 正式改为 `output_message_id`
- `output_message_id` 是 run 输出锚点

## agent_tool_calls

建议字段：

```sql
CREATE TABLE agent_tool_calls (
  id VARCHAR(64) PRIMARY KEY,
  thread_id VARCHAR(64) NOT NULL,
  run_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  message_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL,
  input_json JSON NOT NULL,
  output_json JSON NULL,
  human_request_json JSON NULL,
  error_json JSON NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  metadata_json JSON NOT NULL
);
```

说明：

- `tool_name` 改为 `name`
- `arguments` 改为 `input_json`
- `result` 改为 `output_json`
- `human_request_json` 独立保存，便于直接查询当前待人工确认内容

## agent_run_events

建议字段：

```sql
CREATE TABLE agent_run_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  run_id VARCHAR(64) NOT NULL,
  thread_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  sequence_no BIGINT NOT NULL,
  type VARCHAR(128) NOT NULL,
  message_id VARCHAR(64) NULL,
  tool_call_id VARCHAR(64) NULL,
  payload_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_run_sequence (run_id, sequence_no)
);
```

说明：

- `agent_run_events` 是 append-only 事件日志
- SSE 回放只读取这里
- 不从 checkpoint 反推对外事件

## Redis checkpoint

保持现状：

- checkpoint 继续使用 `threadId`
- 只用于 LangGraph 恢复执行
- 不用于生成消息历史
- 不用于 SSE 补读
- 不直接暴露给前端

## LangGraph 内部映射

后端进入 LangGraph 前，将当前输入映射为内部运行时 payload：

```python
payload = {
    "messages": [
        {
            "role": "user",
            "content": user_text,
        }
    ]
}
config = {
    "configurable": {
        "thread_id": thread_id,
    }
}
```

说明：

- 对外 API 使用 `content[]`
- 进入 LangGraph 前，后端把 `content[]` 转成运行时 `messages`
- 当前只支持 `text`，因此可以提取并合并文本
- 未来支持 `image/file` 时，再决定如何映射为多模态输入或工具输入

后端消费 LangGraph stream 时：

| LangGraph 内部输出 | 对外事件 |
|---|---|
| `messages` token | `message.delta` |
| `updates` run 状态 | `run.updated` |
| `updates` message 状态 | `message.completed / failed / cancelled` |
| `interrupt()` | `tool_call.waiting_human` + `run.updated(requires_action)` |
| `custom` tool progress | `tool_call.progress` 或 `run.progress` |

原则：

- 后端消费 LangGraph 原始输出
- 后端投影成稳定对外事件
- 前端不直接消费 LangGraph 原始结构

## 状态流转

## Run

```text
queued
  -> running
  -> requires_action
  -> running
  -> completed

queued/running/requires_action
  -> cancelled

queued/running/requires_action
  -> failed
```

## Message

```text
streaming
  -> completed

streaming
  -> failed

streaming
  -> cancelled
```

用户消息通常为：

```text
completed
```

## ToolCall

```text
running
  -> waiting_human
  -> completed

running/waiting_human
  -> failed

running/waiting_human
  -> cancelled
```

## 重构范围

需要同步重构的模块包括：

- `agents/app/api/schemas.py`
- `agents/app/api/routes.py`
- `agents/app/messages/models.py`
- `agents/app/messages/service.py`
- `agents/app/messages/repository.py`
- `agents/app/runs/models.py`
- `agents/app/runs/service.py`
- `agents/app/runs/repository.py`
- `agents/app/runs/event_projector.py`
- `agents/app/runs/stream_service.py`
- `agents/app/runs/tool_call_repository.py`
- SQL schema
- 契约测试、repository 测试、stream 测试、resume/cancel 测试

具体调整包括：

- `parts` -> `content`
- `acceptedMessage` -> `inputMessage`
- `assistantMessage` -> `outputMessage`
- `assistantMessageId` -> `outputMessageId`
- `parts_json` -> `content_json`
- `assistant_message_id` -> `output_message_id`
- `tool_name` -> `name`
- `arguments` -> `input`
- `result` -> `output`
- `tool_call.updated(waiting_human)` -> `tool_call.waiting_human`

## 分阶段实施建议

### 第一阶段：契约与命名收敛

- 完成 DTO 改名
- 完成事件名调整
- 完成 service/repository 字段改名
- 保持当前只支持 `text`

### 第二阶段：存储 schema 清理

- 调整 MySQL 表结构
- 清理历史字段名
- 更新 repository 与测试

### 第三阶段：image/file 扩展位启用

- 开放 `MessageContent` 的 `image/file`
- 增加上传/文件引用协议
- 明确前端预览和下载行为

## 最终推荐结论

本次最终态设计确定以下核心决策：

- 消息内容统一使用 `content[]`
- `MessageContent` 固定为：

```ts
type MessageContent =
  | { type: "text"; text: string }
  | { type: "image"; url: string; alt?: string }
  | { type: "file"; fileId?: string; url: string; name: string; size?: number }
```

- 不使用 `parts`
- 不使用 `mimeType`
- 文本默认按 Markdown 渲染
- `Run` 使用 `outputMessageId`
- `POST /agent/runs` 返回 `inputMessage` 与 `outputMessage`
- SSE 以 `tool_call.waiting_human` 作为 interrupt 主识别事件
- MySQL 与外部 DTO 命名统一
- Redis checkpoint 继续只作为 LangGraph 内部恢复状态

这套设计比当前契约更清晰，更符合后端资源模型，也更适合作为开发阶段直接落地的最终态方案。
