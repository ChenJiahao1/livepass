# User Register Minimal Design

## 背景

`damai-go` 已完成 `user-api` 与 `user-rpc` 的初始化骨架，当前需要把首条真实写链路跑通。用户已确认本轮优先实现“注册”，并接受先不做分库分表、先不做验证码、先不做 JWT。

同时，参考相邻 Java 项目可知：

- 原用户主表语义来自 `d_user`
- 原注册流程会写 `d_user` 与 `d_user_mobile`
- 主键使用 `UidGenerator`
- Java 服务层直接复制 DTO 到实体

本轮仅保留这些语义中的必要部分，不复制完整分片与复合校验逻辑。

## 目标

本轮只完成一条最小真实注册链路：

- `user-api` 接收 `/user/register`
- `user-api` 调用 `user-rpc.Register`
- `user-rpc` 将用户记录写入 MySQL `d_user`
- 返回 `BoolResp{Success:true}`

## 范围

本轮包含：

- 新建非分片 `d_user` 表 DDL
- 生成并接入 go-zero model
- 实现注册落库
- 实现手机号重复校验
- 增加最小测试与接口验证

本轮不包含：

- `d_user_mobile`
- `d_user_email`
- 分库分表
- 验证码校验
- JWT 生成
- 登录链路

## 表设计

本轮使用 `d_user` 单表，对齐原表字段语义，保留注册最小闭环所需字段：

- `id`
- `name`
- `rel_name`
- `mobile`
- `gender`
- `password`
- `email_status`
- `email`
- `rel_authentication_status`
- `id_number`
- `address`
- `create_time`
- `edit_time`
- `status`

与原 SQL 不同之处：

- 不拆成 `d_user_0` / `d_user_1`
- 不新增 `d_user_mobile`
- 重复手机号校验通过应用查询实现，而不是依赖辅助表

## 兼容策略

- 请求字段沿用当前最小 API/RPC 契约，只要求 `mobile` 和 `password`
- 密码在 Go 服务内做 `md5` 十六进制摘要后再入库
- `id` 采用本地轻量 `int64` 生成策略，先满足单机开发与测试
- `status` 默认写 `1`

这里的密码摘要属于基于现有样例数据的兼容性推断，而不是直接来自 Java 服务层代码。

## 测试策略

- `user-rpc`：注册成功测试
- `user-rpc`：重复手机号注册失败测试
- `user-api`：调用 RPC 注册成功测试
- 命令验证：执行建表 SQL，启动 `user-rpc` / `user-api` 后，用 HTTP 请求验证 `/user/register`
