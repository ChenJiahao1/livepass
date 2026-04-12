# Xid Kubernetes StatefulSet NodeId 约定

## 背景

`pkg/xid` 已从“通过 etcd lease 抢占 snowflake nodeId”收缩为“启动期解析确定的 nodeId，然后在本地生成 ID”。

当前仓库采用双 provider：

- 线上 Kubernetes 环境使用 `kubernetes`
- 本地开发和测试环境使用 `static`

这次改造只删除了 `xid` 对 etcd 的直接依赖，没有删除仓库里的其它 etcd 用途。

## 号段规划

- `user-rpc`: `0-127`
- `program-rpc`: `128-255`
- `order-rpc`: `256-511`
- `pay-rpc`: `512-639`

checked-in 本地 YAML 当前固定使用：

- `user-rpc -> NodeId: 0`
- `program-rpc -> NodeId: 128`
- `order-rpc -> NodeId: 256`
- `pay-rpc -> NodeId: 512`

测试 helper 当前固定使用：

- `user-rpc tests -> NodeID: 900`
- `program-rpc tests -> NodeID: 901`
- `order-rpc tests -> NodeID: 902`
- `pay-rpc tests -> NodeID: 903`

## Provider 约定

### static

适用场景：

- 本地开发
- CI
- 集成测试
- 单机排障

约束：

- `NodeId` 必须在 `0..1023`
- 同一时刻并行运行的实例不能复用同一个 `NodeId`

示例：

```yaml
Xid:
  Provider: static
  NodeId: 256
```

### kubernetes

适用场景：

- Kubernetes StatefulSet 部署

解析规则：

- `PodName` 优先取配置值
- `PodName` 为空时回退 `os.Hostname()`
- 只接受 `<statefulset-name>-<ordinal>` 格式
- 最终 `nodeId = ServiceBaseNodeId + ordinal`

启动期校验：

- `ServiceBaseNodeId` 必须在 `0..1023`
- `MaxReplicas` 必须大于 `0`
- `ordinal < MaxReplicas`
- `ServiceBaseNodeId + ordinal <= 1023`

示例：

```yaml
Xid:
  Provider: kubernetes
  ServiceBaseNodeId: 256
  MaxReplicas: 64
  PodName: ${POD_NAME}
```

## Kubernetes 运行约束

- 发号服务必须使用 `StatefulSet`，不能用 `Deployment`
- 必须注入稳定的 `POD_NAME`
- `MaxReplicas` 不能超过对应服务预留号段容量
- 禁止通过 force delete 破坏 StatefulSet 稳定身份假设
- 同一个 ordinal 在任一时刻只能有一个存活实例

原因很直接：

- 这个方案依赖 StatefulSet ordinal 提供稳定且唯一的实例编号
- 一旦多个副本在同一时刻共享同一个 ordinal，对应 `nodeId` 就会冲突

## 为什么不删除 etcd

这次重构删除的是：

- `xid.InitEtcd`
- `xid.MustInitEtcd`
- `pkg/xid` 内部 lease grant / keepalive / revoke 逻辑

这次没有删除的 etcd 用途包括：

- go-zero RPC 服务注册发现使用的 `zrpc.RpcServerConf.Etcd`
- RPC client 服务发现使用的 `Etcd`
- `order-rpc` 的 `repeatguard` 分布式锁

因此：

- 不要删除四个 RPC 服务配置里的 `Etcd` 段
- 不要删除 `order-rpc` 的 `RepeatGuard` etcd 相关配置
- `deploy/etcd/docker-compose.yml` 仍然保留

## 运维检查清单

上线前至少确认以下几点：

- StatefulSet 名称与 `PodName` 解析规则匹配
- `ServiceBaseNodeId` 与服务号段一致
- `MaxReplicas` 没有越过号段容量
- 本地和测试环境没有复用静态 `NodeId`
- 监控和日志里没有出现 `unsupported xid provider`、`invalid kubernetes pod name`、`node id out of range` 之类启动错误
