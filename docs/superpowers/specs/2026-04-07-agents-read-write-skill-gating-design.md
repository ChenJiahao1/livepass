# Agents Read/Write Skill Gating Design

## 1. 背景

当前 `agents` 的退款主链路采用“父层指定单一 skill，subagent 单步执行”的方式。该方式能控风险，但读链路被收得过死，subagent 无法在查单、查详情、退款预览之间自主编排。

票务客服和 DeerFlow / Claude Code 这类通用 agent 运行时的核心差异在于：

- 读操作可以默认可用，但需要系统侧治理
- 写操作必须显式授权，不能靠模型自行推进

因此本次调整不追求开放式多 agent 规划，而是把权限边界从“父层指定每一步 task_type”收敛为“父层控制读/写能力包”。

## 2. 目标

- 保留父层的风险控制能力
- 让 subagent 在只读能力范围内自主编排工具调用
- 将写操作统一收敛到显式确认闸之后
- 保持工具仍然通过 skill 披露，不直接裸暴露给 runtime

## 3. 核心决策

### 3.1 skill 从“单步动作”改成“能力包”

不再以 `order.list_recent`、`refund.preview` 这类“一步一个 skill”为主，而是按读写能力打包：

- `refund.read`
  - `list_user_orders`
  - `get_order_detail_for_service`
  - `preview_refund_order`
- `refund.write`
  - `refund_order`

后续其他业务域也采用同样规则：

- `xxx.read`
- `xxx.write`

### 3.2 父层不再指定读链路的每一步 task_type

父层负责：

- 理解用户诉求
- 判断当前是读任务还是写任务
- 决定当前允许的 `allowed_skills`
- 控制是否进入写操作确认闸

subagent 负责：

- 在当前 `allowed_skills` 对应的工具集合中自主选择调用顺序
- 基于工具结果返回结构化输出
- 不越权调用未授权 skill / tool

### 3.3 写接口必须通过系统硬门禁

写接口不靠 prompt 约束，必须由 runtime 明确校验：

- 当前任务 `requires_confirmation=true`
- 当前 skill 的 `access_mode=write`
- 会话中存在合法的前置上下文
  - 例如退款写操作前已有 `selected_order_id`
  - 已有最近一次退款预览结果

未满足条件时，`ToolBroker` 直接拒绝调用写工具。

### 3.4 读接口用户无感知默认可用，但要有系统治理

用户不需要感知有哪些读能力已开启，但系统侧仍要保留这些边界：

- 所有读接口只查当前 `user_id` 的可见数据
- list 类接口保留服务端 `limit` 上限
- 查询条件只接受结构化白名单字段
- 当前阶段只放开退款场景所需读能力，不一次性全域开放

## 4. TaskCard v2

`TaskCard` 从单一 `skill_id` 改成能力授权对象：

```json
{
  "task_id": "task_xxx",
  "session_id": "sess_xxx",
  "domain": "refund",
  "goal": "处理退款咨询并确认退款资格",
  "source_message": "帮我退最近那单",
  "input_slots": {
    "user_id": 3001
  },
  "required_slots": [],
  "allowed_skills": [
    "refund.read"
  ],
  "max_steps": 4,
  "risk_level": "medium",
  "requires_confirmation": false,
  "fallback_policy": "return_parent",
  "expected_output_schema": "refund_read_result_v1"
}
```

写任务示例：

```json
{
  "task_id": "task_submit_xxx",
  "session_id": "sess_xxx",
  "domain": "refund",
  "goal": "提交退款申请",
  "source_message": "确认退款",
  "input_slots": {
    "user_id": 3001,
    "order_id": "ORD-10002"
  },
  "required_slots": [
    "order_id"
  ],
  "allowed_skills": [
    "refund.write"
  ],
  "max_steps": 1,
  "risk_level": "high",
  "requires_confirmation": true,
  "fallback_policy": "handoff",
  "expected_output_schema": "refund_submit_v1"
}
```

## 5. 会话状态

退款主链路至少保留：

- `selected_order_id`
- `recent_order_candidates`
- `last_refund_preview`
- `last_task_summary`
- `last_handoff_ticket_id`

其中 `last_refund_preview` 至少包含：

- `order_id`
- `allow_refund`
- `refund_amount`
- `reject_reason`

## 6. 运行时调整

### 6.1 ParentAgent

- 退款咨询默认生成只读任务，开放 `refund.read`
- 用户明确确认退款后才生成写任务，开放 `refund.write`
- 父层不再硬编码 `refund_preview` / `order_list_recent` 这类单步 task

### 6.2 SkillRegistry / ProviderRegistry

- skill 配置保留 tools 白名单
- skill 元数据新增 `access_mode: read | write`

### 6.3 SubagentRuntime

- 支持多个 `allowed_skills`
- 将当前任务授权 skill 的工具求并集后绑定给 subagent
- 输出仍然以结构化 tool 结果为主

### 6.4 ToolBroker

- 继续做工具白名单校验
- 额外做写工具门禁校验
- 注入 `user_id / session_id / task_id`

## 7. 退款时序

### 7.1 帮我退最近那单

1. 父层识别为退款读任务
2. 生成 `allowed_skills=["refund.read"]` 的任务卡
3. subagent 自主调用：
   - `list_user_orders`
   - `get_order_detail_for_service`（如需要）
   - `preview_refund_order`
4. subagent 返回最近订单和退款预览结果
5. 父层回复用户是否确认退款

### 7.2 用户确认退款

1. 父层检查会话中已有 `selected_order_id` 和 `last_refund_preview`
2. 生成 `allowed_skills=["refund.write"]` 的写任务卡
3. subagent 调用 `refund_order`
4. ToolBroker 校验确认态通过后放行
5. 返回退款提交结果

## 8. 测试重点

- `TaskCard` 支持 `allowed_skills` 与 `requires_confirmation`
- `SubagentRuntime` 能绑定多个读工具
- `ToolBroker` 未确认时阻断写工具
- 父层在退款咨询与确认退款之间正确切换读/写任务
- 会话状态正确保存最近一次退款预览

## 9. 非目标

- 本期不接入 FAQ
- 本期不扩展到全域读能力
- 本期不引入开放式 planner
- 本期不改动 Go provider 的接口语义，仅调整 Python 编排与 gating
