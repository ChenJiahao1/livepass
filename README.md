# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目。

当前阶段：用户域 API / RPC / MySQL / Redis 联调已打通。

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

## 运行测试

```bash
go test ./...
```

## 启动服务

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
```

`user-rpc` 默认注册到本地 `etcd`，登录态存储在 `StoreRedis` 指向的 Redis。

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
