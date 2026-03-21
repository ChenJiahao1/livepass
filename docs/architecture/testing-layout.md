# 测试目录设计

## 目标

将服务级测试从业务实现目录中分离出来，同时保持 Go 包边界和 `go-zero` 服务结构不被破坏。

## 设计原则

- 白盒单测保留在被测包旁边，只测试未导出函数、未导出类型和纯内部算法。
- 服务级集成测试迁移到 `services/<service>/tests/`，避免 `internal/logic` 与 `internal/middleware` 堆满场景测试。
- 跨服务验收测试统一放在根级 `tests/` 或 `scripts/acceptance/`。
- 测试辅助能力集中在 `tests/testkit/`，避免重复散落在各个 `*_test.go` 文件中。
- 仅在该服务集成测试内部使用、且不值得导出的 helper，可以保留在 `tests/integration/*_helpers_test.go`。
- 测试数据放在 `tests/testdata/`，避免 SQL、JSON、YAML fixture 与业务代码混放。

## 推荐结构

```text
damai-go/
├── services/
│   ├── gateway-api/
│   │   ├── internal/
│   │   └── tests/
│   │       ├── config/
│   │       ├── integration/
│   │       ├── testdata/
│   │       └── testkit/
│   ├── user-rpc/
│   │   ├── internal/
│   │   └── tests/
│   │       ├── config/
│   │       ├── integration/
│   │       ├── testdata/
│   │       └── testkit/
│   └── ...
├── tests/
│   ├── acceptance/
│   ├── e2e/
│   ├── testdata/
│   └── testkit/
└── scripts/
    └── acceptance/
```

## 分类规则

### 保留在业务目录

- `internal/logic/*_test.go` 中直接访问未导出函数的白盒单测
- `internal/config/*_test.go` 中必须依赖包内未导出默认值或辅助函数的测试
- 类似 `seat_assignment_test.go` 这类纯算法测试

### 迁移到服务级 `tests/`

- 依赖 `ServiceContext` 的逻辑测试
- 连接 MySQL、Redis 的集成测试
- 需要 fake RPC、fake upstream 的测试
- 走 HTTP 路由、中间件、网关转发链路的测试
- 配置加载黑盒测试

### 迁移到根级 `tests/`

- 下单、支付、关单、释放库存这类跨服务链路测试
- 按 docker-compose 或真实依赖环境运行的验收测试

## 当前落地约定

- `services/user-rpc/tests/integration/`：用户域服务集成测试
- `services/user-rpc/tests/testkit/`：用户域数据库、Redis、seed 辅助
- `services/gateway-api/tests/integration/`：网关鉴权与转发测试
- `services/gateway-api/tests/testkit/`：网关测试 server、token、请求辅助
- `services/order-rpc/tests/integration/order_test_helpers_test.go`、`services/program-rpc/tests/integration/program_test_helpers_test.go`：仅供本服务集成测试使用的私有 fixture/helper，不强行导出到 `testkit`

## 迁移顺序建议

1. 先迁移服务级集成测试，保持白盒单测不动。
2. 再抽取每个服务自己的 `tests/testkit/`。
3. 最后统一补齐根级 `tests/acceptance/`。
