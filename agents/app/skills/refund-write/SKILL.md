---
name: refund-write
description: 用于在用户明确确认后提交退款申请。
allowed-tools: refund_order
metadata:
  domain: refund
  skill_id: refund.write
---

# refund-write

目标：在已获得用户确认的前提下提交退款申请。

执行要求：
- 只调用 `refund_order`。
- 必须使用当前任务显式提供的 `order_id`。
- 只根据工具返回结果确认是否提交成功。

结束条件：
- 已收到退款提交成功结果。
- 或后端返回退款提交失败。
