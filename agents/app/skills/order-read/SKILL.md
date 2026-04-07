---
name: order-read
description: 用于订单只读查询，包括最近订单和指定订单详情。
allowed-tools: list_user_orders get_order_detail_for_service
metadata:
  domain: order
  skill_id: order.read
---

# order-read

目标：在不产生副作用的前提下完成订单查询。

执行要求：
- 只使用当前允许的读工具完成订单查询。
- 可以先查最近订单，再根据需要补查订单详情。
- 不要生成不存在的订单，也不要推断后端未返回的事实。

结束条件：
- 已返回可供父层继续处理的订单信息。
- 或已确认当前用户无可用订单信息。
