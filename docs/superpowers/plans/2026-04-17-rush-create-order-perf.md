# Rush Create Order Perf Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为同场次同票档的 `create order` 抢票核心链路提供可配置压测模式、造数脚本、token 预发脚本、k6 压测脚本与结果校验脚本。

**Architecture:** 在 `gateway-api` 增加仅压测配置生效的免 JWT 身份注入能力，保留下游现有内部网关身份校验；通过 `scripts/perf` 目录下的脚本批量重建压测用户、观演人、座位库存并预发 `purchase token`，最终由 `tests/perf` 中的 k6 脚本从 `create order` 开始压测并输出汇总结果。

**Tech Stack:** Go、go-zero gateway、Bash、MySQL、Docker、k6、jq

---

### Task 1: 网关压测鉴权配置

**Files:**
- Modify: `services/gateway-api/internal/config/config.go`
- Modify: `services/gateway-api/internal/middleware/auth_helper.go`
- Modify: `services/gateway-api/internal/middleware/auth_middleware.go`
- Modify: `services/gateway-api/etc/gateway-api.perf.yaml`
- Test: `services/gateway-api/tests/integration/auth_middleware_test.go`
- Test: `services/gateway-api/tests/config/config_test.go`

- [ ] **Step 1: 写压测配置与中间件失败测试**
- [ ] **Step 2: 运行网关相关测试，确认新增用例先失败**
- [ ] **Step 3: 实现 `PerfMode` 配置、路径白名单与头部注入**
- [ ] **Step 4: 更新压测配置文件，开启 `create/poll/purchase-token/get` 路径**
- [ ] **Step 5: 运行网关相关测试，确认通过**

### Task 2: 压测造数与 token 预发脚本

**Files:**
- Create: `scripts/perf/prepare_rush_perf_dataset.sh`
- Create: `scripts/perf/lib/mysql.sh`
- Create: `scripts/perf/lib/http.sh`
- Create: `scripts/perf/lib/common.sh`
- Test: `tests/perf_prepare_dataset_script_test.sh`

- [ ] **Step 1: 写脚本级失败测试，覆盖导出文件、默认参数与 SQL/HTTP 调用骨架**
- [ ] **Step 2: 运行脚本测试，确认先失败**
- [ ] **Step 3: 实现公共库与主脚本的参数解析、用户/观演人/座位重建**
- [ ] **Step 4: 实现库存预热、purchase token 批量预发与 JSON/CSV 导出**
- [ ] **Step 5: 运行脚本测试，确认通过**

### Task 3: k6 压测脚本与汇总

**Files:**
- Create: `tests/perf/rush_create_order.js`
- Create: `tests/perf/lib/dataset.js`
- Create: `tests/perf/lib/summary.js`
- Create: `scripts/perf/verify_rush_perf_result.sh`

- [ ] **Step 1: 先写数据加载与 summary 相关的最小失败校验**
- [ ] **Step 2: 运行本地脚本检查，确认失败点正确**
- [ ] **Step 3: 实现 k6 `create order -> poll` 流程**
- [ ] **Step 4: 实现压测结果汇总与 MySQL 一致性校验脚本**
- [ ] **Step 5: 运行静态检查/脚本冒烟，确认输出结构正确**

### Task 4: 文档与远端使用说明

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-04-17-rush-create-order-perf-design.md`

- [ ] **Step 1: 补充压测准备、启动、执行与校验命令**
- [ ] **Step 2: 补充关键配置项说明与推荐参数**
- [ ] **Step 3: 运行必要的 grep/脚本确认文档中的路径与命令存在**

### Task 5: 验证

**Files:**
- Verify: `services/gateway-api/tests/integration/auth_middleware_test.go`
- Verify: `services/gateway-api/tests/config/config_test.go`
- Verify: `tests/perf_prepare_dataset_script_test.sh`

- [ ] **Step 1: 运行网关测试**
- [ ] **Step 2: 运行脚本测试**
- [ ] **Step 3: 汇总未自动验证的远端执行步骤**

