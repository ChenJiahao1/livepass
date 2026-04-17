# 全量启动与运行数据重建设计

**日期：** 2026-04-17

## 目标

为 `damai-go` 提供两个独立脚本：

- `scripts/deploy/start_backend.sh`：真正的一键启动入口，负责拉起基础设施、全部 Go 服务、全部 MCP、`agents` proto stub 生成与 `agents` API。
- `scripts/deploy/rebuild_databases.sh`：运行数据重置入口，负责重建 MySQL 业务库、清空 Redis、清空 Kafka 业务 Topic，但不负责启动服务。

## 范围

### 一键启动

启动脚本需要覆盖：

- Docker 基础设施：MySQL、Redis、etcd、Kafka
- Go RPC：`user-rpc`、`program-rpc`、`order-rpc`、`pay-rpc`
- Go Job：`order-close-worker`、`order-close-dispatcher`、`rush-inventory-preheat-worker`、`rush-inventory-preheat-dispatcher`
- Go API：`user-api`、`program-api`、`order-api`、`pay-api`、`gateway-api`
- MCP：`order-mcp`、`program-mcp`
- `agents`：先生成 proto stub，再启动 `uvicorn`

### 运行数据重建

重建脚本默认执行：

- 删除并重建 `damai_user`、`damai_program`、`damai_order`、`damai_pay`、`damai_agents`
- 调用 `scripts/import_sql.sh` 导入 schema 与 seed
- 清空 Redis 当前业务库
- 删除并重建业务 Kafka Topic，清空历史消息

## 关键设计

### 启动脚本

- 启动基础设施由“只检查”改为“检查，不存在则拉起”
- 启动 `agents` 前先执行 `agents/scripts/generate_proto_stubs.sh`
- 空库初始化改为“自动检测后导入”，而不是默认每次导入
- 启动摘要中补充 `program-mcp` 端口 `9083`

### 数据重建脚本

- 仅负责数据层重置，不处理进程生命周期
- 复用现有 `scripts/import_sql.sh`，避免重复维护 SQL 文件清单
- Kafka 采用删除后重建 Topic 的方式清空业务消息
- Redis 默认清空 DB 0，可通过环境变量覆盖

## 约束

- 保持现有脚本入口不变，继续使用 `bash scripts/deploy/start_backend.sh`
- 不改业务服务配置文件
- 不在启动脚本中混入“重建环境”逻辑

## 测试策略

- 先补脚本级测试，验证：
  - 启动脚本包含基础设施拉起、空库检测、`program-mcp`、proto stub 生成
  - 重建脚本包含 MySQL 重建、Redis 清空、Kafka Topic 重建
- 再修改脚本实现
- 最后运行脚本测试集合做验证
