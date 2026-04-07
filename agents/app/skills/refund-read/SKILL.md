---
name: refund-read
description: 用于退款相关只读查询，包括最近订单、订单详情与退款资格预览。
allowed-tools: list_user_orders get_order_detail_for_service preview_refund_order
metadata:
  domain: refund
  skill_id: refund.read
---

# refund-read

目标：在不提交退款的前提下，完成退款相关查询并给出结构化结果。

执行要求：
- 只使用当前允许的读工具完成退款相关查询。
- 可以自主决定先查最近订单、查订单详情，还是直接做退款预览。
- 绝不调用任何写工具，也不要把退款预览当成退款提交结果。

结束条件：
- 已拿到足够的订单信息和退款预览结果。
- 或已确认当前无法继续查询并返回明确原因。
