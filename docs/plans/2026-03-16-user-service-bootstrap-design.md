# User Service Bootstrap Design

## 背景

`damai-go` 当前仍处于空仓初始化阶段，但首期目标已经明确为用户域，并约定采用 `go-zero + etcd + MySQL + Redis` 作为基础技术栈。现有实现计划已经覆盖了 `user-api` 与 `user-rpc` 的方向，但顺序偏向“直接定义契约并生成代码”，与仓库内 [AGENTS.md](/home/chenjiahao/code/project/damai-go/AGENTS.md) 提出的“先搭骨架，再补业务”不完全一致。

本设计将首轮工作收敛为“工程初始化 + 服务骨架生成”，先把目录、配置、共享包和 go-zero 生成主干落地，再进入具体用户业务实现。

## 目标

本轮只完成以下目标：

- 初始化 `damai-go` 的工作区与基础目录
- 建立符合 go-zero 习惯的 `user-api` 与 `user-rpc` 服务骨架
- 补齐本地开发所需的基础配置模板与部署模板
- 预留用户域 API / RPC 契约入口，保证后续可以直接按 go-zero 模式继续实现

本轮不完成以下内容：

- 不实现注册、登录、资料修改等完整业务逻辑
- 不落真实 MySQL 表模型、缓存策略和分片路由
- 不创建 `agents/` 目录
- 不提前抽象复杂公共层或统一业务基类

## 目录设计

首轮初始化后的推荐目录如下：

```text
damai-go/
├── go.mod
├── go.work
├── README.md
├── docs/
│   ├── architecture/
│   ├── migration/
│   ├── api/
│   └── plans/
├── deploy/
│   ├── docker-compose/
│   ├── etcd/
│   ├── mysql/
│   └── redis/
├── scripts/
│   ├── build/
│   ├── deploy/
│   └── goctl/
├── pkg/
│   ├── xerr/
│   ├── xetcd/
│   ├── xjwt/
│   ├── xmiddleware/
│   ├── xmysql/
│   ├── xredis/
│   └── xresponse/
├── sql/
│   └── user/
└── services/
    ├── user-api/
    └── user-rpc/
```

这一级目录只承载基础设施与服务外壳，不承载用户域深层实现。

## 服务设计

### user-api

`user-api` 负责对外 HTTP 接口承接，主要职责保持不变：

- 暴露与原 Java 用户控制器兼容的路径
- 负责参数解析、鉴权、中间件和响应包装
- 将业务调用转发到 `user-rpc`

本轮只要求：

- 提供聚合入口 `user.api`
- 将用户、购票人、验证码契约拆分到 `desc/` 下
- 通过 `goctl api go` 生成标准 go-zero 服务骨架
- 预留 `user-rpc` 客户端依赖与配置项

### user-rpc

`user-rpc` 负责用户域内部业务入口，主要职责保持不变：

- 承接用户域核心业务逻辑
- 统一封装 MySQL、Redis、etcd 等依赖访问入口
- 为 `user-api` 和未来其他服务提供统一内部契约

本轮只要求：

- 定义最小可生成的 `user.proto`
- 使用 `goctl rpc protoc` 生成 gRPC 与 go-zero 主干代码
- 在 `ServiceContext` 中预留数据库、缓存、鉴权等接线位

## 契约策略

本轮契约遵循“最小可生成”原则：

- `user-api` 保留完整的业务模块划分，但请求/响应字段只覆盖骨架生成所需的最小集合
- `user-rpc` 先定义服务名、核心方法名与基础消息体，保证生成结构稳定
- 路由命名、方法命名、服务命名尽量与原 Java 语义保持一致
- 复杂字段、校验细节、错误码映射留到下一轮业务实现时再补齐

这样做的原因是先稳定目录、文件名和依赖方向，减少生成后反复改结构的成本。

## 配置设计

初始化阶段会同时落以下配置：

- `services/user-api/etc/user-api.yaml`
- `services/user-rpc/etc/user-rpc.yaml`
- `deploy/etcd/docker-compose.yml`
- `deploy/mysql/docker-compose.yml`
- `deploy/redis/docker-compose.yml`
- `deploy/docker-compose/docker-compose.infrastructure.yml`

配置原则：

- API 服务嵌入 `rest.RestConf`
- RPC 服务嵌入 `zrpc.RpcServerConf`
- `user-api` 通过 `zrpc.RpcClientConf` 或等价结构声明 `UserRpc`
- `user-rpc` 预留 `Etcd`、`MySQL`、`Redis`、`Auth` 配置块
- 所有值先使用本地开发默认值，避免在首轮引入环境切分复杂度

## 公共包设计

`pkg/` 首轮只保留轻量公共能力：

- `xerr`: 通用错误常量与简单错误码约定
- `xresponse`: HTTP 响应包装结构
- `xjwt`: JWT 配置与基础生成/解析入口
- `xmysql`: MySQL 连接初始化辅助
- `xredis`: Redis 客户端初始化辅助
- `xetcd`: etcd 配置辅助
- `xmiddleware`: 通用鉴权中间件占位

这些包只提供基础设施层能力，不写入用户业务规则。

## 生成策略

为了与 go-zero 官方风格保持一致，本轮统一使用 `goctl` 生成代码：

- API：`goctl api go -api services/user-api/user.api -dir services/user-api --style go_zero`
- RPC：`goctl rpc protoc services/user-rpc/user.proto --go_out=./services/user-rpc --go-grpc_out=./services/user-rpc --zrpc_out=./services/user-rpc --style go_zero`

生成后立即执行：

- `go mod tidy`
- `go build ./...`

如有导入路径不一致，优先修正模块路径与生成物导入，而不是手写替代 goctl 生成内容。

## 验收标准

本轮完成后，仓库应满足以下条件：

- 目录结构符合 [AGENTS.md](/home/chenjiahao/code/project/damai-go/AGENTS.md) 的首期约束，且不包含 `agents/`
- `go.mod`、`go.work`、`README.md`、`pkg/`、`deploy/`、`sql/user/` 已初始化
- `user-api` 与 `user-rpc` 都存在可继续扩展的 go-zero 标准骨架
- 服务配置文件与基础设施模板文件已就位
- `go mod tidy` 与 `go build ./...` 可以作为首轮验证命令

## 后续实现顺序

骨架完成后，下一轮再进入用户业务能力实现，推荐顺序如下：

1. 完善 `user.api` 与 `user.proto` 的请求响应结构
2. 实现 `user-api -> user-rpc` 的最小调用链路
3. 接入用户表与购票人表模型
4. 补充登录、注册、用户资料、购票人、验证码逻辑
5. 增加测试、错误码与缓存策略
