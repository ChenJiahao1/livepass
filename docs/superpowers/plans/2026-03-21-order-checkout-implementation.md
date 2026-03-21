# Order Checkout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a repeatable gateway-based acceptance path that lets a fresh user complete registration, login, ticket-user setup, program lookup, order creation, mock payment, and order verification from zero.

**Architecture:** Keep the existing `gateway-api -> *-api -> *-rpc` flow unchanged and add only acceptance assets around it. The implementation adds one human-readable acceptance guide, one executable shell script that calls the gateway and carries runtime IDs forward, and one README entry point so the checkout path is discoverable and can be executed consistently.

**Tech Stack:** Bash, `curl`, `jq`, existing go-zero services, gateway-api, MySQL/Redis/etcd infrastructure

---

### Task 1: Write The Acceptance Guide

**Files:**
- Create: `docs/api/order-checkout-acceptance.md`
- Modify: `README.md`

- [ ] **Step 1: Write the failing documentation skeleton**

Create `docs/api/order-checkout-acceptance.md` with headings only so the missing operational details are visible immediately:

```md
# 下单主路径验收

## 前置条件

## 启动基础设施

## 导入 SQL

## 启动服务

## 执行主路径

## 成功判定

## 常见失败点
```

- [ ] **Step 2: Verify the skeleton is incomplete**

Run: `sed -n '1,120p' docs/api/order-checkout-acceptance.md`

Expected: the file exists but does not yet provide runnable requests or success criteria.

- [ ] **Step 3: Fill the guide with concrete commands and expected results**

Expand the guide so it includes:

```md
## 执行主路径

1. 注册用户
2. 登录并提取 `userId` / `token`
3. 新增两个观演人
4. 查询用户和观演人列表
5. 查询 `/program/preorder/detail`
6. 调用 `/order/create`
7. 调用 `/order/pay`
8. 调用 `/order/pay/check`
9. 调用 `/order/get`
```

Also add exact gateway URL, required request headers, sample JSON bodies, and success checkpoints for every step.

- [ ] **Step 4: Make the guide discoverable from the repository entry point**

Update `README.md` with a short section linking to the new acceptance guide and the script location:

```md
## 下单主路径验收

完整步骤见 `docs/api/order-checkout-acceptance.md`。
可执行脚本见 `scripts/acceptance/order_checkout.sh`。
```

- [ ] **Step 5: Review the rendered content**

Run: `sed -n '1,260p' docs/api/order-checkout-acceptance.md`

Expected: the guide is executable from top to bottom without relying on undocumented IDs.

- [ ] **Step 6: Commit**

```bash
git add README.md docs/api/order-checkout-acceptance.md
git commit -m "docs: add order checkout acceptance guide"
```

### Task 2: Build The Sequential Checkout Script

**Files:**
- Create: `scripts/acceptance/order_checkout.sh`
- Modify: `docs/api/order-checkout-acceptance.md`

- [ ] **Step 1: Write the failing script scaffold**

Create a minimal executable shell script with strict mode and placeholders:

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
CHANNEL_CODE="${CHANNEL_CODE:-0001}"

echo "TODO: register"
echo "TODO: login"
echo "TODO: checkout"
exit 1
```

- [ ] **Step 2: Verify the scaffold fails immediately**

Run: `bash scripts/acceptance/order_checkout.sh`

Expected: exit non-zero with the placeholder output, proving the real flow is not implemented yet.

- [ ] **Step 3: Implement the real HTTP flow**

Replace the placeholders with concrete functions such as:

```bash
register_user()
login_user()
add_ticket_user()
list_ticket_users()
fetch_preorder()
create_order()
pay_order()
check_payment()
get_order()
```

Implementation requirements:
- use only `gateway-api`
- use `curl` + `jq`
- generate a unique mobile number for each run
- parse and persist `USER_ID`, `TOKEN`, two `TICKET_USER_ID`s, `TICKET_CATEGORY_ID`, and `ORDER_NUMBER`
- send both `Authorization` and `X-Channel-Code` for order requests
- stop on any missing field or `success != true`

- [ ] **Step 4: Sync the guide with the script contract**

Amend `docs/api/order-checkout-acceptance.md` so it documents:

```md
脚本默认读取：
- `BASE_URL`
- `CHANNEL_CODE`
- `PROGRAM_ID`
- `TICKET_CATEGORY_ID`（可选；默认从预下单详情解析）
```

Also describe the script output fields and where execution stops on failure.

- [ ] **Step 5: Run shell syntax verification**

Run: `bash -n scripts/acceptance/order_checkout.sh`

Expected: no output, exit code `0`.

- [ ] **Step 6: Run a dry validation of dependencies**

Add a dependency preflight in the script and verify it manually:

```bash
command -v curl >/dev/null
command -v jq >/dev/null
```

Run: `bash scripts/acceptance/order_checkout.sh`

Expected: if services are not ready yet, the script fails at a real network/data step rather than a shell parsing step.

- [ ] **Step 7: Commit**

```bash
git add docs/api/order-checkout-acceptance.md scripts/acceptance/order_checkout.sh
git commit -m "feat: add gateway checkout acceptance script"
```

### Task 3: Execute And Stabilize The Main Path

**Files:**
- Modify: `docs/api/order-checkout-acceptance.md`
- Modify: `scripts/acceptance/order_checkout.sh`
- Modify: `README.md`

- [ ] **Step 1: Verify infrastructure state**

Run the checks needed before any checkout run:

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml ps
```

Expected: MySQL, Redis, and etcd are up. If not, start them before proceeding.

- [ ] **Step 2: Verify required SQL is loaded**

Run targeted checks against MySQL for:
- user tables
- program seed rows for `programId=10001`
- order tables
- pay tables

Use commands shaped like:

```bash
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT COUNT(*) FROM damai_program.d_program;"
```

Expected: tables exist and the seed program is queryable.

- [ ] **Step 3: Start the required services**

Run the services in separate terminals or background sessions:

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay-rpc.yaml
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
```

Expected: all services stay running without immediate crash.

- [ ] **Step 4: Execute the checkout script end-to-end**

Run: `bash scripts/acceptance/order_checkout.sh`

Expected: printed runtime values include a valid `userId`, `token`, two `ticketUserId`s, one `orderNumber`, and final paid order status.

- [ ] **Step 5: Fix the first real blocker you hit**

If the script fails, make the minimal focused change in either:
- the script
- the acceptance guide
- service code only if the failure proves the main path is currently broken

Then rerun the script from the beginning.

- [ ] **Step 6: Repeat until the main path passes**

Run: `bash scripts/acceptance/order_checkout.sh`

Expected: full success path completes with no manual value editing.

- [ ] **Step 7: Update the docs with verified output examples**

After a real successful run, add one short verified example block to `docs/api/order-checkout-acceptance.md` describing the observed success markers, not hard-coded secrets or volatile IDs.

- [ ] **Step 8: Run final verification**

Run:

```bash
bash -n scripts/acceptance/order_checkout.sh
go test ./...
```

Expected: shell syntax passes and the repository test suite remains green.

- [ ] **Step 9: Commit**

```bash
git add README.md docs/api/order-checkout-acceptance.md scripts/acceptance/order_checkout.sh
git commit -m "test: verify order checkout main path"
```
