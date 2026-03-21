# Kafka Compose Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Kafka 纳入统一基础设施 compose，并同步修正文档与本地 broker 地址说明。

**Architecture:** 在现有 `deploy/docker-compose/docker-compose.infrastructure.yml` 中加入单节点 KRaft Kafka 服务，对宿主机暴露 `9094`，容器内保留 `9092`。同步更新 `order-rpc` 本地配置和订单验收文档，使本地联调路径收敛为单条命令。

**Tech Stack:** Docker Compose、Kafka KRaft、Go 配置文件 YAML、Markdown 文档

---

### Task 1: 更新基础设施 Compose

**Files:**
- Modify: `deploy/docker-compose/docker-compose.infrastructure.yml`
- Test: `deploy/docker-compose/docker-compose.infrastructure.yml`（配置文件变更，使用 `docker compose config` 做静态校验）

- [ ] **Step 1: 阅读现有基础设施 compose，确认 service 命名、端口风格和相对路径**

Run: `sed -n '1,240p' deploy/docker-compose/docker-compose.infrastructure.yml`
Expected: 看到当前仅有 `etcd`、`mysql`、`redis`

- [ ] **Step 2: 写入最小 Kafka 单节点 KRaft 配置**

Edit: 在同一文件新增 `kafka` service，要求包含：
- 合适的 Kafka 镜像
- KRaft 单节点环境变量
- `9094:9094` 端口映射
- 内外双 listener / advertised listener
- 基础健康检查

- [ ] **Step 3: 运行 compose 静态校验**

Run: `docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml config`
Expected: exit code 0，展开结果中包含 `kafka` service

### Task 2: 对齐本地 order-rpc Kafka 地址

**Files:**
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Test: `services/order-rpc/etc/order-rpc.yaml`（配置文件变更，使用精确 grep 校验）

- [ ] **Step 1: 检查现有 Kafka broker 地址**

Run: `sed -n '24,40p' services/order-rpc/etc/order-rpc.yaml`
Expected: 看到 `Kafka.Brokers` 当前值

- [ ] **Step 2: 把本地 broker 地址改为 compose 暴露地址**

Edit: 将 `Kafka.Brokers` 从旧端口改为 `127.0.0.1:9094`

- [ ] **Step 3: 校验配置已更新**

Run: `rg -n "127.0.0.1:9094" services/order-rpc/etc/order-rpc.yaml`
Expected: 能命中 Kafka broker 行

### Task 3: 修正文档说明

**Files:**
- Modify: `README.md`
- Modify: `docs/api/order-checkout-acceptance.md`
- Modify: `docs/api/order-checkout-failure-acceptance.md`
- Test: 三份文档（使用 `rg` 校验过时表述已消失，新地址说明已写入）

- [ ] **Step 1: 阅读现有文档中的 Kafka 表述**

Run: `rg -n "单独启动|额外准备 Kafka|9092|broker" README.md docs/api/order-checkout-acceptance.md docs/api/order-checkout-failure-acceptance.md`
Expected: 命中过时说明和当前 broker 描述

- [ ] **Step 2: 更新 README 与验收文档**

Edit:
- README 改为基础设施 compose 已包含 Kafka
- 主路径验收的启动成功判定加入 Kafka 容器
- 失败分支验收不再要求单独启动 Kafka
- 文档统一提示本地 `Kafka.Brokers` 应与 `127.0.0.1:9094` 对齐

- [ ] **Step 3: 校验文档已收敛**

Run: `rg -n "单独启动 Kafka|额外准备 Kafka|127.0.0.1:9092" README.md docs/api/order-checkout-acceptance.md docs/api/order-checkout-failure-acceptance.md`
Expected: 不再命中过时表述

- [ ] **Step 4: 校验新说明已存在**

Run: `rg -n "127.0.0.1:9094|基础设施 compose 已包含 Kafka|mysql.*redis.*etcd.*kafka|Kafka broker.*127.0.0.1:9094" README.md docs/api/order-checkout-acceptance.md docs/api/order-checkout-failure-acceptance.md`
Expected: 命中新说明

### Task 4: 汇总验证

**Files:**
- Modify: 无
- Test: `deploy/docker-compose/docker-compose.infrastructure.yml`, `services/order-rpc/etc/order-rpc.yaml`, `README.md`, `docs/api/order-checkout-acceptance.md`, `docs/api/order-checkout-failure-acceptance.md`

- [ ] **Step 1: 重新执行 compose 静态校验**

Run: `docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml config >/tmp/damai-kafka-compose.out && tail -n 40 /tmp/damai-kafka-compose.out`
Expected: exit code 0，输出尾部可见 `kafka` service 相关展开结果

- [ ] **Step 2: 查看本次改动 diff**

Run: `git diff -- deploy/docker-compose/docker-compose.infrastructure.yml services/order-rpc/etc/order-rpc.yaml README.md docs/api/order-checkout-acceptance.md docs/api/order-checkout-failure-acceptance.md`
Expected: diff 只覆盖 spec 中定义的范围

- [ ] **Step 3: 记录验证结论并准备收尾**

Run: `git status --short`
Expected: 仅出现计划内文件改动和已存在的无关未跟踪项
