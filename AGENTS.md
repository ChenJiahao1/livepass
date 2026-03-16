# damai-go Agents Guide

## 目标

`damai-go` 是基于 `Go + go-zero` 重建的大麦业务总线项目。

设计原则：

- 工程结构遵循 `go-zero` 官方习惯
- 业务命名参考原 Java 项目语义
- 服务发现使用 `etcd`
- 当前以用户域为首期落地范围
- 后续逐步扩展到 `program`、`order`、`pay`、`customize` 和 `agents`

## 总体约束

- 对外接口可以参考原 Java 项目已有业务能力
- 当前阶段默认采用单库单表实现，先不做分库分表
- 数据库表结构设计与实现优先参考原 Java 项目已有表定义
- `go-zero` 服务按服务类型组织：HTTP 服务使用 `services/*-api/`，gRPC 服务使用 `services/*-rpc/`
- 涉及 `go-zero` 服务开发时，使用 `zero-skills`
- 公共能力放在 `pkg/`，禁止把具体业务规则放入公共包
- 各服务的命名使用简洁英文，和 Java 项目语义保持一致
- 目录结构按 `go-zero` 生成结果扩展，不沿用 Java 的 `*-service` 目录形式
- `gateway` 作为 HTTP 入口服务，归入 `services/gateway-api/`
- `agents` 是预留的 Python 独立组件，不纳入 `go-zero` 服务目录规范，保留根级目录

## Codex 本地上下文

- 当前项目的 Codex 本地补充上下文位于 `.codex/`
- 执行 go-zero 相关任务时，先遵守本文件，再参考 `.codex/README.md`
- `.codex/ai-context/` 中的静态规则仅补充 go-zero 工作流、模式和 goctl 用法，不覆盖本文件的项目约束
- `zero-skills` 通过全局 skills 提供，本仓库内不重复 vendoring skills

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
│   ├── user-rpc/
│   ├── program-api/
│   ├── program-rpc/
│   ├── order-api/
│   ├── order-rpc/
│   ├── pay-api/
│   ├── pay-rpc/
│   ├── customize-api/
│   ├── customize-rpc/
│   └── gateway-api/
├── jobs/
│   ├── order-close/
│   ├── program-warmup/
│   └── cache-rebuild/
└── agents/
    ├── app/
    ├── config/
    ├── tests/
    ├── pyproject.toml
    └── README.md
```

## user 服务分层

### user-api

职责：

- 对外提供 HTTP 接口
- 保持接口层稳定
- 负责参数校验、鉴权、中间件、响应封装
- 调用 `user-rpc`，不承载核心业务归属

### user-rpc

职责：

- 承接用户域核心业务逻辑
- 负责 MySQL、Redis、etcd 等基础设施接入
- 作为内部统一契约供其他服务复用
- 后续供 `gateway-api`、`agents`、其他业务服务调用

## 目录约定

- 默认情况下，一个 `go-zero` 物理服务对应 `services/` 下一个独立目录
- 需要对外提供 HTTP 能力时，使用 `<name>-api`
- 需要对内提供 gRPC 能力时，使用 `<name>-rpc`
- 是否同时存在 `api` 与 `rpc`，由该业务域是否同时需要两类能力决定，不强制成对出现
- 非 `go-zero` 独立组件必须显式声明为例外；当前仅 `agents/` 作为 Python 组件保留在根级目录

## 后续服务边界

- `program`：节目、场次、票档、座位、活动主业务
- `order`：订单创建、关闭、状态流转
- `pay`：支付、回调、退款
- `customize`：规则、广播、扩展配置
- `agents`：基于 Python 实现的智能客服组件，通过内部服务契约获取业务数据
