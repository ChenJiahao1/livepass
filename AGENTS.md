# damai-go Agents Guide

## 目标

`damai-go` 是基于 `Go + go-zero` 重建的大麦业务总线项目。
原有java项目  `/home/chenjiahao/code/project/damai`

注意： 你写的代码将会提交给claude code审核

设计原则：

- 工程结构遵循 `go-zero` 官方习惯
- 业务命名参考原 Java 项目语义
- 当前以用户域为首期落地范围
- 后续逐步扩展到 `program`、`order`、`pay`、`customize` 和 `agents`

## 总体约束

- 对外接口可以参考原 Java 项目已有业务能力
- 当前阶段默认采用单库单表实现，先不做分库分表
- 数据库表结构设计与实现优先参考原 Java 项目已有表定义
- `go-zero` 服务按服务类型组织：HTTP 服务使用 `services/*-api/`，gRPC 服务使用 `services/*-rpc/`
- 涉及 `go-zero` 服务开发时，使用 `zero-skills`
- 所有 `goctl` 生成命令统一使用 `--style go_zero`，禁止省略 `--style` 或改用其他 style
- `go-zero` 相关 Go 文件名统一使用下划线风格，例如 `refund_order_logic.go`、`service_context.go`、`order_rpc_server.go`
- Go 标识符遵循 Go 原生驼峰命名，例如 `RefundOrderLogic`、`NewRefundOrderLogic`；文件名规则与标识符规则不要混用
- 公共能力放在 `pkg/`，禁止把具体业务规则放入公共包
- 各服务的命名使用简洁英文，和 Java 项目语义保持一致
- 目录结构按 `go-zero` 生成结果扩展，不沿用 Java 的 `*-service` 目录形式
- `gateway` 作为 HTTP 入口服务，归入 `services/gateway-api/`
- `agents` 是预留的 Python 独立组件，不纳入 `go-zero` 服务目录规范，保留根级目录
- 白盒单测可以保留在被测包旁边，但服务级集成测试必须放在 `services/<service>/tests/`
- 禁止将服务级测试继续放在 `internal/logic/`、`internal/middleware/`、`internal/config/` 等业务实现目录
- 跨服务验收测试放在根级 `tests/` 或 `scripts/acceptance/`
- 测试辅助优先放在 `services/<service>/tests/testkit/`；仅服务内集成测试使用的私有 helper 可以放在 `services/<service>/tests/integration/*_helpers_test.go`
- 设计或调整测试目录时，以 `docs/architecture/testing-layout.md` 为准

## Codex 本地上下文

- 当前项目的 Codex 本地补充上下文位于 `.codex/`
- 执行 go-zero 相关任务时，先遵守本文件，再参考 `.codex/README.md`
- `.codex/ai-context/` 中的静态规则仅补充 go-zero 工作流、模式和 goctl 用法，不覆盖本文件的项目约束
- 仓库内历史上已经存在的非下划线风格文件，仅视为待收敛遗留，不作为后续生成或新增文件的命名依据
- `zero-skills` 通过全局 skills 提供，本仓库内不重复 vendoring skills

## 抢票压测入口

- 抢票链路压测前置检查入口：`scripts/perf/order_pressure_preflight.sh`
- 抢票链路压测 runbook：`docs/architecture/order-pressure-preflight.md`
- 典型场景先执行：
  - `USER_POOL_FILE=.tmp/perf/order_user_pool.2000users.unique_20260324.json TARGET_RPC_INSTANCES=5 PROGRAM_ID=10001 TICKET_CATEGORY_ID=40001 REQUIRED_SEAT_COUNT=1000 bash scripts/perf/order_pressure_preflight.sh check`
- `prepare` 模式可用于补齐 Kafka topic 分区，并在需要时执行 ledger warmup
- 当前压测前置校验以订单分片表和 Redis seat ledger 为准

## 业务命名

- 用户服务：`user`
- 节目/活动服务：`program`
- 订单服务：`order`
- 支付服务：`pay`
- 定制规则服务：`customize`
- 网关服务：`gateway`
- 智能客服服务：`agents`

## 推荐目录

```text
damai-go/
├── go.work
├── go.mod
├── README.md
├── docs/
│   ├── architecture/
│   ├── migration/
│   └── api/
├── deploy/
│   ├── etcd/
│   ├── mysql/
│   ├── redis/
│   ├── docker-compose/
│   └── gateway/
├── scripts/
│   ├── goctl/
│   ├── build/
│   └── deploy/
├── pkg/
│   ├── xerr/
│   ├── xlog/
│   ├── xjwt/
│   ├── xmysql/
│   ├── xredis/
│   ├── xetcd/
│   ├── xid/
│   ├── xresponse/
│   └── xmiddleware/
├── sql/
│   ├── user/
│   ├── program/
│   ├── order/
│   ├── pay/
│   └── customize/
├── services/
│   ├── user-api/
│   │   └── tests/
│   ├── user-rpc/
│   │   └── tests/
│   ├── .../
│   └── gateway-api/
│       └── tests/
├── jobs/
│   ├── order-close/
│   │   └── tests/
│   ├── program-warmup/
│   └── cache-rebuild/
├── tests/
│   ├── acceptance/
│   ├── e2e/
│   ├── testdata/
│   └── testkit/
└── agents/
    ├── app/
    ├── config/
    ├── tests/
    ├── pyproject.toml
    └── README.md
```

## 目录约定

- 当前仅 `agents/` 作为 Python 组件保留在根级目录
- `services/<service>/tests/` 用于服务级集成测试、配置测试和测试辅助
- `jobs/<job>/tests/` 用于任务级测试
- `pkg/<pkg>/tests/` 用于公共包黑盒测试
- 根级 `tests/` 仅用于跨服务验收、端到端和共享测试资产
- 白盒单测与纯算法测试可以保留在业务包目录，例如 `internal/logic/*_test.go`

## 后续服务边界

- `program`：节目、场次、票档、座位、活动主业务
- `order`：订单创建、关闭、状态流转
- `pay`：支付、回调、退款
- `customize`：规则、广播、扩展配置
- `agents`：基于 Python 实现的智能客服组件，通过内部服务契约获取业务数据


program 的座位不支持用户手动选座，但是需要保留系统安排座位
不做真实支付,  仅模拟
