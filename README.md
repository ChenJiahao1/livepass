# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目。

当前阶段：用户域链路已打通，`program` 域 Phase 1 只读链路已补齐（category/home/page/detail/ticket-category）。

## 本地基础设施启动

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

## 初始化用户域表结构

MySQL 容器启动后，执行以下命令导入用户域 SQL：

```bash
for f in sql/user/d_user.sql sql/user/d_user_mobile.sql sql/user/d_user_email.sql sql/user/d_ticket_user.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_user < "$f"
done
```

## 初始化 program 域表结构

`damai_program` 会在 MySQL 首次启动时由 `deploy/mysql/init/01-create-databases.sql` 自动创建。导入 program 域 Phase 1 只读表结构和种子数据：

```bash
for f in \
  sql/program/d_program_category.sql \
  sql/program/d_program_group.sql \
  sql/program/d_program.sql \
  sql/program/d_program_show_time.sql \
  sql/program/d_ticket_category.sql \
  sql/program/dev_seed.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_program < "$f"
done
```

## 运行测试

```bash
go test ./...
```

## 启动服务

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
```

`user-rpc` 与 `program-rpc` 默认注册到本地 `etcd`。`user-api` 默认监听 `8888`，`program-api` 默认监听 `8889`。`user-rpc` 登录态存储在 `StoreRedis` 指向的 Redis。

## 手工验证用户链路

注册：

```bash
curl -X POST http://127.0.0.1:8888/user/register \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13800000003","password":"123456","confirmPassword":"123456"}'
```

预期响应：

```json
{"success":true}
```

登录：

```bash
curl -X POST http://127.0.0.1:8888/user/login \
  -H 'Content-Type: application/json' \
  -d '{"code":"0001","mobile":"13800000003","password":"123456"}'
```

预期响应包含 `userId` 与 `token`：

```json
{"userId":116260553874210817,"token":"<jwt>"}
```

按 ID 查询用户：

```bash
curl -X POST http://127.0.0.1:8888/user/get/id \
  -H 'Content-Type: application/json' \
  -d '{"id":116260553874210817}'
```

预期返回用户基础信息：

```json
{"id":116260553874210817,"name":"","relName":"","gender":1,"mobile":"13800000003","emailStatus":0,"email":"","relAuthenticationStatus":0,"idNumber":"","address":""}
```

## 手工验证 program Phase 1 链路

查询演出分类：

```bash
curl -X POST http://127.0.0.1:8889/program/category/select/all \
  -H 'Content-Type: application/json' \
  -d '{}'
```

首页分类分组：

```bash
curl -X POST http://127.0.0.1:8889/program/home/list \
  -H 'Content-Type: application/json' \
  -d '{"parentProgramCategoryIds":[1,2]}'
```

分页查询：

```bash
curl -X POST http://127.0.0.1:8889/program/page \
  -H 'Content-Type: application/json' \
  -d '{"parentProgramCategoryId":1,"timeType":0,"pageNumber":1,"pageSize":10,"type":1}'
```

查询演出详情：

```bash
curl -X POST http://127.0.0.1:8889/program/detail \
  -H 'Content-Type: application/json' \
  -d '{"id":10001}'
```

查询演出下票档：

```bash
curl -X POST http://127.0.0.1:8889/ticket/category/select/list/by/program \
  -H 'Content-Type: application/json' \
  -d '{"programId":10001}'
```

预期：以上五个接口都返回 HTTP 200，且能看到 `dev_seed.sql` 中的分类、演出、场次和票档数据。
