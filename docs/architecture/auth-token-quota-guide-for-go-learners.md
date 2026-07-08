# 认证、令牌与额度链路学习指南

这份文档梳理 new-api 的登录、控制台鉴权、API Key 鉴权、权限控制、额度预扣和后结算。它适合和 `backend-request-patterns-for-go-learners.md`、`data-model-and-state-guide-for-go-learners.md` 一起读。

先记住一个容易误解的点：new-api 控制台登录主链路不是 JWT，而是 Cookie Session。代码中确实有 JWT，但主要用于部分上游供应商签名，例如 Vertex、智谱、Kling 等。

## 一、两套认证体系

new-api 有两套不同认证体系：

| 场景 | 入口 | 鉴权方式 | 核心中间件 |
| --- | --- | --- | --- |
| 控制台/后台 API | `/api/...` | Cookie Session 或用户 access token | `UserAuth` / `AdminAuth` / `RootAuth` |
| AI relay API | `/v1/...`、Gemini、MJ、Suno 等 | API Key | `TokenAuth` / `TokenAuthReadOnly` |

两者共享用户模型和额度，但入口、上下文 key、错误格式不一样。

## 二、控制台登录链路

典型入口：

```text
POST /api/user/login
```

源码链路：

```text
router/api-router.go
  -> controller.Login
  -> model.User.ValidateAndFill
  -> controller.setupLogin
  -> gorilla session
  -> response
```

核心行为：

1. `controller.Login` 绑定用户名、密码、可选 2FA 信息。
2. `model.User.ValidateAndFill` 用 username 或 email 查用户。
3. 校验密码 hash。
4. 检查用户状态是否启用。
5. 如果启用 2FA，先写 `pending_username`、`pending_user_id` 到 session。
6. 登录成功后 `setupLogin` 写入 session。
7. `model.RecordLoginLog` 记录登录日志。

session 中常见字段：

```text
id
username
role
status
group
```

Session store 在 `main.go` 中注册，名字是 `session`，使用 cookie store。Cookie 配置包括 HttpOnly、SameSiteStrict、30 天生命周期等。

## 三、用户 access token

控制台还有一种“用户 access token”，用于没有 Cookie Session 的后台 API 调用。

入口：

```text
GET /api/user/token
```

源码链路：

```text
router/api-router.go
  -> UserAuth()
  -> controller.GenerateAccessToken
  -> users.access_token
```

它不是 JWT，而是随机 key，保存到用户表字段 `access_token`。验证时走：

```text
model.ValidateAccessToken
```

`authHelper` 会先尝试 session。如果没有 session，再从 `Authorization` 读取用户 access token。

## 四、后台 API 权限中间件

后台 API 使用这几个中间件：

```text
UserAuth()
AdminAuth()
RootAuth()
RequirePermission(...)
```

前三个都包装同一个 `authHelper`，区别是最低角色要求不同：

```text
UserAuth  -> common user
AdminAuth -> admin
RootAuth  -> root
```

`authHelper` 做的事情：

```text
读取 session 或 Authorization access token
  -> 校验 New-Api-User header
  -> 加载用户
  -> 检查状态
  -> 检查角色
  -> 写 Gin context
  -> c.Next()
```

它写入的常见 context 值：

```text
id
username
role
group
user_group
use_access_token
```

更细粒度的权限通过 `RequirePermission` 实现。比如渠道管理路由会先要求管理员，再按接口细分：

```text
ChannelRead
ChannelWrite
ChannelSensitiveWrite
```

读后台 API 时，不要只看 controller。先看 router 上叠了哪些 middleware，否则容易漏掉权限前提。

## 五、API Key 管理

API Key 由用户在控制台创建，后端模型是：

```text
model.Token
```

关键字段：

| 字段 | 含义 |
| --- | --- |
| `UserId` | 所属用户 |
| `Key` | token key |
| `Name` | token 名称 |
| `Status` | 启用/禁用 |
| `RemainQuota` | token 自身剩余额度 |
| `UnlimitedQuota` | 是否不限额 |
| `UsedQuota` | 已用额度 |
| `ModelLimitsEnabled` | 是否限制模型 |
| `ModelLimits` | 可用模型列表 |
| `AllowIps` | IP 白名单 |
| `Group` | 指定请求分组 |
| `CrossGroupRetry` | 是否允许跨组重试 |

创建链路：

```text
/api/token
  -> UserAuth()
  -> controller.AddToken
  -> model.Token.Insert
```

查询、更新、删除也都在 `controller/token.go`，并且必须确认 token 属于当前用户，避免泄漏其他用户 key。

## 六、API Key 鉴权链路

AI relay 请求走 `TokenAuth()`。

典型入口：

```text
Authorization: Bearer sk-...
POST /v1/chat/completions
```

源码链路：

```text
router/relay-router.go
  -> middleware.TokenAuth()
  -> model.ValidateUserToken
  -> model.GetTokenByKey
  -> model.GetUserCache
  -> middleware.SetupContextForToken
  -> middleware.Distribute()
  -> controller.Relay
```

`TokenAuth` 支持多种协议的 key 位置：

| 协议/场景 | key 来源 |
| --- | --- |
| OpenAI 兼容 | `Authorization: Bearer sk-...` |
| Claude Messages | `x-api-key` |
| Gemini | query `key` 或 `x-goog-api-key` |
| Midjourney | `mj-api-secret` |
| Realtime WebSocket | `Sec-WebSocket-Protocol` 中的 `openai-insecure-api-key` |

验证后会检查：

1. token 是否存在。
2. token 状态是否可用。
3. token 是否过期。
4. token 额度是否足够。
5. token IP 白名单。
6. 用户是否被禁用。
7. token 指定 group 是否对用户可用。
8. group ratio 是否存在。

然后 `SetupContextForToken` 写入 Gin context：

```text
id
token_id
token_key
token_name
token_unlimited_quota
token_quota
token_model_limit_enabled
token_model_limit
ContextKeyTokenGroup
ContextKeyTokenCrossGroupRetry
specific_channel_id
```

如果 API Key 形如指定渠道的格式，普通用户会被拒绝，管理员才允许指定渠道。

## 七、TokenAuthReadOnly

`TokenAuthReadOnly()` 是较宽松的 API Key 认证，用于只读查询接口，例如：

```text
/api/usage/token
/api/log/token
```

它允许用户用 API Key 查询自己的 token 用量或日志，但不会进入完整 relay 分发链路。

这类接口的设计目标是方便外部工具检查 key 状态，同时避免给它后台管理权限。

## 八、额度模型

new-api 的“钱包”主要是用户表上的 quota 字段，不是单独的 wallet 表。

核心字段在 `model.User`：

```text
Quota
UsedQuota
RequestCount
```

Token 也有自己的额度字段：

```text
RemainQuota
UsedQuota
UnlimitedQuota
```

所以一次请求可能同时影响：

- 用户钱包额度。
- token 剩余额度。
- token 已用额度。
- 用户已用额度。
- 用户请求次数。
- 渠道已用额度。
- 消费日志。

## 九、预扣费链路

relay 请求进入 `controller.Relay` 后，会先估算本次请求成本。

主链路：

```text
controller.Relay
  -> 估算 tokens
  -> relay/helper/price.go 计算价格
  -> service.PreConsumeBilling
  -> service.NewBillingSession
  -> BillingSession.preConsume
  -> PreConsumeTokenQuota
  -> WalletFunding.PreConsume / SubscriptionFunding.PreConsume
```

预扣的意义是：

```text
请求开始前先冻结或扣除一笔额度
  -> 防止余额不足还打到上游
  -> 请求失败时退款
  -> 请求成功后按实际 usage 结算差额
```

如果用户余额不足，预扣会返回错误，relay 不会继续请求上游。

## 十、后结算链路

请求成功后，根据上游返回 usage 计算实际费用。

文本请求链路：

```text
service.PostTextConsumeQuota
  -> TryTieredSettle / 普通价格计算
  -> model.UpdateUserUsedQuotaAndRequestCount
  -> 更新 token/channel 额度
  -> service.SettleBilling
  -> model.RecordConsumeLog
```

音频、WebSocket、异步任务也有各自的 post consume 函数，但模式类似：

```text
预扣额度
  -> 实际 usage
  -> actualQuota - preConsumedQuota
  -> 正数补扣
  -> 负数退款
  -> 写日志
```

`BillingSession.Settle` 负责把差额结算回钱包或订阅额度。

## 十一、失败退款

`controller.Relay` 在预扣成功后会注册 defer 逻辑。请求失败时，会调用：

```text
BillingSession.Refund
```

退款需要非常谨慎，因为 relay 有重试、流式响应、上游部分成功等复杂情况。读这段时要跟着：

```text
controller.Relay
  -> retry loop
  -> newAPIError
  -> billingSession
  -> record error log
```

判断一个错误是否记录日志、是否跳过重试、是否退款，要看 `types.NewAPIError` 上的 option。

## 十二、消费日志

消费日志入口：

```text
model.RecordConsumeLog
```

错误日志入口：

```text
model.RecordErrorLog
```

日志里重要字段：

```text
user_id
token_id
channel_id
model_name
token_name
quota
prompt_tokens
completion_tokens
group
request_id
upstream_request_id
other
```

`request_id` 来自 `middleware.RequestId()`，`upstream_request_id` 来自上游响应 header。排查扣费或请求失败时，这两个字段是最重要的线索。

## 十三、学习路径

建议按下面顺序读源码：

1. `controller/user.go`：登录、setupLogin、access token。
2. `middleware/auth.go`：`authHelper`、`TokenAuth`、`SetupContextForToken`。
3. `model/user.go`：用户状态、quota、access token。
4. `model/token.go`：API Key 模型和校验。
5. `router/api-router.go`：后台 API 如何挂鉴权。
6. `router/relay-router.go`：relay API 如何挂 TokenAuth 和 Distribute。
7. `controller/relay.go`：relay 主流程。
8. `service/billing.go`、`service/billing_session.go`：预扣、结算、退款。
9. `service/text_quota.go`：文本 usage 到 quota。
10. `model/log.go`：消费日志和错误日志。

把这条线走通，你就能理解 new-api 最核心的商业逻辑：谁能请求、请求走哪个 token、如何扣钱、失败如何退、日志如何查。

