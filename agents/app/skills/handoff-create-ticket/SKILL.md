---
name: handoff-create-ticket
description: 用于创建人工客服工单，并返回工单编号或排队结果。
allowed-tools: create_handoff_ticket
metadata:
  domain: handoff
  skill_id: handoff.create_ticket
---

# handoff-create-ticket

目标：在自动处理失败或需要人工介入时创建人工客服工单。

执行要求：
- 只调用 `create_handoff_ticket`。
- 若已有用户消息上下文，优先保留原始诉求，不要自行扩写。
- 返回工单号或排队状态，不要伪造人工已处理完成的结论。

结束条件：
- 已拿到工单编号。
- 或已拿到后端返回的人工排队结果。
