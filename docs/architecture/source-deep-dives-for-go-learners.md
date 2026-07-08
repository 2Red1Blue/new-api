# new-api 源码深挖专题

这份文档把 new-api 按四条“读完就能串起来”的主线拆开讲：

1. 启动、初始化、配置、数据库、缓存、后台任务
2. Relay 请求全链路、渠道选择、provider adaptor、重试与错误处理
3. 计费、预扣费、结算、日志、订阅/钱包、tiered billing expression
4. `web/default` 前端、后台管理 API、用户/令牌/渠道管理

它适合已经读过 `go-source-learning-guide.md` 和 `source-walkthroughs-for-go-learners.md` 后继续深入。读法建议是：先按专题看调用链，再打开对应源码跟跳，最后用每节末尾的 Go 学习点复盘。

## 一、启动、配置、数据库、缓存和后台任务

### 1.1 入口文件和责任边界

启动入口在 `main.go`。

`main()` 做运行期编排：

- 调用 `InitResources()` 初始化基础资源。
- 根据环境决定 Gin mode。
- 启动内存渠道缓存、配置同步、权限同步、数据看板、订阅重置、系统实例上报、系统任务 runner 等后台任务。
- 创建 `gin.New()`，挂全局 middleware 和 session。
- 调用 `router.SetRouter()` 注册所有 HTTP 路由。
- 创建 `http.Server` 并 `ListenAndServe()`。
- 等待 `SIGINT` / `SIGTERM`，调用 `srv.Shutdown()` 优雅退出。
- 退出前在数据看板开启时调用 `model.SaveQuotaDataCache()`，最后关闭 DB。

`InitResources()` 做资源准备：

```text
InitResources()
  -> godotenv.Load(".env")
  -> common.InitEnv()
  -> logger.SetupLogger()
  -> ratio_setting.InitRatioSettings()
  -> service.InitHttpClient()
  -> service.InitTokenEncoders()
  -> model.InitDB()
  -> authz.Init(model.DB)
  -> model.CheckSetup()
  -> model.InitOptionMap()
  -> common.CleanupOldCacheFiles()
  -> model.GetPricing()
  -> model.InitLogDB()
  -> common.InitRedisClient()
  -> perfmetrics.Init()
  -> common.StartSystemMonitor()
  -> i18n.Init()
  -> i18n.SetUserLangLoader(model.GetUserLanguage)
  -> oauth.LoadCustomProviders()
```

一个好用的理解方式：`InitResources()` 是“HTTP server 启动前必须准备好”的资源；`main()` 后半段是“进程开始服务后需要常驻运行”的任务。

### 1.2 关键文件索引

| 模块 | 入口 | 读什么 |
| --- | --- | --- |
| 进程启动 | `main.go` | `main()`、`InitResources()`、优雅退出 |
| 环境变量 | `common/init.go` | `common.InitEnv()` 解析 flag/env |
| 日志 | `logger/logger.go` | `logger.SetupLogger()` |
| HTTP client | `service/http_client.go` | `service.InitHttpClient()` |
| Tokenizer | `service/tokenizer.go` | `service.InitTokenEncoders()` |
| 主数据库 | `model/main.go` | `model.InitDB()`、`chooseDB()`、`migrateDB()` |
| 日志库 | `model/main.go` | `model.InitLogDB()`、`migrateLOGDB()` |
| Option | `model/option.go` | `InitOptionMap()`、`SyncOptions()`、`updateOptionMap()` |
| 配置注册 | `setting/config/config.go` | `ConfigManager.Register()`、`ExportAllConfigs()` |
| 倍率配置 | `setting/ratio_setting/*.go` | 模型倍率、模型价格、分组倍率 |
| 定价缓存 | `model/pricing.go` | `model.GetPricing()`、`InvalidatePricingCache()` |
| Redis | `common/redis.go` | `common.InitRedisClient()` |
| 渠道缓存 | `model/channel_cache.go` | `InitChannelCache()`、`SyncChannelCache()` |
| 系统任务 | `service/system_task.go` | `StartSystemTaskRunner()` |
| 系统任务注册 | `controller/system_task_handlers.go` | `RegisterScheduledSystemTasks()` |

### 1.3 配置加载的三层来源

第一层是代码默认值。默认值分散在 `common`、`setting/*`、`setting/ratio_setting`。例如模型倍率和价格先由 `ratio_setting.InitRatioSettings()` 加载到内存 map；一些分层配置通过包内 `init()` 注册到 `config.GlobalConfig`。

第二层是环境变量。`common.InitEnv()` 读取 `DEBUG`、`NODE_TYPE`、`SYNC_FREQUENCY`、限流、超时、SQL/Redis 之外的运行参数等。大多数环境变量只在启动时生效，不走热更新。

第三层是 DB `options` 表。`model.InitOptionMap()` 先把默认值写入 `common.OptionMap`，再调用 `loadOptionsFromDatabase()` 从数据库覆盖。数据库中每个 option 最终都会进入 `updateOptionMap()`，由它把字符串配置分发给 `common`、`setting`、`ratio_setting` 等包。

分层配置形如：

- `performance_setting.disk_cache_enabled`
- `theme.frontend`
- `billing_setting.billing_mode`
- `billing_setting.billing_expr`

这类 key 由 `handleConfigUpdate()` 处理：先根据配置名前缀找到注册结构体，再通过反射写入字段。部分配置还有后处理，例如 billing 配置更新后会让定价缓存失效，theme 配置会同步到当前主题。

### 1.4 数据库初始化和迁移

主库由 `model.InitDB()` 初始化。核心流程：

1. `chooseDB("SQL_DSN", false)` 根据 DSN 选择 SQLite、MySQL 或 PostgreSQL。
2. 调用 `common.SetMainDatabaseType()` 保存数据库类型。
3. `initCol()` 根据方言设置 `group`、`key` 等保留字列的引用方式，以及布尔值 SQL 表达方式。
4. 配置 GORM 连接池。
5. 非 master 节点只连接数据库，不执行迁移。
6. master 节点执行 `migrateDB()`。

`migrateDB()` 使用 GORM `AutoMigrate` 迁移大多数表，包括 `Channel`、`Token`、`User`、`Option`、`Ability`、`Log`、`Task`、`Model`、`Vendor`、订阅相关表、OAuth 表、性能指标表、上游身份表、系统实例表、系统任务表、Casbin 表等。

SQLite 有一些特殊迁移逻辑。原因是 SQLite 对 `ALTER COLUMN` 支持很弱，所以项目里会用 `CREATE TABLE` 和 `ALTER TABLE ADD COLUMN` 这种兼容写法。读迁移时要特别留意 `common.UsingMainDatabase(...)`、`common.UsingLogDatabase(...)` 这类方言分支。

日志库由 `model.InitLogDB()` 初始化。如果未配置 `LOG_SQL_DSN`，日志库复用主库；如果单独配置，可以是 SQLite/MySQL/PostgreSQL，也可以是 ClickHouse。ClickHouse 只允许作为日志库，不允许作为主库。

### 1.5 缓存和周期同步

Redis 由 `common.InitRedisClient()` 初始化。没配置 `REDIS_CONN_STRING` 时 `RedisEnabled=false`；配置后会 ping 并设置连接池。Redis 主要用于用户、token 等缓存，并不是所有缓存都进 Redis。

渠道路由缓存仍然是进程内缓存。`model.InitChannelCache()` 从 DB 读取 channels 和 abilities，构建 `group -> model -> channel ids`，并按 priority 排序。`main()` 中如果 Redis 开启，会强制启用 `MemoryCacheEnabled`，所以 Redis 场景也会有进程内渠道缓存。

周期同步任务包括：

- `go model.SyncChannelCache(common.SyncFrequency)`：刷新渠道缓存。
- `go model.SyncOptions(common.SyncFrequency)`：刷新 DB options 到内存。
- `go authz.StartPolicySync(common.SyncFrequency)`：刷新权限策略。
- `go model.UpdateQuotaData()`：刷新数据看板聚合。

### 1.6 系统任务 runner

`controller.RegisterScheduledSystemTasks()` 注册周期任务，例如渠道测试、上游模型更新检测、Midjourney 轮询、异步任务轮询。

`service.StartSystemTaskRunner()` 是正式的系统任务调度器。它只在 master 节点运行，整体流程是：

```text
StartSystemTaskRunner()
  -> runSystemTaskScheduler()
     -> 判断 scheduled handler 是否到期
     -> 创建 system_tasks pending 行
  -> runSystemTaskClaimPass()
     -> 为每种任务类型领取最早 pending 任务
     -> model.ClaimSystemTask()
        -> system_task_locks 做租约
     -> runWithLeaseHeartbeat()
        -> 周期续租
        -> 租约丢失则取消 context
     -> handler.Handle(ctx, task)
     -> model.FinishSystemTask()
```

这里有两层去重：`SystemTask.ActiveKey` 防止同类型 active row 重复创建；`SystemTaskLock` 防止多 master 并发执行同一类任务。

### 1.7 Go 学习点

- `//go:embed`：`main.go` 把 `web/default/dist` 和 `web/classic/dist` 打包进二进制。
- blank import：`_ "github.com/QuantumNous/new-api/setting/performance_setting"` 是为了触发包内 `init()`。
- goroutine：`go model.SyncOptions(...)` 这类常驻后台任务体现了 Go 服务端常见模式。
- `sync.RWMutex` / 泛型 map：`common.OptionMap` 和 `types.RWMap[K,V]` 展示了并发安全读写。
- `context.Context`：系统任务 handler 用 context 做取消传播。
- `defer`：关闭数据库、释放锁、取消 context、panic recovery 都大量使用。

## 二、Relay、渠道选择、provider adaptor 和重试

### 2.1 从路由到 controller

Relay 路由集中在 `router/relay-router.go`。全局 relay middleware 包括：

- `middleware.CORS()`
- `middleware.DecompressRequestMiddleware()`
- `middleware.BodyStorageCleanup()`
- `middleware.StatsMiddleware()`

`/v1` 路由再挂：

- `middleware.SystemPerformanceCheck()`
- `middleware.TokenAuth()`
- `middleware.ModelRequestRateLimit()`

HTTP 子路由挂 `middleware.Distribute()` 后按路径进入 `controller.Relay()`：

| 路径 | RelayFormat | 说明 |
| --- | --- | --- |
| `/v1/chat/completions` | `RelayFormatOpenAI` | OpenAI 兼容文本 |
| `/v1/completions` | `RelayFormatOpenAI` | completions |
| `/v1/messages` | `RelayFormatClaude` | Claude Messages |
| `/v1/responses` | `RelayFormatOpenAIResponses` | OpenAI Responses |
| `/v1/images/generations` | `RelayFormatOpenAIImage` | 图片生成 |
| `/v1/audio/transcriptions` | `RelayFormatOpenAIAudio` | 音频转写 |
| `/v1/embeddings` | `RelayFormatEmbedding` | embeddings |
| `/v1/rerank` | `RelayFormatRerank` | rerank |
| `/v1beta/models/*path` | `RelayFormatGemini` | Gemini |
| `/v1/realtime` | `RelayFormatOpenAIRealtime` | WebSocket realtime |

### 2.2 `controller.Relay()` 主编排

`controller.Relay()` 是中转请求最重要的总控函数：

```text
controller.Relay(c, relayFormat)
  -> helper.GetAndValidateRequest(c, relayFormat)
  -> relaycommon.GenRelayInfo(c, relayFormat, request, ws)
  -> request.GetTokenCountMeta()
  -> service.CheckSensitiveText()
  -> service.EstimateRequestToken()
  -> helper.ModelPriceHelper()
  -> service.PreConsumeBilling()
  -> for retry <= RetryTimes
     -> getChannel()
     -> common.GetBodyStorage()
     -> c.Request.Body = io.NopCloser(bodyStorage)
     -> 按 relayFormat 分派 Helper
     -> 成功则 return
     -> processChannelError()
     -> shouldRetry()
  -> defer 统一写错误响应或退款
```

这个函数里最值得学习的是两个 `defer`：

- 顶部 defer 负责把 `newAPIError` 转成 OpenAI/Claude/WebSocket 错误响应。
- 预扣费后的 defer 负责请求失败时调用 `relayInfo.Billing.Refund(c)`，再根据错误类型收取违规费用。

### 2.3 TokenAuth：你是谁，能用什么

`middleware.TokenAuth()` 负责 relay API key 鉴权。它兼容多种客户端：

- OpenAI 风格：`Authorization: Bearer sk-...`
- Claude 风格：`x-api-key`
- Gemini 风格：`x-goog-api-key` 或 query `key`
- WebSocket realtime：`Sec-WebSocket-Protocol`

鉴权后会检查：

- token 是否存在、是否过期、是否启用
- 用户是否存在、是否启用
- IP 限制
- token group 和用户 group
- token model limits
- special channel id / specific channel id

最终会把这些信息写入 Gin context，例如 `id`、`token_id`、`token_group`、`using_group`、`specific_channel_id`、`token_name`、`token_quota`。

这里要分清：`TokenAuth()` 只解决“身份和可用范围”，不负责真正调用 provider。

### 2.4 Distribute：预选渠道

`middleware.Distribute()` 读取请求中的 model 和 group，然后选择一个初始渠道。

核心步骤：

1. `getModelRequest()` 从 body、路径或协议格式中提取模型名。
2. 检查 token 模型白名单。
3. 如果指定了具体渠道，直接使用。
4. 尝试 channel affinity：`service.GetPreferredChannelByAffinity()`。
5. 否则调用 `service.CacheGetRandomSatisfiedChannel()` 选择渠道。
6. `SetupContextForSelectedChannel()` 把渠道信息写入 context。

`SetupContextForSelectedChannel()` 写入的值很多，包括渠道 ID、类型、base URL、key、model mapping、param override、header override、多 key index、stream options 支持情况等。后续 `RelayInfo.InitChannelMeta()` 会从这些 context 值复制到 `RelayInfo.ChannelMeta`。

### 2.5 RetryParam 和 getChannel 状态机

重试参数是 `service.RetryParam`：

```go
type RetryParam struct {
    Ctx         *gin.Context
    TokenGroup  string
    ModelName   string
    RequestPath string
    Retry       *int
    Excluded    map[int]struct{}
    Failures    map[int]int
}
```

`controller.getChannel()` 有一个容易误解的状态：

- 首次请求时 `RelayInfo.ChannelMeta == nil`，通常复用 `Distribute()` 已经写入 context 的渠道。
- `TextHelper()` 等 helper 会调用 `info.InitChannelMeta(c)`，把 context 渠道固化到 `RelayInfo`。
- 后续 retry 时，如果需要重新选渠道，会把 `ChannelMeta` 设为空对象或 nil，然后调用 `CacheGetRandomSatisfiedChannel()`。

同一请求中，如果一个渠道连续失败达到 `perRequestChannelFailureLimit = 4`，会被加入 `retryParam.Excluded`。模型不存在这种错误会立即排除当前渠道。

### 2.6 限流：用户请求限流和渠道 RPM 限流

`middleware.ModelRequestRateLimit()` 是用户/分组/模型维度请求限流。它支持 Redis 和内存实现，请求结束后根据响应状态决定是否记录成功请求。

`service.CheckAndReserveChannelRPM()` 是渠道上游 RPM 限流。它看的是 channel 自身的 `OtherSettings.UpstreamRPMLimit`，用于保护某个上游渠道不要超过每分钟请求数。

这两者不是一套东西：前者限制用户访问 gateway，后者限制 gateway 打上游 provider。

### 2.7 Adaptor interface 和 provider 实现

Provider 适配统一由 `relay/channel/adapter.go` 定义：

```go
type Adaptor interface {
    Init(info *relaycommon.RelayInfo)
    GetRequestURL(info *relaycommon.RelayInfo) (string, error)
    SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
    ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
    DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
    DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
    GetModelList() []string
    GetChannelName() string
    ...
}
```

`relay.GetAdaptor(info.ApiType)` 是工厂函数，根据 APIType 返回 `openai.Adaptor`、`claude.Adaptor`、`gemini.Adaptor`、`aws.Adaptor` 等。

`relay.TextHelper()` 的共性流程是：

```text
TextHelper()
  -> info.InitChannelMeta(c)
  -> request := DeepCopy(textReq)
  -> helper.ModelMappedHelper()
  -> 处理 StreamOptions
  -> adaptor := GetAdaptor(info.ApiType)
  -> adaptor.Init(info)
  -> adaptor.ConvertOpenAIRequest() 或透传 body
  -> common.Marshal(convertedRequest)
  -> RemoveDisabledFields()
  -> ApplyParamOverrideWithRelayInfo()
  -> adaptor.DoRequest()
  -> service.RelayErrorHandler() 处理非 200
  -> adaptor.DoResponse()
  -> service.PostTextConsumeQuota()
```

OpenAI adaptor 要处理 Azure、Realtime、Responses、OpenRouter 等 URL/header 差异。Claude adaptor 把 OpenAI 请求转成 Claude Messages，并写 `x-api-key`、`anthropic-version`。Gemini adaptor 支持 OpenAI/Claude 转 Gemini，并根据 `generateContent`、`streamGenerateContent`、`embedContent` 等动作构造 URL。

### 2.8 流式和非流式响应

非流式 handler 一般会：

1. `io.ReadAll(resp.Body)` 读完整上游响应。
2. 用 `common.Unmarshal()` 解析。
3. 必要时转换成下游格式。
4. 写回 downstream。
5. 从上游 usage 或本地估算 usage 做结算。

流式 handler 会设置 SSE 相关 header，并使用 `relay/helper/stream_scanner.go` 的 scanner 逐行处理 `data:`。它要同时处理：

- `[DONE]`
- 上游超时
- 客户端断开
- ping 保活
- 并发写锁
- usage 累积
- 下游已经写出后不能再返回 JSON 错误

### 2.9 Go 学习点

- `interface`：`channel.Adaptor` 是 provider 多态的核心。
- 类型断言：`info.Request.(*dto.GeneralOpenAIRequest)`、`resp.(*http.Response)`、`usage.(*dto.Usage)`。
- `io.Reader`：请求 body 是一次性流，所以项目用 `common.BodyStorage` 支持重试时重复读取。
- `*gin.Context`：业务状态通过 context 在 middleware、controller、relay helper 间传递。
- `context.Context`：流式扫描和上游请求用它感知取消。
- `defer`：错误响应、失败退款、响应体关闭都依赖 defer 保证执行。

## 三、计费、预扣费、结算和日志

### 3.1 计费主线

普通文本请求的计费从 `controller.Relay()` 开始：

```text
service.EstimateRequestToken()
  -> relayInfo.SetEstimatePromptTokens(tokens)
  -> helper.ModelPriceHelper()
     -> 生成 PriceData
  -> service.PreConsumeBilling()
     -> NewBillingSession()
     -> BillingSession.preConsume()
  -> relay.TextHelper()
     -> adaptor.DoResponse()
     -> service.PostTextConsumeQuota()
        -> calculateTextQuotaSummary()
        -> service.SettleBilling()
        -> model.RecordConsumeLog()
```

失败路径由 `controller.Relay()` 的 defer 收口：

```text
newAPIError != nil
  -> service.NormalizeViolationFeeError()
  -> relayInfo.Billing.Refund(c)
  -> service.ChargeViolationFeeIfNeeded()
```

### 3.2 PriceData 如何生成

`relay/helper/price.go` 的 `ModelPriceHelper()` 生成 `types.PriceData`。它先确定计费模型名：

- 默认使用 `relayInfo.OriginModelName`
- 如果 `relayInfo.PricingModelName` 不为空，使用它

随后 `HandleGroupRatio()` 处理分组倍率：

- 如果 context 中有 `auto_group`，会更新 `relayInfo.UsingGroup`
- 如果配置了用户组到渠道组的特殊倍率，优先使用特殊倍率
- 否则使用普通 group ratio

普通计费分两类：

- 按 token 倍率：读取 `ModelRatio`、`CompletionRatio`、`CacheRatio`、`ImageRatio`、`AudioRatio`、`AudioCompletionRatio`，预扣大致是 `(max(promptTokens, PreConsumedQuota) + max_tokens) * modelRatio * groupRatio`。
- 固定价格：读取 `ModelPrice`，预扣是 `modelPrice * common.QuotaPerUnit * groupRatio`。

如果模型走 `billing_setting.BillingModeTieredExpr`，则进入 `modelPriceHelperTiered()`。

任务、MJ 等按次计费使用 `ModelPriceHelperPerCall()`，结果主要放在 `PriceData.Quota`；普通文本/音频预扣使用 `PriceData.QuotaToPreConsume`。

### 3.3 BillingSession 生命周期

统一入口是 `service.PreConsumeBilling()`：

```text
PreConsumeBilling(c, quota, relayInfo)
  -> NewBillingSession(c, relayInfo, quota)
  -> relayInfo.Billing = session
```

`BillingSession` 定义在 `service/billing_session.go`，它保存一次请求的计费状态：

- `funding`：资金来源，钱包或订阅。
- `preConsumedQuota`：实际预扣额度。
- `tokenConsumed`：令牌额度实际扣减量。
- `extraReserved`：请求发送前额外补充预留。
- `trusted`：是否命中信任额度旁路。
- `fundingSettled`：资金来源是否已经提交结算。
- `settled`：整个 session 是否已结算。
- `refunded`：是否已退款。
- `mu sync.Mutex`：防止重复结算和重复退款。

`FundingSource` 接口在 `service/funding_source.go`：

```go
type FundingSource interface {
    Source() string
    PreConsume(amount int) error
    Settle(delta int) error
    Refund() error
}
```

实现有：

- `WalletFunding`：调整用户钱包 quota。
- `SubscriptionFunding`：调整订阅预扣记录和订阅已用额度。

`NewBillingSession()` 会根据用户计费偏好决定优先用订阅还是钱包。偏好包括 `subscription_first`、`wallet_first`、`subscription_only`、`wallet_only`。默认是订阅优先。

### 3.4 结算和退款如何保证

正常响应后，`PostTextConsumeQuota()` 或 `PostAudioConsumeQuota()` 会根据真实 usage 计算实际 quota，然后调用 `SettleBilling()`。

`SettleBilling()` 的逻辑：

```text
if relayInfo.Billing != nil:
  preConsumed := relayInfo.Billing.GetPreConsumedQuota()
  delta := actualQuota - preConsumed
  relayInfo.Billing.Settle(actualQuota)
  发送额度提醒
else:
  fallback PostConsumeQuota()
```

`BillingSession.Settle()` 分两步：

1. 调整资金来源：`funding.Settle(delta)`
2. 调整 token quota：`DecreaseTokenQuota` 或 `IncreaseTokenQuota`

如果资金来源已经结算但 token quota 调整失败，session 会记录 `fundingSettled`，防止后续 `Refund()` 把已经提交的资金来源误退。

失败退款由 `BillingSession.Refund()` 处理。它先在锁内检查 `settled/refunded/fundingSettled`，然后异步执行退款：

1. `funding.Refund()`
2. 如果订阅额外预留过，回滚额外预留。
3. 如果 token quota 扣过，增加 token quota。

订阅退款还在 model 层有幂等记录：`SubscriptionPreConsumeRecord` 使用 `request_id` 做唯一请求记录，退款时会在事务中锁记录并标记 refunded。

### 3.5 tiered_expr 计费表达式

读 tiered billing 必须先看 `pkg/billingexpr/expr.md`。它的核心思想是：一条表达式就是计费真相。表达式系数是 `$ / 1M tokens`，系统负责把 token 归一化、执行表达式、转换成 quota。

主要源码落点：

| 责任 | 文件 | 核心 |
| --- | --- | --- |
| 配置 | `setting/billing_setting/tiered_billing.go` | `BillingModeTieredExpr`、`billing_expr` |
| 编译缓存 | `pkg/billingexpr/compile.go` | `CompileFromCache()`、表达式 hash、`UsedVars` |
| 执行 | `pkg/billingexpr/run.go` | `RunExprWithRequest()`、变量环境 |
| 预扣 | `relay/helper/price.go` | `modelPriceHelperTiered()` |
| 结算 | `service/tiered_settle.go` | `BuildTieredTokenParams()`、`TryTieredSettle()` |
| quota 计算 | `pkg/billingexpr/settle.go` | `ComputeTieredQuotaWithRequest()` |
| 日志 | `service/log_info_generate.go` | `InjectTieredBillingInfo()` |

预扣时，`modelPriceHelperTiered()` 用估算 prompt tokens 和 estimated completion tokens 运行表达式，生成 `BillingSnapshot`。这个 snapshot 会冻结表达式字符串、hash、group ratio、估算 token、估算 tier、quota per unit 等。

结算时，`TryTieredSettle()` 使用冻结快照和真实 usage 重算 quota。这样做的意义是：即使管理员在请求期间修改了表达式，本次请求仍按发起时的表达式结算。

### 3.6 consume log 和 error log

消费日志入口：

- 文本：`service.PostTextConsumeQuota()` 调 `model.RecordConsumeLog()`
- 音频：`service.PostAudioConsumeQuota()` 调 `model.RecordConsumeLog()`
- 任务：`service/task_billing.go` 的 `LogTaskConsumption()`

`model.RecordConsumeLog()` 写入 `LogTypeConsume`，受 `common.LogConsumeEnabled` 控制。

错误日志入口在 `controller.processChannelError()`。当 `constant.ErrorLogEnabled` 且 `types.IsRecordErrorLog(err)` 时，会构造 `other`，包含 error type/code、status code、channel、admin info、request path 等，再调用 `model.RecordErrorLog()`。

日志 `other` 字段由 `service/log_info_generate.go` 的一组函数生成，里面会记录分组倍率、模型倍率、缓存信息、请求转换链、订阅计费信息、tiered_expr 信息等。

### 3.7 Go 学习点

- 接口多态：`FundingSource` 和 `BillingSettler` 比 `Adaptor` 更小，更适合理解 Go interface。
- 锁：`BillingSession.mu` 保护一次请求生命周期，避免重复结算/退款。
- `defer`：`controller.Relay()` 里的失败退款 defer 是计费安全的关键。
- decimal：额度计算使用 `shopspring/decimal` 降低浮点误差。
- 错误包装：业务错误用 `types.NewAPIError` 携带状态码、错误码、重试策略、日志策略。

## 四、前端、后台 API、用户/令牌/渠道管理

### 4.1 web/default 前端结构

`web/default` 是当前默认前端，技术栈在 `web/default/package.json`：

- React 19
- TypeScript
- Rsbuild
- Tailwind CSS
- Base UI
- TanStack Router / Query / Table / Virtual
- Zustand
- i18next
- React Hook Form
- Zod

构建入口在 `web/default/rsbuild.config.ts`。`source.entry.index` 指向 `./src/main.tsx`，`@` 指向 `src`，开发代理会把 `/api`、`/mj`、`/pg` 转发到后端。TanStack Router 插件生成 `routeTree.gen`。

应用入口是 `web/default/src/main.tsx`。它创建 `QueryClient` 和 `createRouter({ routeTree })`，再包裹：

- `QueryClientProvider`
- `ThemeProvider`
- `FontProvider`
- `DirectionProvider`
- `RouterProvider`

根路由在 `web/default/src/routes/__root.tsx`，负责 setup 状态判断、系统配置加载、导航进度、`Outlet`、`Toaster`。

认证路由在 `web/default/src/routes/_authenticated/route.tsx`。它先读 Zustand 的 `auth.user`，没有则跳 `/sign-in`；首次进入会调用 `getSelf()` 校验后端 session。

### 4.2 API client、状态和 i18n

API client 在 `web/default/src/lib/api.ts`：

- `axios.create({ baseURL: "", withCredentials: true })`
- GET 请求默认去重
- 请求头附加 `New-Api-User`
- 响应拦截统一处理 `{ success, message, data }`
- 401 会清理登录态

全局状态主要用 Zustand：

- 登录态：`web/default/src/stores/auth-store.ts`
- 系统配置：`web/default/src/stores/system-config-store.ts`

页面内局部状态通常放 feature provider，例如 `UsersProvider`、`ApiKeysProvider`、`ChannelsProvider`。

i18n 在 `web/default/src/i18n/config.ts`。语言包括 `en`、`zh`、`fr`、`ru`、`ja`、`vi`。组件文本按照 `web/default/AGENTS.md` 要求使用 `useTranslation()` 和 `t("English source string")`。

### 4.3 后端管理 API

后台 API 路由入口在 `router/api-router.go` 和 `router/channel-router.go`。

用户相关：

- 前端 API：`web/default/src/features/users/api.ts`
- 后端 controller：`controller/user.go`
- 后端 model：`model/user.go`

典型链路：

```text
web/default users page
  -> features/users/api.ts
  -> /api/user 或 /api/user/search
  -> router/api-router.go
  -> middleware.UserAuth/AdminAuth
  -> controller.GetAllUsers/SearchUsers/CreateUser/UpdateUser/ManageUser
  -> model.GetAllUsers/SearchUsers/InsertWithTx/EditWithTx
```

令牌相关：

- 前端 API：`web/default/src/features/keys/api.ts`
- 后端 controller：`controller/token.go`
- 后端 model：`model/token.go`

列表返回会使用 masked key；真实 key 通过 `/api/token/:id/key` 获取。

渠道相关：

- 前端 API：`web/default/src/features/channels/api.ts`
- 后端路由：`router/channel-router.go`
- 后端 controller：`controller/channel.go`
- 后端 model：`model/channel.go`
- 运行时 service：`service/channel.go`、`service/channel_select.go`

渠道管理里权限更细：`authz.ChannelRead`、`ChannelWrite`、`ChannelOperate`、`ChannelSensitiveWrite`，查看真实 key 和上游密码还要求 root 和安全验证。

### 4.4 前后端字段对应

通用响应结构：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

分页通常使用 `common.PageInfo`：

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 10
}
```

用户字段大体对应 `model.User` 的 JSON tag。前端表单会把 `quota_dollars` 这类 UI 展示字段转换为后端 quota units，不一定直接等于 DB 字段。

令牌字段对应 `model.Token`：`id`、`name`、`key`、`status`、`remain_quota`、`used_quota`、`unlimited_quota`、`expired_time`、`group`、`model_limits`、`allow_ips` 等。

渠道字段对应 `model.Channel`：`type`、`key`、`base_url`、`models`、`group`、`model_mapping`、`status_code_mapping`、`priority`、`weight`、`setting`、`settings`、`param_override`、`header_override`、`channel_info`、`upstream_profile` 等。

注意：创建渠道不是直接发完整 `Channel`，而是 `AddChannelRequest { mode, multi_key_mode, channel, upstream_profile }`；更新时还有 `key_mode` 支持多 key 追加或覆盖。

### 4.5 新增页面或接口的读源码路线

1. 前端先从 `web/default/src/routes/...` 找 `createFileRoute()`。
2. 找到对应 feature，例如 `features/users`、`features/keys`、`features/channels`。
3. 看 `api.ts` 和 `types.ts`，确认请求路径、参数名、响应结构。
4. 看 `lib/*form.ts` 和 Zod schema，理解 UI 表单值如何转换成 API payload。
5. 后端从 `router/api-router.go` 或 `router/channel-router.go` 找 URL 和 middleware。
6. 进入 `controller/*` 看参数校验、权限、响应。
7. 进入 `model/*` 或 `service/*` 看 GORM 查询、缓存失效、跨模型业务。
8. 如果涉及权限，补看 `service/authz` 和 `middleware.RequirePermission()`。

### 4.6 Go/React 联读学习点

- React 页面不是直接散落请求逻辑，而是 route -> feature -> provider/table/dialog -> api/types/form。
- TypeScript 类型、Zod schema、后端 model 是三套相关但不完全相同的结构。
- 后端 Gin handler 的生命周期是 middleware 写 context，controller 读 context 和请求，model/service 执行业务，最后统一返回。
- 敏感字段不要从列表接口直接返回。令牌 key、渠道 key 都有专门查看接口和额外权限。
- 后端新增 JSON 解析时要使用 `common.DecodeJson`、`common.Unmarshal`、`common.Marshal`，不要直接调用 `encoding/json` 的 marshal/unmarshal。

## 五、容易误解的总表

| 误解 | 实际情况 |
| --- | --- |
| `InitOptionMap()` 只是初始化默认值 | 它会先写默认值，再从 DB 覆盖，DB options 通常优先 |
| Redis 开启后所有缓存都进 Redis | 渠道路由缓存仍是进程内缓存，Redis 主要缓存用户/token 等 |
| slave 节点不连数据库 | slave 会连接数据库，只是不做迁移和多数 master-only 后台任务 |
| `Distribute()` 已经完成 relay | 它只是预选渠道，真正转换请求和调用上游在 relay helper/adaptor |
| retry 每次都会换渠道 | 首次通常复用 middleware 选中的渠道，只有特定状态才重选 |
| 用户限流和渠道 RPM 是一套 | 用户限流保护 gateway，渠道 RPM 保护上游 provider |
| `PassThroughBodyEnabled` 还会完整转换请求 | 透传时通常不走 adaptor 请求转换和参数 override |
| `StreamOptions` 总会发给上游 | 只有 stream 且渠道支持时保留，否则会清空 |
| 预扣费失败一定只影响钱包 | 资金来源可能是钱包也可能是订阅，由 `FundingSource` 决定 |
| 结算时用的是最新 tiered 表达式 | 使用请求开始时冻结的 `BillingSnapshot` |
| 前端类型等于后端 DB model | UI form、API payload、DB model 经常不同，需要看 transform |

## 六、建议的深挖顺序

1. 先读 `main.go` 和 `router/relay-router.go`，建立请求入口图。
2. 读 `middleware/auth.go` 和 `middleware/distributor.go`，理解 context 如何传递身份与渠道。
3. 读 `controller/relay.go`，把请求解析、预扣费、重试、错误响应串起来。
4. 读 `relay/compatible_handler.go` 和 `relay/channel/adapter.go`，理解 adaptor interface。
5. 选一个 provider，例如 `relay/channel/openai`，跟一次请求转换和响应处理。
6. 读 `relay/helper/price.go`、`service/billing.go`、`service/billing_session.go`、`service/text_quota.go`，理解计费闭环。
7. 读 `model/main.go`、`model/option.go`、`setting/ratio_setting`，理解配置和数据库。
8. 最后读 `web/default/src/main.tsx`、`routes`、`features/*/api.ts`，把前后端管理台串起来。
