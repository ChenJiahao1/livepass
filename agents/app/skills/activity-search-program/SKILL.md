---
name: activity-search-program
description: 用于根据节目 ID 查询节目信息，并返回演出时间等基础详情。
allowed-tools: search_programs
metadata:
  domain: activity
  skill_id: activity.search_program
---

# activity-search-program

目标：根据给定节目 ID 查询节目基础信息，并返回标题和演出时间。

执行要求：
- 只调用 `search_programs`。
- 必须使用传入的 `program_id`。
- 只基于工具返回结果描述节目，不要补造节目数据。

结束条件：
- 已拿到节目信息。
- 或后端返回该节目不存在。
