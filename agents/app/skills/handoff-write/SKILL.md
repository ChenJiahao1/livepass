---
name: handoff-write
description: 用于创建人工客服工单。
allowed-tools: create_handoff_ticket
metadata:
  domain: handoff
  skill_id: handoff.write
---

# handoff-write

目标：创建人工客服工单并返回工单结果。

执行要求：
- 只调用 `create_handoff_ticket`。
- 不要假设工单已创建，必须以工具结果为准。

结束条件：
- 已返回工单编号。
- 或已明确创建失败。
