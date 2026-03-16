# User Domain Alignment Design

**Date:** 2026-03-16

## Goal

在 `damai-go` 中按 `go-zero` 最佳实践补齐用户域能力，数据库采用单库单表，接口和核心行为尽量对齐原 Java 用户服务，验证码能力本轮不实现。

## Scope

本轮覆盖：

- 用户注册
- 用户登录与登出
- 用户信息查询
- 用户信息修改
- 手机号与邮箱唯一性维护
- 实名认证
- 购票人增删查
- 用户与购票人聚合查询
- Redis 登录态与登录失败次数控制
- 本地渠道 `code -> tokenSecret` 配置

本轮不覆盖：

- 验证码接口
- BloomFilter
- Java 组合校验器容器
- 分库分表

## Architecture

### user-api

职责：

- 暴露 HTTP 路由
- 处理请求绑定和参数校验
- 调用 `user-rpc`
- 返回统一响应

约束：

- 不承载业务逻辑
- 不直接访问数据库和 Redis

### user-rpc

职责：

- 实现用户域核心业务
- 访问 MySQL 与 Redis
- 管理 token 创建、解析和登录态
- 暴露内部统一 gRPC 契约

约束：

- 所有核心规则都放在 logic 层
- 复杂查询下沉到 model 自定义方法

### pkg

本轮会扩展下列稳定公共能力：

- `pkg/xjwt`：token 创建与解析
- `pkg/xredis`：Redis 配置与客户端初始化辅助
- `pkg/xerr`：领域错误与消息

## Database Design

数据库按原 Java 实体改为单库单表，放在 `sql/user/`：

- `d_user`
- `d_user_mobile`
- `d_user_email`
- `d_ticket_user`

### d_user

存储用户主体信息，包括：

- 基本资料
- 密码
- 认证状态
- 邮箱字段
- 手机号字段

### d_user_mobile

存储手机号到用户的唯一映射，用于：

- 注册去重
- 手机号登录
- 修改手机号时的唯一性校验

### d_user_email

存储邮箱到用户的唯一映射，用于：

- 邮箱登录
- 修改邮箱时的唯一性校验

### d_ticket_user

存储购票人信息，用于：

- 查询购票人列表
- 新增购票人
- 删除购票人
- 内部聚合查询

## API Alignment

对外 HTTP 路由统一按 Java `controller` 路径实现，不保留 `/user/getById` 别名。

保留和补齐的接口：

- `POST /user/get/id`
- `POST /user/get/mobile`
- `POST /user/register`
- `POST /user/exist`
- `POST /user/login`
- `POST /user/logout`
- `POST /user/update`
- `POST /user/update/password`
- `POST /user/update/email`
- `POST /user/update/mobile`
- `POST /user/authentication`
- `POST /user/get/user/ticket/list`
- `POST /ticket/user/list`
- `POST /ticket/user/add`
- `POST /ticket/user/delete`

不实现：

- `POST /user/captcha/check/need`
- `POST /user/captcha/get`
- `POST /user/captcha/verify`

## Behavior Alignment

### Register

- 校验手机号不能为空
- 校验密码和确认密码一致
- 校验手机号唯一
- 写入 `d_user`
- 写入 `d_user_mobile`
- 如果请求带邮箱则写入 `d_user_email`
- `mailStatus` 映射到 `email_status`
- `mail` 映射到 `email`

### Exist

- 按手机号检查是否存在
- 存在则返回业务错误
- 不存在则返回成功

### Login

- 支持手机号或邮箱二选一
- 校验 `code` 存在于本地渠道配置
- 校验密码
- Redis 记录登录态
- Redis 记录手机号或邮箱登录失败次数
- 超过阈值后拒绝继续登录
- token 使用渠道对应 `tokenSecret`

### Logout

- 解析 token
- 获取用户 ID
- 删除 Redis 登录态

### Update

- 更新用户基本资料
- 如带 `mobile` 且发生变化，同步更新 `d_user_mobile`
- 需要校验新手机号唯一性

### Update Password

- 更新 `d_user.password`

### Update Email

- 校验邮箱唯一性
- 更新 `d_user.email`
- 更新 `d_user.email_status`
- 同步维护 `d_user_email`

### Update Mobile

- 校验手机号唯一性
- 更新 `d_user.mobile`
- 同步维护 `d_user_mobile`

### Authentication

- 更新 `rel_name`
- 更新 `id_number`
- 更新 `rel_authentication_status = 1`

### Ticket User

- 列表：按 `user_id` 查询
- 新增：校验用户存在与 `(user_id, id_type, id_number)` 不重复
- 删除：按购票人 ID 删除

### Get User And Ticket User List

- 返回 `userVo`
- 返回 `ticketUserVoList`
- 仅作为内部复用接口

## Security And State

### Token

- 保留 Java 的 `code` 语义
- 用本地配置 `ChannelMap` 替代远程渠道服务
- token 中保存 `userId`
- token 过期时间可配置

### Redis Keys

本轮最小化实现以下能力：

- 登录态缓存
- 手机号登录失败计数
- 邮箱登录失败计数

验证码相关 Redis key 不实现。

## Response Mapping

### UserVo

字段对齐：

- `id`
- `name`
- `relName`
- `gender`
- `mobile`
- `emailStatus`
- `email`
- `relAuthenticationStatus`
- `idNumber`
- `address`

### TicketUserVo

字段对齐：

- `id`
- `userId`
- `relName`
- `idType`
- `idNumber`

### Desensitization

返回时做脱敏：

- `mobile`
- `idNumber`
- `ticketUser.relName`

数据库存储保持原值。

## go-zero Constraints

- handler 仅处理 HTTP
- api logic 仅做 RPC 调用和 DTO 转换
- rpc logic 仅通过 `ServiceContext` 访问依赖
- model 承担自定义查询
- 不在 handler 和 api logic 中写业务规则

## Delivery Order

1. 补齐 SQL
2. 补齐 model
3. 扩展配置与公共能力
4. 扩展 `user.proto`
5. 实现 `user-rpc`
6. 更新 `user-api`
7. 补齐测试
8. 本地联调验证
