---
name: order-get-detail
description: 用于查询单个订单详情，并输出面向客服服务视图的标准化结果。
allowed-tools: get_order_detail_for_service
metadata:
  domain: order
  skill_id: order.get_detail
---

# order-get-detail

目标：查询指定订单详情，并返回适合客服回复和后续业务判断的订单数据。

执行要求：
- 只调用 `get_order_detail_for_service`。
- 必须使用传入的 `order_id`，不要自行猜测或替换订单号。
- 如果订单不存在或不可见，返回失败事实，不要补造订单内容。

结束条件：
- 已获取订单详情。
- 或后端返回该订单不可查询。
