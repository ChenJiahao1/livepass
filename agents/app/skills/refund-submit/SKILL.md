---
name: refund-submit
description: 用于提交订单退款申请，并返回退款受理结果。
allowed-tools: refund_order
metadata:
  domain: refund
  skill_id: refund.submit
---

# refund-submit

目标：提交指定订单的退款申请，并返回后端确认的受理结果。

执行要求：
- 只调用 `refund_order`。
- 必须使用传入的 `order_id`。
- 只根据工具结果确认是否已提交，不要将预览结果当作提交结果。

结束条件：
- 已收到退款提交成功结果。
- 或后端返回退款提交失败。
