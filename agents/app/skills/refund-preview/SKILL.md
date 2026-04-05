---
name: refund-preview
description: 用于预览订单退款资格、预计退款金额和拒绝原因。
allowed-tools: preview_refund_order
metadata:
  domain: refund
  skill_id: refund.preview
---

# refund-preview

用于预览订单退款资格、预计退款金额和拒绝原因。

目标：确认指定订单当前是否可退款，并返回预计退款金额、退款比例和拒绝原因。

执行要求：
- 只调用 `preview_refund_order`。
- 必须基于工具结果给出结论，不要自己推导退款金额。
- 若不可退款，优先返回后端提供的拒绝原因。

结束条件：
- 已确认订单可退款并拿到退款预估。
- 或已确认当前不可退款并拿到拒绝原因。
