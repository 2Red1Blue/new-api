# new-api 源码精读 Walkthrough

这份文档是 `go-source-learning-guide.md` 的实战篇。前者按 Go 语言点组织，本篇按 new-api 的真实业务调用链组织。目标是让你能打开源码，一边跳转函数，一边理解 Go 在真实项目中如何落地。

适合读者：

- 已掌握 Go 基本语法。
- 能看懂 struct、interface、error、goroutine 的基本写法。
- 希望通过 new-api 的真实源码建立“读大型 Go 项目”的能力。

阅读方式：

1. 先读每条链路的“全局路径”。
2. 打开列出的文件，按步骤跳函数。
3. 每一步只回答三个问题：输入是什么、输出是什么、状态写到哪里。
4. 最后完成本节练习。

## 目录

- [一、登录链路：从 Gin handler 到 session](#一登录链路从-gin-handler-到-session)
- [二、API Key 调用链路：从 TokenAuth 到 RelayInfo](#二api-key-调用链路从-tokenauth-到-relayinfo)
- [三、渠道选择链路：从 model 字段到上游 channel](#三渠道选择链路从-model-字段到上游-channel)
- [四、OpenAI 非流式文本链路：从请求转换到响应写回](#四openai-非流式文本链路从请求转换到响应写回)
- [五、上游 HTTP 请求链路：URL、Header、Body、Client](#五上游-http-请求链路urlheaderbodyclient)
- [六、文本计费结算链路：usage 到 quota 到日志](#六文本计费结算链路usage-到-quota-到日志)
- [七、系统任务链路：定时任务、DB lease、goroutine](#七系统任务链路定时任务db-leasegoroutine)
- [八、读源码的断点清单](#八读源码的断点清单)

## 一、登录链路：从 Gin handler 到 session

### 全局路径

```text
POST /api/user/login
  -> router/api-router.go
  -> controller.Login
  -> model.User.ValidateAndFill
  -> controller.setupLogin
  -> sessions.Default(c).Save()
  -> model.RecordLoginLog
  -> JSON { success, data }
```

### 1. 路由入口

文件：`router/api-router.go`

找到：

```go
userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
```

读这一行时注意 Gin 的 handler 顺序：

1. 先进入 `/api` group 的公共 middleware。
2. 再进入 `CriticalRateLimit()`。
3. 再进入匿名请求体限制。
4. 再进入 Turnstile 校验。
5. 最后执行 `controller.Login`。

Go 学习点：Gin handler 本质都是 `func(*gin.Context)`，middleware 也是同样签名，只是会在内部调用 `c.Next()` 继续链路。

### 2. 请求体解析

文件：`controller/user.go`

`LoginRequest`：

```go
type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}
```

`Login()` 中：

```go
var loginRequest LoginRequest
err := json.NewDecoder(c.Request.Body).Decode(&loginRequest)
```

这里使用标准库 `encoding/json`。这段属于 controller 旧代码风格；当前项目规则要求业务代码优先使用 `common.DecodeJson` / `common.Unmarshal` 这类 wrapper。读源码时要能区分“现有历史代码”和“新增代码应该遵守的规则”。

### 3. 用 model 做校验

`Login()` 构造一个临时 `model.User`：

```go
user := model.User{
    Username: username,
    Password: password,
}
err = user.ValidateAndFill()
```

读法：

- `user` 是值变量。
- `ValidateAndFill()` 使用指针接收者时，可以把数据库查到的字段填回 `user`。
- 返回 `error` 表示认证失败、数据库失败或参数失败。

这里是 Go 的常见风格：controller 不直接写 SQL，而是把业务对象交给 model 方法填充。

### 4. 错误分支

`Login()` 对错误使用：

```go
switch {
case errors.Is(err, model.ErrDatabase):
case errors.Is(err, model.ErrUserEmptyCredentials):
default:
}
```

Go 学习点：

- `errors.Is` 可以识别被包装过的 sentinel error。
- `switch` 没有表达式时，每个 `case` 是布尔条件。
- 不同错误映射到不同 i18n 消息。

### 5. 2FA 分支

如果 `model.IsTwoFAEnabled(user.Id)` 为 true：

```go
session := sessions.Default(c)
session.Set("pending_username", user.Username)
session.Set("pending_user_id", user.Id)
err := session.Save()
```

这个分支不直接登录，而是写 pending session，让 `/api/user/login/2fa` 继续验证。

读源码时注意：同一个 handler 里多个早返回分支，都是业务状态机的一部分。Go 项目常用 early return 降低嵌套。

### 6. setupLogin

`setupLogin(user, c)` 做真正登录：

```go
session.Set("id", user.Id)
session.Set("username", user.Username)
session.Set("role", user.Role)
session.Set("status", user.Status)
session.Set("group", user.Group)
err := session.Save()
```

然后：

- `recordLoginAudit(user, c)` 写登录日志。
- `c.JSON(http.StatusOK, gin.H{...})` 返回用户基础信息。

Go 学习点：

- `gin.H` 是 `map[string]any` 的便捷类型。
- session 是请求上下文中的对象，最终通过 cookie/session store 持久化。

### 练习

跟读 `/api/user/login/2fa`：

1. 它从 session 里读哪些 pending 字段？
2. 成功后是否复用 `setupLogin()`？
3. 失败时如何返回错误？

## 二、API Key 调用链路：从 TokenAuth 到 RelayInfo

### 全局路径

```text
POST /v1/chat/completions
  -> router/relay-router.go
  -> middleware.TokenAuth()
  -> model.ValidateUserToken()
  -> model.GetUserCache()
  -> middleware.SetupContextForToken()
  -> middleware.Distribute()
  -> relaycommon.GenRelayInfo()
```

### 1. 路由入口

文件：`router/relay-router.go`

典型路由：

```go
httpRouter.POST("/chat/completions", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatOpenAI)
})
```

这个 group 前面挂了：

```text
SystemPerformanceCheck
TokenAuth
ModelRequestRateLimit
Distribute
```

顺序很重要：必须先认证 token，才能知道用户分组和 token 模型限制；必须先分发渠道，`controller.Relay` 才能生成完整 `RelayInfo`。

### 2. TokenAuth 兼容多种客户端

文件：`middleware/auth.go`

`TokenAuth()` 支持多种 key 来源：

- OpenAI 兼容：`Authorization: Bearer sk-...`
- Anthropic：`x-api-key`
- Gemini：query `key` 或 `x-goog-api-key`
- Midjourney：`mj-api-secret`
- Realtime WebSocket：`Sec-WebSocket-Protocol`

读法：先忽略所有分支细节，只看最终归一化目标：

```text
把各种 key 来源统一成 key 字符串
  -> 去掉 Bearer / sk- 前缀
  -> 拆分 sk-key-channelId
  -> model.ValidateUserToken(key)
```

Go 学习点：

- 字符串处理大量使用 `strings.HasPrefix`、`strings.TrimPrefix`、`strings.Split`。
- 中间件通过 early return 终止请求。
- 管理员指定渠道是通过解析 key 后缀实现的。

### 3. token 和用户缓存

`model.ValidateUserToken(key)`：

```text
GetTokenByKey
  -> 检查 token 状态
  -> 检查过期时间
  -> 检查剩余额度
```

然后 `TokenAuth()` 读取：

```go
userCache, err := model.GetUserCache(token.UserId)
```

`userCache.WriteContext(c)` 会把用户信息写入 Gin context。这里的设计目的是：relay 高频请求不必每次都完整查用户表。

### 4. token 分组和模型限制

`TokenAuth()` 处理：

- token IP 白名单。
- token 绑定分组。
- 用户是否能使用该分组。
- 分组是否还在 `GroupRatio` 中存在。
- token model limits。

然后调用：

```go
err = SetupContextForToken(c, token, parts...)
```

### 5. SetupContextForToken

这个函数把后续链路需要的 token 状态写入 context：

```go
c.Set("id", token.UserId)
c.Set("token_id", token.Id)
c.Set("token_key", token.Key)
c.Set("token_name", token.Name)
c.Set("token_unlimited_quota", token.UnlimitedQuota)
```

还会写：

- `ContextKeyTokenGroup`
- `ContextKeyTokenCrossGroupRetry`
- `specific_channel_id`
- `token_model_limit_enabled`
- `token_model_limit`

### 6. GenRelayInfo

文件：`relay/common/relay_info.go`

`relaycommon.GenRelayInfo()` 会根据 relay format 选择构造函数，例如 OpenAI：

```go
info = GenRelayInfoOpenAI(c, request)
```

底层 `genBaseRelayInfo()` 从 context 读取：

- 用户 id、quota、group。
- token id、key、group。
- 原始模型名。
- request id。
- request start time。
- stream 状态。

Go 学习点：这就是 context 到显式 struct 的转换。中间件阶段状态零散地存在 Gin context 中；进入 relay 主流程后，状态集中到 `RelayInfo`，后续函数都传 `*RelayInfo`。

### 练习

追踪 `token_quota`：

1. `TokenAuth()` 什么时候写入？
2. `BillingSession.shouldTrust()` 如何读取？
3. 如果 token 是 unlimited，会发生什么？

## 三、渠道选择链路：从 model 字段到上游 channel

### 全局路径

```text
Distribute()
  -> getModelRequest()
  -> token model limit check
  -> GetPreferredChannelByAffinity()
  -> CacheGetRandomSatisfiedChannel()
  -> model.GetRandomSatisfiedChannelWithExclusions()
  -> SetupContextForSelectedChannel()
```

### 1. 读取 model

文件：`middleware/distributor.go`

`getModelRequest()` 会根据不同 API 形态读取模型：

- JSON body：读取 `model` 字段。
- multipart/form-data：表单中读取。
- Gemini native：从 URL path 提取。
- audio/image：必要时填默认模型。
- task 类接口：根据 action 推导模型。

Go 学习点：大型 handler 前置逻辑常常不是单一解析，而是协议兼容层。读这种函数时先按 URL 类型分块，不要逐行平铺。

### 2. 模型白名单

如果 token 开启模型限制：

```go
modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
```

然后检查 `tokenModelLimit` map。

Go 学习点：

- 从 context 取值要做类型断言。
- map 查找使用 `value, ok := m[key]`。
- 权限失败直接 abort。

### 3. 亲和性优先

`Distribute()` 会先尝试：

```go
service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup)
```

如果用户或模型最近绑定了某个渠道，且渠道仍满足模型/分组/路径要求，就优先使用。

这体现了项目设计：普通选择逻辑前面还有“业务偏好层”。

### 4. 普通渠道选择

`service.CacheGetRandomSatisfiedChannel(&service.RetryParam{...})`

`RetryParam` 包含：

- Gin context。
- token group。
- model name。
- request path。
- retry 指针。
- excluded map。
- failure map。

底层到 `model.GetRandomSatisfiedChannelWithExclusions()`，从内存缓存里按 group/model 找候选。

选择策略：

1. 分组过滤。
2. 模型能力过滤。
3. Advanced Custom 路径过滤。
4. priority 层级。
5. 同 priority 内按 weight 随机。
6. 排除本请求失败过的渠道。

### 5. 写入选中渠道上下文

`SetupContextForSelectedChannel(c, channel, modelName)` 写入：

- channel id/name/type/key/baseURL。
- model mapping。
- status code mapping。
- param/header override。
- multi-key 信息。
- Azure/Vertex/Gemini 等特殊 `api_version` 或 region。

然后 `controller.Relay` 才能基于这些 context 初始化 `RelayInfo.ChannelMeta`。

### 练习

选择一个渠道配置字段，比如 `ParamOverride`：

1. 它在 `Channel` struct 中怎么存？
2. `SetupContextForSelectedChannel()` 怎么写入 context？
3. `TextHelper()` 在哪里应用它？
4. 日志中是否记录应用情况？

## 四、OpenAI 非流式文本链路：从请求转换到响应写回

### 全局路径

```text
controller.Relay()
  -> helper.GetAndValidateRequest()
  -> relaycommon.GenRelayInfo()
  -> helper.ModelPriceHelper()
  -> service.PreConsumeBilling()
  -> relay.TextHelper()
  -> openai.Adaptor.ConvertOpenAIRequest()
  -> channel.DoApiRequest()
  -> openai.OpenaiHandler()
  -> service.PostTextConsumeQuota()
```

### 1. controller.Relay 主控

文件：`controller/relay.go`

先看它的结构：

```text
defer 错误响应
解析请求
生成 RelayInfo
敏感词检查
估算 token
计算价格
预扣费
defer 失败退款
for retry loop:
  getChannel
  reset body
  relayHandler
  processChannelError
```

Go 学习点：

- 大函数要先看“骨架”，再读分支。
- `defer` 用于统一错误响应和失败退款。
- retry loop 中每次都重新获取 body storage，因为请求体需要重新发给不同渠道。

### 2. TextHelper 初始化渠道元信息

文件：`relay/compatible_handler.go`

`TextHelper()` 第一件事：

```go
info.InitChannelMeta(c)
```

这一步把 context 中的 channel 信息复制到 `RelayInfo.ChannelMeta`。之后 adaptor 只依赖 `RelayInfo`，不需要到处读 Gin context。

### 3. 模型映射

```go
err = helper.ModelMappedHelper(c, info, request)
```

这一步可能把客户端请求的 `model` 改成上游模型。读它时重点看：

- `OriginModelName`
- `UpstreamModelName`
- `PricingModelName`
- `IsModelMapped`

这四个字段分别服务于请求、上游、计费、日志。

### 4. adaptor 转换请求

如果不是 pass-through：

```go
convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
```

OpenAI adaptor 可能基本保持 OpenAI 格式；Claude adaptor 会转成 Claude Messages；Gemini adaptor 会转成 Gemini 格式。

Go 学习点：主流程调用 interface 方法，不关心具体类型。具体 provider 差异由动态分派解决。

### 5. Marshal、字段裁剪、参数覆盖

`TextHelper()` 会：

1. `common.Marshal(convertedRequest)`。
2. `relaycommon.RemoveDisabledFields(...)`。
3. `relaycommon.ApplyParamOverrideWithRelayInfo(...)`。
4. `relaycommon.NewOutboundJSONBody(jsonData)`。

这段展示了 Go 中“结构化对象 -> JSON bytes -> bytes 级处理 -> io.Reader”的过程。

### 6. 响应处理

非流式 OpenAI 响应走 `OpenaiHandler()`：

1. `io.ReadAll(resp.Body)`。
2. `common.Unmarshal(responseBody, &simpleResponse)`。
3. 检查上游 error。
4. 如果 usage 缺失，用本地估算补齐。
5. 根据下游 RelayFormat 可能转 Claude/Gemini 响应。
6. `service.IOCopyBytesGracefully(c, resp, responseBody)` 写回客户端。
7. 返回 usage 给计费。

### 练习

在 `TextHelper()` 中标出所有可能 return error 的位置，并分类：

- 请求转换错误。
- JSON 错误。
- 上游 HTTP 错误。
- 响应解析错误。
- 计费前不会出现在这里的错误。

## 五、上游 HTTP 请求链路：URL、Header、Body、Client

### 全局路径

```text
adaptor.DoRequest()
  -> channel.DoApiRequest()
  -> adaptor.GetRequestURL()
  -> http.NewRequest()
  -> adaptor.SetupRequestHeader()
  -> processHeaderOverride()
  -> doRequest()
```

### 1. DoApiRequest 的职责

文件：`relay/channel/api_request.go`

这个函数是上游 HTTP 请求的统一出口。读它时关注四件事：

- URL 从哪里来。
- header 如何构造。
- body 如何传入。
- client/proxy 如何选择。

provider 自己通常只负责 `GetRequestURL()` 和 `SetupRequestHeader()`，真正发 HTTP 请求由公共函数处理。

### 2. ContentLength 细节

`applyUpstreamContentLength()` 说明了一个 Go HTTP 细节：

```text
net/http.NewRequest 只有在 body 是 *bytes.Reader、*bytes.Buffer、*strings.Reader 时自动识别 ContentLength。
如果 body 被类型擦除成 io.Reader，就可能变成 chunked transfer。
```

所以项目在 `RelayInfo.UpstreamRequestBodySize` 中保存 body 大小，再手动设置 `req.ContentLength`。

这是读源码时很值得记住的 Go 经验：interface 抽象有时会丢失具体类型信息，导致标准库无法自动推断。

### 3. Header passthrough 和 override

`processHeaderOverride()` 支持：

- 普通 header override。
- `{api_key}` 占位符。
- `{client_header:<name>}` 从客户端请求头取值。
- `*` 通配透传。
- `re:` / `regex:` 正则透传。

同时会跳过不安全 header：

- hop-by-hop headers。
- cookie。
- host。
- content-length。
- authorization。
- x-api-key。
- WebSocket 握手 header。

Go 学习点：

- map 表示集合：`map[string]struct{}{}`。
- `sync.Map` 缓存正则编译结果。
- 字符串规范化用 `strings.ToLower`、`strings.TrimSpace`。

### 练习

解释为什么 wildcard header passthrough 不能透传 `authorization`。从安全和多 provider key 两个角度回答。

## 六、文本计费结算链路：usage 到 quota 到日志

### 全局路径

```text
OpenaiHandler 返回 *dto.Usage
  -> service.PostTextConsumeQuota()
  -> calculateTextQuotaSummary()
  -> TryTieredSettle()
  -> SettleBilling()
  -> GenerateTextOtherInfo()
  -> model.RecordConsumeLog()
  -> perfmetrics.RecordRelaySample()
```

### 1. PostTextConsumeQuota 的输入

文件：`service/text_quota.go`

函数签名：

```go
func PostTextConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string)
```

输入包含：

- Gin context：读取请求侧状态和日志上下文。
- RelayInfo：用户、token、渠道、计费 session、价格数据。
- Usage：上游返回或本地估算的 token 用量。
- extraContent：补充日志说明。

### 2. usage 为空或不可信

开头处理：

```go
originUsage := usage
if usage == nil {
    extraContent = append(extraContent, "上游无计费信息")
}
clearUntrustedCachedTokens(ctx, usage)
```

读法：计费代码不能默认上游永远返回正确 usage，所以先处理缺失和不可信缓存 token。

### 3. 计算 summary

```go
summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
```

summary 是把复杂计费变量整理成一个中间结果：

- prompt/completion tokens。
- model ratio / model price。
- group ratio。
- cache/image/audio/web search/file search 等附加计费。
- total quota。
- Claude/OpenAI usage 语义。

Go 学习点：当函数需要产出很多中间值时，用 struct summary 比返回十几个值更清晰。

### 4. tiered expression 分支

如果请求使用 tiered billing：

```go
tieredOk, tieredQuota, tieredRes := TryTieredSettle(...)
if tieredOk {
    summary.Quota = composeTieredTextQuota(...)
}
```

这是 Go 中常见模式：

- `ok` 表示这条逻辑是否适用。
- `quota` 是结果。
- `tieredRes` 是可选详情。

### 5. 更新用量与结算

如果 `summary.TotalTokens == 0`：

- quota 设为 0。
- 记录错误日志说明无法扣费。

否则：

```go
model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, summary.Quota)
model.UpdateChannelUsedQuota(relayInfo.ChannelId, summary.Quota)
```

然后：

```go
if err := SettleBilling(ctx, relayInfo, summary.Quota); err != nil {
    logger.LogError(ctx, "error settling billing: "+err.Error())
}
```

这里把“统计 used quota”和“实际资金结算”分开：

- used quota 是历史统计。
- BillingSession 负责预扣费差额补扣/返还。

### 6. 生成日志 other

根据 usage 语义生成：

- `GenerateClaudeOtherInfo(...)`
- `GenerateTextOtherInfo(...)`

再补充：

- reject reason。
- image/web search/file search/audio/cache details。
- tiered billing info。
- channel affinity 相关字段。

最后：

```go
model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{...})
```

### 练习

拿一个带 cache read 的 Claude 响应，思考：

1. `summary.IsClaudeUsageSemantic` 应该是什么？
2. cache read token 会写入哪些 other 字段？
3. tiered expression 中 `len` 和 `p` 为什么可能不同？

## 七、系统任务链路：定时任务、DB lease、goroutine

### 全局路径

```text
main.go
  -> controller.RegisterScheduledSystemTasks()
  -> service.StartSystemTaskRunner()
  -> runSystemTaskScheduler()
  -> model.CreateSystemTask()
  -> runSystemTaskClaimPass()
  -> model.ClaimSystemTask()
  -> runWithLeaseHeartbeat()
  -> handler.Run()
  -> model.FinishSystemTask()
```

### 1. 注册 handler

文件：`controller/system_task_handlers.go`

```go
func RegisterScheduledSystemTasks() {
    service.RegisterSystemTaskHandler(channelTestHandler{})
    service.RegisterSystemTaskHandler(modelUpdateHandler{})
    service.RegisterSystemTaskHandler(midjourneyPollHandler{})
    service.RegisterSystemTaskHandler(asyncTaskPollHandler{})
}
```

每个 handler 实现接口：

```go
Type() string
Enabled() bool
Interval() time.Duration
NewPayload() any
Run(ctx context.Context, task *model.SystemTask, runnerID string)
```

Go 学习点：这些 struct 没有显式声明“implements”，只要方法集满足接口，就自动实现。

### 2. runner 单例

文件：`service/system_task.go`

```go
var systemTaskRunnerOnce sync.Once
```

`StartSystemTaskRunner()` 用 `sync.Once` 保证同一进程只启动一次 runner。

如果不是 master 节点，直接 return。这是分布式部署下的第一层约束。

### 3. scheduler 和 claim

runner 每次 pass：

1. 清理 stale lock。
2. 调 `runSystemTaskScheduler()` 创建到期任务。
3. 调 `runSystemTaskClaimPass(runnerID)` 抢 pending task。

claim 成功后：

```go
gopool.Go(func() {
    runWithLeaseHeartbeat(dispatchTask, runnerID, func(ctx context.Context) {
        dispatchHandler.Run(ctx, dispatchTask, runnerID)
    })
})
```

Go 学习点：

- 闭包捕获变量时，代码先复制 `dispatchHandler := handler`，避免循环变量捕获问题。
- 每个任务独立 goroutine，互不阻塞。
- `ctx` 传给 handler，允许 lease 丢失时取消任务。

### 4. heartbeat

`runWithLeaseHeartbeat()`：

- 创建可 cancel context。
- ticker 周期续租。
- fn 执行完后关闭 done。
- 如果续租失败，cancel context。

这是一段很适合学习 Go 并发控制的代码：ticker、channel、context、goroutine、defer 同时出现。

### 练习

解释为什么系统任务需要 DB lease，而不是只靠 `common.IsMasterNode`。提示：考虑多个 master、进程崩溃、任务运行时间超过轮询间隔。

## 八、读源码的断点清单

如果你使用 GoLand 或 Delve 调试，可以按下面断点练习。

### 登录断点

1. `controller.Login`
2. `model.User.ValidateAndFill`
3. `controller.setupLogin`
4. `model.RecordLoginLog`

观察：

- `loginRequest` 如何从 body 填充。
- `user` 在 `ValidateAndFill` 前后字段有什么变化。
- session 中写入了哪些 key。

### relay 断点

1. `middleware.TokenAuth`
2. `middleware.Distribute`
3. `controller.Relay`
4. `relay.TextHelper`
5. `relay/channel/openai.(*Adaptor).DoResponse`
6. `service.PostTextConsumeQuota`

观察：

- Gin context 中逐步增加了哪些 key。
- `RelayInfo` 何时形成。
- `OriginModelName` 和 `UpstreamModelName` 是否相同。
- `FinalPreConsumedQuota` 和最终 `summary.Quota` 是否相同。

### 渠道选择断点

1. `middleware.getModelRequest`
2. `service.CacheGetRandomSatisfiedChannel`
3. `model.GetRandomSatisfiedChannelWithExclusions`
4. `middleware.SetupContextForSelectedChannel`

观察：

- `RetryParam` 的 retry 值。
- excluded map 中有哪些渠道。
- auto group 是否改变了实际分组。
- selected channel 的 priority 和 weight。

### 系统任务断点

1. `service.StartSystemTaskRunner`
2. `service.runSystemTaskScheduler`
3. `service.runSystemTaskClaimPass`
4. `service.runWithLeaseHeartbeat`
5. 任意 handler 的 `Run`

观察：

- runnerID 如何生成。
- task 从 pending 到 running 再到 succeeded/failed。
- heartbeat 续租间隔。
- context 何时取消。

