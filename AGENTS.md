# damai-go Agents Guide

## 作用范围

- 本文件仅用于约束仓库内 agent 的工作方式、代码生成约束、命名规范和目录规则。
- 项目背景、架构分层、运行方式、联调方法与对外入口说明，以 `README.md` 为准。
- 本仓库已按独立项目维护，不再参考任何外部历史项目作为实现依据。

## 基本要求

- 生成的代码将接受严格代码评审，要求结构清晰、命名准确、边界明确。
- 先遵守本文件，再参考 `.codex/README.md` 与 `.codex/ai-context/` 中的补充上下文。
- 涉及 go-zero 服务开发时，必须使用 `zero-skills`。

## 架构约束

项目默认采用 `Gateway -> API -> RPC` 分层：

- `gateway-api`：统一 HTTP 入口、路由汇聚、统一鉴权接入与观测接入。
- `services/*-api/`：HTTP 适配与协议层，负责 handler/middleware、参数解析、上下文提取、响应模型收口与必要的轻量聚合。
- `services/*-rpc/`：主要业务逻辑与服务契约层，负责领域规则、状态流转、数据访问、缓存、消息队列与内部服务协作。

实现要求：

- 不要把核心业务规则写进 `gateway-api`。
- 不要把核心业务规则堆进 `services/*-api/`。
- 需要承载领域规则、状态机、跨资源编排的逻辑，应进入 `services/*-rpc/`。

## 目录与命名

- go-zero HTTP 服务目录使用 `services/*-api/`。
- go-zero gRPC 服务目录使用 `services/*-rpc/`。
- `gateway` 作为 HTTP 入口服务，固定放在 `services/gateway-api/`。
- `agents` 是根级 Python 独立组件，不纳入 go-zero 服务目录规范。
- 公共能力放在 `pkg/`，禁止把具体业务规则放入公共包。
- Go 文件名统一使用下划线风格，例如 `refund_order_logic.go`、`service_context.go`、`order_rpc_server.go`。
- Go 标识符遵循 Go 原生驼峰命名，例如 `RefundOrderLogic`、`NewRefundOrderLogic`。

## goctl 与 go-zero 规则

- 所有 `goctl` 生成命令统一使用 `--style go_zero`，禁止省略 `--style` 或改用其他 style。
- 目录结构按 `go-zero` 生成结果扩展，避免自创与 go-zero 明显冲突的结构。
- 新增 handler、logic、svc、server、pb、types 等文件时，优先贴合 go-zero 官方生成风格。
- `.codex/README.md` 与 `.codex/ai-context/` 提供 go-zero 工作流、模式与 `goctl` 用法补充；它们仅作补充说明，不覆盖本文件约束。

## 服务边界

- `user`：注册、登录、资料维护、观演人管理
- `program`：节目、场次、票档、座位、预下单信息、系统自动分座冻结
- `order`：下单、查单、取消、支付检查、退款、超时关单
- `pay`：模拟支付、支付单查询、模拟退款
- `customize`：规则、广播、扩展配置
- `gateway`：统一外部 HTTP 入口
- `agents`：智能客服与业务协同组件

当前明确业务约束：

- `program` 不支持用户手动选座，但必须保留系统分配座位能力。
- `pay` 不做真实支付，仅模拟支付与退款。

## 测试约定

- 白盒单测可以保留在被测包旁边。
- 服务级测试统一放在 `services/<service>/tests/`。
- 任务级测试统一放在 `jobs/<job>/tests/`。
- 跨服务验收和端到端测试统一放在根级 `tests/` 或 `scripts/acceptance/`。
- `pkg/<pkg>/tests/` 用于公共包黑盒测试。

## 编码偏好

- 优先做小而清晰的改动，保持职责单一。
- 新增业务逻辑时，优先判断它属于 `API` 适配层还是 `RPC` 业务层，避免边界漂移。
- 除非已有约定要求，否则不要把业务规则抽进 `pkg/`。
- 仓库内历史遗留的不规范命名仅视为待收敛现状，不作为新增文件的依据。
