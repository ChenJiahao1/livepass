---
name: order-list-recent
description: 用于查询当前用户最近订单列表，并返回稳定排序结果，供父层选择最近订单。
allowed-tools: list_user_orders
metadata:
  domain: order
  skill_id: order.list_recent
---

# order-list-recent

目标：查询当前用户最近订单，并输出可供后续订单详情或退款流程复用的稳定结果。

执行要求：
- 只调用 `list_user_orders`。
- 返回结果必须保持后端提供的稳定顺序，不要自行重排。
- 如果无订单，明确返回空列表，不要编造默认订单。

结束条件：
- 已拿到最近订单列表。
- 或后端明确返回当前用户无订单。
