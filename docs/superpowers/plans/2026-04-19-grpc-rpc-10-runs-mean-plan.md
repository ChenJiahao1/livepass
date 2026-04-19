# gRPC `PerfCreateOrder` 直压 10 次取均值方案

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 连续执行 10 次 `order-rpc/PerfCreateOrder` 直压实验，得到更稳定的 RPC 返回性能均值，并同时验证最终异步冻座与订单数据是否正确收敛。

**Architecture:** 保持 `showTimeId`、`ticketCategoryId`、用户数、座位数、VUS、iterations 全部固定，每轮都重建数据库并重新准备数据集，避免把实验变量混在一起。结果拆成两层：第一层看同步 RPC 返回性能，第二层看异步消费最终是否收敛到 2000 冻座 / 2000 订单相关行。

**Tech Stack:** bash, k6, gRPC, Go, go-zero, MySQL, Redis, Kafka, jq

---

## 指标定义

- `client_qps`
  - 定义：`成功/失败总请求数 ÷ 客户端整轮耗时`
  - 来源：`client_qps.json.qpsByClientElapsed`
  - 口径：从脚本记录的 `startEpoch` 到 `endEpoch`
  - 含义：更接近“这一轮压测从客户端视角的真实吞吐”
  - 特点：包含 k6 初始化、连接、请求发送、等待响应、脚本结束的整体时间

- `window_qps`
  - 定义：`服务端完成请求数 ÷ 服务端响应完成时间窗`
  - 来源：`order_rpc_qps.json.qpsByResponseWindow`
  - 口径：`第一条 create perf 日志时间` 到 `最后一条 create perf 日志时间`
  - 含义：更像“服务端完成响应的集中回写速率”
  - 特点：对日志时间窗非常敏感，容易被短时间集中完成放大，不适合作为主结论

- 指标解释
  - 如果 `client_qps` 波动大，说明整轮压测从客户端视角就不稳定，可能受 k6 本机 CPU、容器调度、网络抖动影响
  - 如果 `window_qps` 波动更大，说明服务端响应完成时间非常集中或非常分散，这个值更容易被“完成时刻扎堆”放大或缩小
  - 因此最终结论以 `client_qps + avg/p95/p99 + final_state` 为主，`window_qps` 只做辅助观察

## 固定实验口径

- `SHOW_TIME_ID=30001`
- `TICKET_CATEGORY_ID=40001`
- `USER_COUNT=2000`
- `SEAT_COUNT=2000`
- `MIN_TICKET_COUNT=1`
- `MAX_TICKET_COUNT=1`
- `VUS=2000`
- `ITERATIONS=2000`
- 使用同一份 `gRPC PerfCreateOrder` 直压脚本，不切换到 HTTP

## 执行步骤

### Task 1: 跑 1 次预热

**Files:**
- Use: `scripts/perf/run_rush_rpc_perf.sh`
- Output: `tmp/perf/results/`

- [ ] 执行 1 次预热，确认链路可用，但预热结果不纳入均值
- [ ] 确认 `summary.json` 存在且 `createSuccessCount=2000`
- [ ] 确认 `final_state.json` 最终收敛到 2000

### Task 2: 连续跑 10 次正式实验

**Files:**
- Use: `scripts/perf/run_rush_rpc_perf.sh`
- Output: `tmp/perf/results/rush-rpc-*`

- [ ] 连续执行 10 次 `gRPC PerfCreateOrder` 直压
- [ ] 每一轮都保留独立结果目录
- [ ] 每一轮都记录以下字段：
  - `run_id`
  - `client_qps`
  - `window_qps`
  - `avg_ms`
  - `p95_ms`
  - `p99_ms`
  - `final_state_ok`

### Task 3: 校验每轮最终收敛

**Files:**
- Check: `tmp/perf/results/<run-id>/final_state.json`

- [ ] 每轮都确认以下字段最终为 `2000`
  - `seatFrozen`
  - `seatOccupied`
  - `orderRows`
  - `orderTicketRows`
  - `userGuardRows`
  - `viewerGuardRows`
  - `seatGuardRows`
- [ ] 若任意一轮未收敛，单独标记该轮，不纳入“业务正确”结论

### Task 4: 做 10 次汇总

**Files:**
- Aggregate: `tmp/perf/results/`
- Create: `docs/perf/2026-04-19-grpc-rpc-10-runs-summary.md`

- [ ] 汇总 `client_qps` 的均值、中位数、最小值、最大值
- [ ] 汇总 `window_qps` 的均值、中位数、最小值、最大值
- [ ] 汇总 `avg/p95/p99` 的均值
- [ ] 汇总 `final_state_ok`，输出是否为 `10/10`

## 结果判读规则

- 主指标
  - `client_qps`
  - `avg`
  - `p95`
  - `p99`
  - `final_state_ok`

- 辅助指标
  - `window_qps`

- 判读方法
  - 如果 `10/10` 都收敛到 2000，说明该实验下 `gRPC PerfCreateOrder` 不仅返回成功，而且最终业务正确
  - 如果 `client_qps` 相对稳定，但 `window_qps` 上下波动很大，说明 `window_qps` 受响应完成聚集效应影响明显，不应作为主 KPI
  - 如果 `client_qps` 和 `p95/p99` 同时波动大，则需要额外排查 k6 本机资源、order-rpc CPU、Redis 压力和 Docker 调度影响

## 输出要求

- 每轮保留原始结果目录，不覆盖
- 最终单独产出一份汇总文档：`docs/perf/2026-04-19-grpc-rpc-10-runs-summary.md`
- 汇总文档至少包含：
  - 10 轮原始结果目录列表
  - `client_qps` 均值
  - `avg/p95/p99` 均值
  - `final_state_ok` 统计
  - 对 `window_qps` 的辅助说明
