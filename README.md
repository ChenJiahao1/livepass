# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目。

当前阶段：用户服务工程初始化。

本地基础设施启动：

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

最小注册链路验证：

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml

curl -X POST http://127.0.0.1:8888/user/register \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13800000003","password":"123456"}'
```
