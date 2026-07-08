# new-api 实现全景梳理

本文面向希望系统掌握 new-api 的开发者。读完后应能回答三类问题：

1. 项目启动后有哪些资源、任务、路由和前端资源被装配。
2. 一个 API 请求从客户端进入到上游 provider，再到计费、日志、重试，是怎样流动的。
3. 后台管理、数据库、配置、缓存、异步任务和前端控制台分别承担什么职责。

本文以当前仓库代码为准，重点引用关键文件和类型，便于继续深入阅读。

## 目录

- [一、整体心智模型](#一整体心智模型)
- [二、目录与模块边界](#二目录与模块边界)
- [三、启动流程](#三启动流程)
- [四、数据库、模型与迁移](#四数据库模型与迁移)
- [五、配置 Option 与 setting 系统](#五配置-option-与-setting-系统)
- [六、HTTP 路由体系](#六http-路由体系)
- [七、认证、权限与上下文](#七认证权限与上下文)
- [八、中继 relay 主流程](#八中继-relay-主流程)
- [九、渠道选择与重试](#九渠道选择与重试)
- [十、provider adaptor 体系](#十provider-adaptor-体系)
- [十一、模型映射、请求转换与响应转换](#十一模型映射请求转换与响应转换)
- [十二、计费体系](#十二计费体系)
- [十三、日志、审计与用量统计](#十三日志审计与用量统计)
- [十四、异步任务、视频、音乐和系统任务](#十四异步任务视频音乐和系统任务)
- [十五、用户、令牌、订阅与支付](#十五用户令牌订阅与支付)
- [十六、缓存、Redis 与性能能力](#十六缓存redis-与性能能力)
- [十七、前端 web/default](#十七前端-webdefault)
- [十八、常见开发改动入口](#十八常见开发改动入口)
- [十九、关键流程速查](#十九关键流程速查)
- [二十、精读索引与兼容性注意](#二十精读索引与兼容性注意)

## 一、整体心智模型

new-api 是一个 AI API 网关。它把 OpenAI、Claude、Gemini、Azure、AWS Bedrock、OpenRouter、Suno、Midjourney、视频生成等多种上游能力聚合成统一入口，同时负责账号、令牌、渠道、分组、计费、日志、后台管理和前端控制台。

后端最重要的分层是：

```text
router -> middleware -> controller -> relay/service -> model
```

其中：

- `router/` 决定请求属于后台 API、relay API、视频 API、前端静态资源还是 dashboard。
- `middleware/` 完成通用能力，如请求 ID、日志、i18n、CORS、TokenAuth、UserAuth、渠道分发、限流、审计。
- `controller/` 是 HTTP handler，负责解析请求和调用 service/relay。
- `relay/` 是 AI API 代理核心，按请求格式和渠道类型转换请求、发上游、处理响应。
- `service/` 是业务逻辑层，包含计费、渠道选择、任务轮询、权限、用量、通知、转换等。
- `model/` 是 GORM 模型和数据库访问层，同时承担部分缓存同步和迁移。
- `setting/` 维护运行时配置，很多配置来自 `options` 表并热加载到内存。
- `web/default/` 是当前默认 React 19 管理端。

最核心的 relay 请求链路可以简化为：

```text
客户端
  -> /v1/chat/completions 等 relay 路由
  -> TokenAuth 验证令牌和用户
  -> Distribute 读取 model 并选渠道
  -> controller.Relay 解析请求、敏感词、估算 token、预扣费
  -> relay.TextHelper/ImageHelper/... 初始化 adaptor
  -> adaptor.Convert* 转上游格式
  -> adaptor.DoRequest 发上游
  -> adaptor.DoResponse 转响应并写回客户端
  -> service.Post*ConsumeQuota 结算
  -> model.RecordConsumeLog 记录日志
```

后台管理 API 则通常是：

```text
前端 axios
  -> /api/*
  -> UserAuth/AdminAuth/RootAuth
  -> controller/*
  -> service/model/setting
  -> JSON: { success, message, data }
```

## 二、目录与模块边界

### 后端主目录

| 目录 | 职责 | 代表文件 |
| --- | --- | --- |
| `main.go` | 启动、资源初始化、Gin server、嵌入前端资源、后台任务 | `main.go` |
| `router/` | 路由注册 | `router/main.go`, `router/api-router.go`, `router/relay-router.go`, `router/web-router.go` |
| `middleware/` | 认证、渠道分发、限流、日志、审计、CORS、请求体复用 | `middleware/auth.go`, `middleware/distributor.go` |
| `controller/` | HTTP handler | `controller/relay.go`, `controller/user.go`, `controller/channel.go` |
| `relay/` | relay 编排、请求响应处理、adaptor 注册 | `relay/compatible_handler.go`, `relay/relay_adaptor.go` |
| `relay/channel/` | 各 provider 的 adaptor | `relay/channel/openai/`, `relay/channel/claude/`, `relay/channel/gemini/` |
| `service/` | 业务服务、计费、选择、任务、通知、转换 | `service/billing.go`, `service/channel_select.go`, `service/quota.go` |
| `model/` | 数据模型、DB 操作、迁移、缓存 | `model/main.go`, `model/channel.go`, `model/token.go`, `model/user.go` |
| `setting/` | 配置解析与内存状态 | `setting/ratio_setting/`, `setting/operation_setting/`, `setting/billing_setting/` |
| `common/` | 公共工具、环境变量、Redis、JSON、日志、quota 常量 | `common/env.go`, `common/json.go`, `common/redis.go` |
| `dto/` | 请求响应 DTO | `dto/openai_request.go`, `dto/claude.go`, `dto/gemini.go` |
| `types/` | 跨模块类型 | `types/error.go`, `types/price_data.go`, `types/relay_format.go` |
| `constant/` | 常量、渠道类型、上下文 key | `constant/channel.go`, `constant/context_key.go` |
| `i18n/` | 后端 i18n | `i18n/locales/` |
| `oauth/` | OAuth provider 注册与实现 | `oauth/registry.go`, `oauth/github.go`, `oauth/oidc.go` |
| `pkg/` | 内部包 | `pkg/billingexpr/`, `pkg/cachex/`, `pkg/perf_metrics/` |

### 前端主目录

| 目录 | 职责 |
| --- | --- |
| `web/default/src/routes/` | TanStack Router 文件路由 |
| `web/default/src/features/` | 按业务功能划分页面、API、类型、组件 |
| `web/default/src/lib/` | axios、错误处理、格式化、权限、主题工具 |
| `web/default/src/stores/` | Zustand store |
| `web/default/src/components/` | 通用 UI、布局、表格组件 |
| `web/default/src/i18n/` | i18next 配置和 locale JSON |

## 三、启动流程

入口是 `main.go` 的 `main()`，资源初始化集中在 `InitResources()`。

### 1. InitResources

`InitResources()` 按顺序完成：

1. 加载 `.env`。
2. `common.InitEnv()` 读取环境变量并初始化全局开关。
3. `logger.SetupLogger()` 配置日志。
4. `ratio_setting.InitRatioSettings()` 初始化模型倍率、价格、分组倍率等默认配置。
5. `service.InitHttpClient()` 初始化上游 HTTP client。
6. `service.InitTokenEncoders()` 初始化 token 计数器。
7. `model.InitDB()` 初始化主库，master 节点执行迁移。
8. `authz.Init(model.DB)` 初始化 Casbin/权限系统。
9. `model.CheckSetup()` 判断系统是否已完成初始化。
10. `model.InitOptionMap()` 加载默认 Option，并从数据库覆盖。
11. `common.CleanupOldCacheFiles()` 清理旧磁盘缓存。
12. `model.GetPricing()` 初始化模型价格数据。
13. `model.InitLogDB()` 初始化日志库，未指定 `LOG_SQL_DSN` 时复用主库。
14. `common.InitRedisClient()` 初始化 Redis。
15. `perfmetrics.Init()` 初始化性能指标。
16. `common.StartSystemMonitor()` 启动系统监控。
17. `i18n.Init()` 初始化后端翻译。
18. `oauth.LoadCustomProviders()` 加载自定义 OAuth provider。

### 2. 运行时后台任务

`main()` 初始化后会启动多个后台循环：

- `model.InitChannelCache()` 和 `model.SyncChannelCache()`：内存渠道缓存。
- `model.SyncOptions()`：定时从 `options` 表刷新配置。
- `authz.StartPolicySync()`：定时刷新授权策略。
- `model.UpdateQuotaData()`：数据看板统计。
- `controller.AutomaticallyUpdateChannels()`：可选的渠道更新。
- `service.StartCodexCredentialAutoRefreshTask()`：Codex 凭证自动刷新。
- `service.StartSubscriptionQuotaResetTask()`：订阅额度重置。
- `service.StartChannelAutoPriorityScanTask()`：渠道自动优先级扫描。
- `service.StartSystemInstanceReporter()`：上报当前实例。
- `controller.RegisterScheduledSystemTasks()` 和 `service.StartSystemTaskRunner()`：系统任务框架。
- `model.InitBatchUpdater()`：可选批量更新。
- pprof、Pyroscope、系统监控等可选诊断能力。

系统任务之外还有一些传统常驻任务，它们各自保证幂等或按 master 节点执行。读代码时可以先把它们理解为“维持系统状态的新鲜度”：配置热更新、渠道缓存、授权策略、订阅重置、渠道检测、异步任务轮询、实例心跳。

### 3. Gin server 装配

`main()` 创建 `gin.New()` 后挂载：

- panic recovery。
- `middleware.RequestId()`。
- `middleware.PoweredBy()`。
- `middleware.I18n()`。
- `middleware.SetUpLogger(server)`。
- `sessions.Sessions("session", cookie store)`。
- Umami/Google Analytics 注入。
- `router.SetRouter(server, ThemeAssets{...})`。

前端资源通过 Go embed 嵌入：

- `web/default/dist`
- `web/classic/dist`

`router/web-router.go` 会根据当前主题服务 default 或 classic 静态资源。

### 4. 优雅退出

收到 SIGINT/SIGTERM 后：

1. 使用 `SHUTDOWN_TIMEOUT_SECONDS`，默认 120 秒。
2. `srv.Shutdown(ctx)` 等待请求结束，特别是 SSE。
3. 如果启用数据导出，调用 `model.SaveQuotaDataCache()`。
4. 关闭数据库连接。

## 四、数据库、模型与迁移

数据库入口在 `model/main.go`。

### 1. 支持的数据库

主库支持：

- SQLite，默认使用 `common.SQLitePath`。
- MySQL，`SQL_DSN` 不以 postgres/local 开头时走 MySQL。
- PostgreSQL，`SQL_DSN` 以 `postgres://` 或 `postgresql://` 开头。

日志库支持：

- 默认复用主库。
- 可通过 `LOG_SQL_DSN` 单独指定。
- 日志库额外支持 ClickHouse，但主库不支持 ClickHouse。

`chooseDB(envName, isLog)` 负责根据 DSN 选择 GORM driver。

### 2. 跨库兼容细节

`initCol()` 会按数据库方言设置：

- `commonGroupCol`：`group` 是保留字，PostgreSQL 用 `"group"`，MySQL/SQLite 用反引号。
- `commonKeyCol`：同理处理 `key`。
- `commonTrueVal` / `commonFalseVal`：PostgreSQL 用 `true/false`，其他用 `1/0`。
- 日志库也有 `logGroupCol` / `logKeyCol`。

后续查询中遇到 `group`、`key`、布尔 SQL 字面量时应使用这些变量。

### 3. 自动迁移

`migrateDB()` 使用 `DB.AutoMigrate()` 迁移主表：

- `Channel`
- `Token`
- `User`
- `PasskeyCredential`
- `Option`
- `Redemption`
- `Ability`
- `Log`
- `Midjourney`
- `TopUp`
- `QuotaData`
- `Task`
- `Model`
- `Vendor`
- `PrefillGroup`
- `Setup`
- `TwoFA`
- `Checkin`
- `SubscriptionOrder`
- `UserSubscription`
- `SubscriptionPreConsumeRecord`
- `CustomOAuthProvider`
- `UserOAuthBinding`
- `PerfMetric`
- `ChannelUpstreamProfile`
- `UpstreamIdentity`
- `SystemInstance`
- `SystemTask`
- `SystemTaskLock`
- `CasbinRule`
- `AuthzRole`

SQLite 对部分表有专门兼容逻辑，例如订阅计划表通过 `ensureSubscriptionPlanTableSQLite()` 处理。

### 4. 关键模型

#### User

`model.User` 表示平台用户。重要字段：

- `Username`, `Password`, `DisplayName`, `Email`
- `Role`, `Status`
- `Quota`, `UsedQuota`, `RequestCount`
- `Group`
- OAuth 绑定字段，如 `GitHubId`, `DiscordId`, `OidcId`
- `AccessToken`，用于后台系统访问 token
- `Setting`，JSON 字符串，对应 `dto.UserSetting`
- `AdminPermissions`，运行时字段

用户设置通过：

- `User.GetSetting()`
- `User.SetSetting()`
- `UpdateUserSetting()`

#### Token

`model.Token` 表示用户 API key。重要字段：

- `Key`
- `Status`
- `RemainQuota`
- `UnlimitedQuota`
- `ModelLimitsEnabled`
- `ModelLimits`
- `AllowIps`
- `Group`
- `CrossGroupRetry`

`ValidateUserToken()` 做 relay API token 校验：

1. 通过 `GetTokenByKey()` 取 token，Redis 启用时优先走缓存。
2. 检查状态、过期时间、额度。
3. 非无限额度且 `RemainQuota <= 0` 时拒绝。

#### Channel

`model.Channel` 表示一个上游渠道。重要字段：

- `Type`：渠道类型，映射到 `constant.ChannelType*`。
- `Key`：上游 key，可多行或 JSON 数组。
- `BaseURL`
- `Models`
- `Group`
- `ModelMapping`
- `StatusCodeMapping`
- `Priority`
- `Weight`
- `AutoBan`
- `Setting`
- `ParamOverride`
- `HeaderOverride`
- `OtherSettings`
- `ChannelInfo`

`ChannelInfo` 管理多 key：

- `IsMultiKey`
- `MultiKeySize`
- `MultiKeyStatusList`
- `MultiKeyPollingIndex`
- `MultiKeyMode`

`GetNextEnabledKey()` 根据多 key 模式选择上游 key，支持随机和轮询。

#### Option

`model.Option` 是全局配置表：

```go
type Option struct {
    Key   string `gorm:"primaryKey"`
    Value string
}
```

`options` 表是运行时配置中心，启动时由 `InitOptionMap()` 载入默认值，再用数据库值覆盖。

#### Log

`model.Log` 是日志表，消费日志、充值日志、管理审计、错误日志都在这里。

重要字段：

- `Type`：`LogTypeConsume`, `LogTypeManage`, `LogTypeError` 等。
- `Quota`, `PromptTokens`, `CompletionTokens`
- `ChannelId`, `TokenId`, `Group`
- `RequestId`, `UpstreamRequestId`
- `Other`：JSON，存储计费明细、管理员信息、转换链、流状态等。

普通用户查询日志时会剥离 `admin_info`、`audit_info`、`stream_status` 等管理员字段。

## 五、配置 Option 与 setting 系统

配置系统分两层：

1. `setting/*` 包中保存强类型或 map 形式的运行时变量。
2. `options` 数据库表持久化配置，启动和热同步时写入内存。

### 1. Option 加载

`model.InitOptionMap()`：

1. 初始化 `common.OptionMap`。
2. 写入默认值，包括登录注册、支付、配额、模型倍率、分组倍率、敏感词、监控、前端展示等。
3. 从 `config.GlobalConfig.ExportAllConfigs()` 导出所有模型配置。
4. 调用 `loadOptionsFromDatabase()` 用数据库值覆盖。

`model.SyncOptions(frequency)` 会定时调用 `loadOptionsFromDatabase()`，所以大部分系统设置支持多实例热更新。

### 2. 更新 Option

单项更新：

- `model.UpdateOption(key, value)`
- 先保存 DB，再调用 `updateOptionMap(key, value)` 同步内存。

批量更新：

- `model.UpdateOptionsBulk(values)`
- 在事务中写入 DB，成功后统一更新内存。

### 3. 常见 setting 包

| 包 | 作用 |
| --- | --- |
| `setting/ratio_setting` | 模型倍率、模型价格、分组倍率、缓存倍率、音频/图片倍率 |
| `setting/billing_setting` | tiered billing mode 与表达式 |
| `setting/operation_setting` | 运营配置，如支付、渠道监控、自动禁用、重试状态码 |
| `setting/system_setting` | 系统级配置，如 OIDC、Discord、Passkey、主题 |
| `setting/model_setting` | 模型行为配置，如 Claude/Gemini/Grok/Qwen 等 |
| `setting/performance_setting` | 性能相关配置 |
| `setting/console_setting` | 控制台配置校验 |

## 六、HTTP 路由体系

路由总入口是 `router.SetRouter()`：

```go
SetApiRouter(router)
SetDashboardRouter(router)
SetRelayRouter(router)
SetVideoRouter(router)
if FRONTEND_BASE_URL empty:
    SetWebRouter(router, assets)
else:
    NoRoute redirect to FRONTEND_BASE_URL
```

### 1. `/api` 后台 API

定义在 `router/api-router.go`。

全局中间件：

- `RouteTag("api")`
- gzip
- `BodyStorageCleanup()`
- `GlobalAPIRateLimit()`

常见路由：

- `/api/setup`：系统初始化。
- `/api/status`：系统状态。
- `/api/user/*`：注册、登录、自身信息、充值、2FA、Passkey。
- `/api/token/*`：令牌管理。
- `/api/channel/*`：渠道管理。
- `/api/option/*`：Root 配置。
- `/api/subscription/*`：订阅。
- `/api/log/*`：日志。
- `/api/system_task/*`：系统任务。
- `/api/perf-metrics/*`：性能指标。
- `/api/pricing`, `/api/rankings`：公开或受导航权限控制的展示数据。

认证类型：

- `UserAuth()`：普通登录用户。
- `AdminAuth()`：管理员。
- `RootAuth()`：root。
- `HeaderNavModuleAuth()`：前台导航模块权限。
- 支付 webhook 和部分公开路由不要求用户登录。

### 2. `/v1`, `/v1beta`, `/mj`, `/suno` relay API

定义在 `router/relay-router.go`。

全局 relay 中间件：

- `CORS()`
- `DecompressRequestMiddleware()`
- `BodyStorageCleanup()`
- `StatsMiddleware()`

主要入口：

- `/v1/chat/completions` -> `controller.Relay(..., RelayFormatOpenAI)`
- `/v1/completions` -> OpenAI 格式
- `/v1/messages` -> Claude 格式
- `/v1/responses` -> OpenAI Responses
- `/v1/responses/compact` -> Responses compaction
- `/v1/images/generations`, `/v1/images/edits` -> Image
- `/v1/embeddings` -> Embedding
- `/v1/audio/*` -> Audio
- `/v1/rerank` -> Rerank
- `/v1/realtime` -> WebSocket realtime
- `/v1beta/models/*path` -> Gemini native
- `/mj/*` -> Midjourney
- `/suno/*` -> Suno task

relay 路由通常挂载：

```text
SystemPerformanceCheck
TokenAuth
ModelRequestRateLimit
Distribute
```

### 3. Web 静态资源路由

`router/web-router.go`：

- gzip
- `GlobalWebRateLimit()`
- `Cache()`
- `static.Serve("/", themeFS)`
- `NoRoute` 返回当前主题 `index.html`

如果请求路径以 `/v1`、`/api`、`/assets` 开头且未匹配，会返回 relay not found，而不是 SPA index。

## 七、认证、权限与上下文

### 1. Session 用户认证

`middleware/auth.go` 的 `authHelper(c, minRole)`：

1. 从 session 取 `username`, `role`, `id`, `status`。
2. 如果 session 没有用户，尝试用 `Authorization` access token 调 `model.ValidateAccessToken()`。
3. 要求请求头 `New-Api-User` 存在且等于当前用户 id。
4. 检查用户状态、角色、用户名合法性。
5. 将 `username`, `role`, `id`, `group`, `user_group` 写入 Gin context。
6. 管理员/root 写接口会自动走审计兜底。

对应中间件：

- `UserAuth()`
- `AdminAuth()`
- `RootAuth()`

### 2. API Token 认证

`TokenAuth()` 用于 relay API：

1. 支持 WebSocket `Sec-WebSocket-Protocol` 中携带 key。
2. Claude `/v1/messages` 可用 `x-api-key`。
3. Gemini `/v1beta/models` 可用 query `key` 或 `x-goog-api-key`。
4. 普通 OpenAI 兼容路径使用 `Authorization: Bearer sk-*`。
5. 支持 `sk-key-channelId` 形式让管理员指定渠道。
6. 调 `model.ValidateUserToken()` 验 token。
7. 校验 IP 白名单。
8. 读取 `model.GetUserCache()` 并写入用户上下文。
9. 处理 token 绑定分组、用户可用分组、弃用分组。
10. 调 `SetupContextForToken()` 写入 token id/key/name/quota/model limits 等。

### 3. 权限系统

权限系统位于 `service/authz/` 和 `model/authz_role.go`、`model/casbin_rule.go`。

- `authz.Init(model.DB)` 在启动时初始化。
- `middleware.RequirePermission(permission)` 用于更细粒度权限。
- `authz.StartPolicySync()` 定时刷新策略，适配多节点。

### 4. Gin context 是请求链路胶水

大量状态通过 `constant.ContextKey*` 写入 context：

- 用户和 token：`ContextKeyUserId`, `ContextKeyTokenId`, `ContextKeyTokenGroup`
- 分组：`ContextKeyUsingGroup`, `ContextKeyUserGroup`, `ContextKeyAutoGroup`
- 渠道：`ContextKeyChannelId`, `ContextKeyChannelType`, `ContextKeyChannelKey`, `ContextKeyChannelBaseUrl`
- 模型：`ContextKeyOriginalModel`, `ContextKeyChannelModelMapping`
- 请求：`ContextKeyRequestStartTime`, `ContextKeyIsStream`

后续 `RelayInfo` 会从 context 复制这些信息。

## 八、中继 relay 主流程

relay 的核心 handler 是 `controller.Relay(c, relayFormat)`。

### 1. 请求解析和 RelayInfo

`controller.Relay`：

1. 如果是 realtime，升级 WebSocket。
2. 调 `helper.GetAndValidateRequest(c, relayFormat)` 解析并校验请求 DTO。
3. 调 `relaycommon.GenRelayInfo(c, relayFormat, request, ws)` 构造 `RelayInfo`。
4. 记录 timing mark。

`RelayInfo` 是 relay 全链路上下文对象，定义在 `relay/common/relay_info.go`。它包含：

- 用户和 token 信息。
- 选中渠道信息 `ChannelMeta`。
- 请求格式、模式、模型名。
- stream/realtime/audio/reasoning 状态。
- 计费信息 `PriceData`、`Billing`、订阅字段。
- tiered billing snapshot。
- 请求转换链 `RequestConversionChain`。
- timing 标记。

### 2. 敏感词与 token 估算

`controller.Relay` 根据配置决定是否构造完整 `TokenCountMeta`：

- 需要敏感词检查或 token 计数时：`request.GetTokenCountMeta()`。
- 两者都关闭时：`fastTokenCountMetaForPricing()`，避免拼接巨大文本。

然后：

1. `service.CheckSensitiveText()` 做敏感词检查。
2. `service.EstimateRequestToken()` 估算输入 token。
3. `relayInfo.SetEstimatePromptTokens(tokens)` 写入估算值。

### 3. 价格计算与预扣费

`helper.ModelPriceHelper(c, relayInfo, tokens, meta)`：

- 读取模型价格或倍率。
- 应用分组倍率。
- 如果是 tiered expression，走 `modelPriceHelperTiered()`。
- 产出 `types.PriceData`，包括 `QuotaToPreConsume`。

如果不是免费模型，则：

```go
service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
```

预扣费成功后，`relayInfo.Billing` 保存计费会话。

### 4. 渠道重试循环

`controller.Relay` 构造 `service.RetryParam`，进入重试循环：

1. `getChannel(c, relayInfo, retryParam)` 获取当前或新的渠道。
2. 从 `common.GetBodyStorage(c)` 取可复用请求体，重置 `c.Request.Body`。
3. 根据请求格式分派：
   - OpenAI Realtime -> `relay.WssHelper`
   - Claude -> `relay.ClaudeHelper`
   - Gemini -> `geminiRelayHandler`
   - 其他 -> `relayHandler`
4. 成功则返回。
5. 失败则 `processChannelError()`，决定是否禁用渠道、记录错误日志、是否重试。
6. 如果失败且已预扣费，defer 中 `relayInfo.Billing.Refund(c)` 退款，并可能收取违规费用。

### 5. relayHandler 分派

`relayHandler(c, info)` 根据 `RelayMode` 调用：

- 图片：`relay.ImageHelper`
- 音频：`relay.AudioHelper`
- rerank：`relay.RerankHelper`
- embedding：`relay.EmbeddingHelper`
- Responses：`relay.ResponsesHelper`
- 默认文本：`relay.TextHelper`

Gemini native 根据路径是否包含 embed 分派到 `GeminiEmbeddingHandler` 或 `GeminiHelper`。

## 九、渠道选择与重试

渠道选择主要由 `middleware.Distribute()` 和 `service.CacheGetRandomSatisfiedChannel()` 完成。

### 1. Distribute 读取模型

`middleware/distributor.go` 的 `getModelRequest()` 会根据路径和 Content-Type 提取模型名：

- JSON body 通过 `gjson` 读取 `model` 和 `group`。
- multipart/form-data、form-urlencoded 通过可复用 body 解析。
- Midjourney/Suno/video 有专门逻辑。
- Gemini native 从 `/v1beta/models/{model}:generateContent` 路径提取。
- `/v1/audio/*` 缺省模型是 `tts-1` 或 `whisper-1`。
- `/v1/images/generations` 缺省模型是 `dall-e`。
- `/v1/responses/compact` 会给模型名加 compact 后缀。

### 2. token 模型限制

如果 token 启用了 `ModelLimitsEnabled`：

1. 用 `ratio_setting.FormatMatchingModelName()` 标准化模型名。
2. 检查模型是否在 token 允许列表。
3. 不允许则返回 403。

### 3. 分组和 auto group

使用分组来自：

- 用户分组。
- token 绑定分组。
- playground 请求中的 `group`。
- `auto` 分组时，可能根据用户可用自动分组选择具体分组。

`service.GetUserAutoGroup()` 和 `service.GroupInUserUsableGroups()` 参与判断。

### 4. 渠道亲和性

`Distribute()` 会先检查：

- `service.GetPreferredChannelByAffinity()`
- `service.IsChannelSatisfiableForModel()`
- `service.MarkChannelAffinityUsed()`
- `service.RecordChannelAffinityFromContext()`

命中亲和渠道且可用时优先使用。失败策略由 operation setting 控制，例如是否在 rate limit 或 unavailable 时打破亲和性。

### 5. 普通渠道选择

亲和性未命中时调用：

```go
service.CacheGetRandomSatisfiedChannel(&service.RetryParam{...})
```

选择逻辑综合：

- 渠道状态。
- 分组匹配。
- 模型是否支持。
- priority/weight。
- auto group。
- 请求路径支持，尤其 Advanced Custom 渠道。
- 已排除渠道。
- 跨分组重试配置。

选中后调用 `SetupContextForSelectedChannel()` 写 context。

底层候选主要来自 `model/channel_cache.go` 的内存结构：

- `group2model2channels`：按分组、模型组织可用渠道。
- `channelsIDM`：按渠道 id 快速取渠道。
- ability 表：渠道支持模型能力缓存。

`model.GetRandomSatisfiedChannelWithExclusions()` 会从这些缓存中取候选，先按 priority 分层，再在同层按 weight 随机。Advanced Custom 渠道额外检查请求路径是否被该渠道配置支持。

### 6. 上游 RPM 和失败排除

在 relay retry loop 中：

- `service.CheckAndReserveChannelRPM(channel)` 做渠道 RPM 限制。
- 单请求同一渠道失败达到 `perRequestChannelFailureLimit` 会排除。
- 模型不存在错误会调用 `service.MarkModelUnavailableForChannel()`，该请求立即排除此渠道。
- 上游余额不足会触发通知和可选自动禁用。
- `shouldRetry()` 结合状态码、错误类型、特定渠道、operation setting 决定是否重试。

## 十、provider adaptor 体系

adaptor 接口定义在 `relay/channel/adapter.go`：

```go
type Adaptor interface {
    Init(info *relaycommon.RelayInfo)
    GetRequestURL(info *relaycommon.RelayInfo) (string, error)
    SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
    ConvertOpenAIRequest(...)
    ConvertClaudeRequest(...)
    ConvertGeminiRequest(...)
    ConvertEmbeddingRequest(...)
    ConvertAudioRequest(...)
    ConvertImageRequest(...)
    ConvertOpenAIResponsesRequest(...)
    DoRequest(...)
    DoResponse(...)
    GetModelList() []string
    GetChannelName() string
}
```

adaptor 获取入口是 `relay.GetAdaptor(apiType)`，定义在 `relay/relay_adaptor.go`。

渠道类型先通过 `common.ChannelType2APIType()` 转 API type，再选择 adaptor。典型映射：

- OpenAI/Azure/OpenRouter/Xinference -> OpenAI adaptor 或兼容分支。
- Anthropic -> Claude adaptor。
- Gemini -> Gemini adaptor。
- AWS -> Bedrock adaptor。
- Ali、Baidu、Tencent、Xunfei、Zhipu、VolcEngine、MiniMax 等各自独立 adaptor。
- Advanced Custom -> 高级自定义 adaptor。

### 1. TextHelper 的 adaptor 生命周期

`relay/compatible_handler.go` 的 `TextHelper()` 是文本 relay 核心：

1. `info.InitChannelMeta(c)` 从 context 初始化渠道元信息。
2. DeepCopy 原始 OpenAI 请求，避免修改原对象。
3. `helper.ModelMappedHelper()` 应用模型映射。
4. 处理 `StreamOptions` 支持能力。
5. `GetAdaptor(info.ApiType)`。
6. `adaptor.Init(info)`。
7. 如果启用 chat-completions-via-responses，走 `chatCompletionsViaResponses()`。
8. 如果全局或渠道开启 pass-through，直接复用原请求体。
9. 否则调用 `adaptor.ConvertOpenAIRequest()`。
10. 应用渠道 system prompt、disabled fields、param override。
11. `common.Marshal()` 序列化请求体。
12. `adaptor.DoRequest()` 发上游。
13. 非 200 走 `service.RelayErrorHandler()`。
14. `adaptor.DoResponse()` 处理上游响应并写回客户端。
15. 根据 usage 走文本或音频结算。

实际 HTTP 请求构造在 `relay/channel/api_request.go` 的 `DoApiRequest()`：

- 调 adaptor 的 `GetRequestURL()`。
- 创建上游 `http.Request`。
- 调 adaptor 的 `SetupRequestHeader()`。
- 应用渠道 header override。
- 设置 `ContentLength`，特别是 `relayInfo.UpstreamRequestBodySize`。
- 根据渠道代理或全局 client 发送请求。
- 对流式请求设置 SSE 相关响应头，并由 stream handler 继续处理。

### 2. OpenAI adaptor 示例

`relay/channel/openai/adaptor.go`：

- `GetRequestURL()` 处理 Azure deployment URL、Responses API、Realtime、Custom URL。
- `SetupRequestHeader()` 设置 Authorization、Azure `api-key`、OpenRouter referer/title、Realtime subprotocol。
- `ConvertOpenAIRequest()` 处理 OpenRouter reasoning、O 系列/GPT-5 max token 字段、thinking suffix 等。
- `DoResponse()` 对 stream 调 `OaiStreamHandler()`，非 stream 调 `OpenaiHandler()`。

`relay/channel/openai/relay-openai.go`：

- `OaiStreamHandler()` 扫描 SSE，转发 chunk，累计文本，解析 usage，补 fallback usage。
- `OpenaiHandler()` 读取完整响应，解析 OpenAI response，处理错误，必要时转换成 Claude/Gemini 响应，写回客户端。

### 3. Claude adaptor 示例

`relay/channel/claude/adaptor.go`：

- `GetRequestURL()` 指向 `/v1/messages`，必要时追加 `?beta=true`。
- `SetupRequestHeader()` 设置 `x-api-key`、`anthropic-version`、`anthropic-beta`。
- `ConvertOpenAIRequest()` 将 OpenAI 请求转 Claude Messages。
- `ConvertClaudeRequest()` 原样透传 Claude 请求。
- `DoResponse()` 根据 stream 走 `ClaudeStreamHandler()` 或 `ClaudeHandler()`。

### 4. TaskAdaptor

异步任务使用 `TaskAdaptor`：

- 负责验证任务请求、估算计费、构造 URL/header/body、提交上游、解析 task id。
- 轮询阶段还负责 `FetchTask()`、`ParseTaskResult()`、`AdjustBillingOnComplete()`。

任务 adaptor 注册在 `relay.GetTaskAdaptor(platform)`。

## 十一、模型映射、请求转换与响应转换

new-api 的一个关键能力是：下游用一种协议，上游可以是另一种协议。

### 1. RelayFormat

`types/relay_format.go` 定义格式，例如：

- `openai`
- `claude`
- `gemini`
- `embedding`
- `openai_image`
- `openai_audio`
- `openai_responses`
- `openai_realtime`

`controller.Relay` 传入初始格式，`RelayInfo.RequestConversionChain` 记录转换链。

### 2. 模型映射

`helper.ModelMappedHelper(c, info, request)` 根据渠道的 `ModelMapping` 把客户端模型名映射到上游模型名。

`RelayInfo` 中：

- `OriginModelName`：客户端请求模型名。
- `UpstreamModelName`：上游实际模型名。
- `PricingModelName`：计费模型名，降级或特殊场景可能不同。
- `IsModelMapped`：是否发生模型映射。

日志 `other` 中会记录映射信息。

### 3. OpenAI/Claude/Gemini 转换

转换函数分布在：

- `service/convert.go`
- `service/relayconvert/`
- 各 adaptor 的 `Convert*Request()`
- 各 provider 的 response handler

典型方向：

- Claude request -> OpenAI upstream：`openai.Adaptor.ConvertClaudeRequest()` 调 `service.ClaudeToOpenAIRequest()`。
- OpenAI request -> Claude upstream：`claude.Adaptor.ConvertOpenAIRequest()` 调 `RequestOpenAI2ClaudeMessage()`。
- Gemini request -> OpenAI upstream：`openai.Adaptor.ConvertGeminiRequest()` 调 `service.GeminiToOpenAIRequest()`。
- OpenAI response -> Claude/Gemini downstream：`service.ResponseOpenAI2Claude()`、`service.ResponseOpenAI2Gemini()`。

### 4. Responses API 兼容

Responses 相关文件：

- `relay/responses_handler.go`
- `relay/channel/openai/relay_responses.go`
- `relay/channel/gemini/relay_responses.go`
- `service/openai_chat_responses_compat.go`
- `service/relayconvert/`

同时支持：

- `/v1/responses`
- `/v1/responses/compact`
- chat completions 经 Responses 上游转发
- Responses 与 Chat Completions 互转

## 十二、计费体系

计费是 new-api 的核心复杂度之一，由预扣费、后结算、钱包/订阅资金源、token 额度、日志展示共同组成。

### 1. quota 基本概念

内部以 quota 为单位：

- `common.QuotaPerUnit` 表示 1 美元或配置单位对应多少 quota。
- 模型可以按 token 倍率计费，也可以按价格计费。
- 分组倍率会乘到最终 quota 上。

### 2. 价格计算入口

`relay/helper/price.go` 的 `ModelPriceHelper()`：

1. 确定 `pricingModel`。
2. 读取模型价格 `ratio_setting.GetModelPrice()`。
3. 计算 `groupRatioInfo := HandleGroupRatio(c, info)`。
4. 如果模型配置为 `tiered_expr`，走表达式计费。
5. 否则：
   - 有模型价格时：`modelPrice * QuotaPerUnit * groupRatio`。
   - 无模型价格时：按 prompt token、max tokens、模型倍率、补全倍率等估算预扣费。
6. 如果关闭免费模型预扣，且价格/倍率/分组倍率为 0，则标记免费模型。

`types.PriceData` 会写入 `relayInfo.PriceData`。

### 3. BillingSession

`service/billing_session.go` 定义统一计费会话。

核心状态：

- `funding`：资金源，钱包或订阅。
- `preConsumedQuota`：实际预扣额度。
- `tokenConsumed`：token 额度扣减。
- `trusted`：是否命中信任额度旁路。
- `fundingSettled`, `settled`, `refunded`：生命周期状态。

入口：

- `service.PreConsumeBilling(c, quota, relayInfo)` 创建 session 并预扣。
- `service.SettleBilling(ctx, relayInfo, actualQuota)` 后结算。
- 失败时 `relayInfo.Billing.Refund(c)` 退款。

### 4. 钱包与订阅

`NewBillingSession()` 根据 `relayInfo.UserSetting.BillingPreference` 选择：

- `subscription_only`
- `wallet_only`
- `wallet_first`
- `subscription_first`，默认

订阅优先时：

1. 检查用户是否有活跃订阅。
2. 有则优先订阅预扣。
3. 订阅不足且允许钱包溢出时回退钱包。
4. 没有活跃订阅时走钱包。

钱包路径会先检查 `model.GetUserQuota()` 是否足够。

### 5. 信任额度旁路

当用户和 token 额度均大于 `common.GetTrustQuota()` 时，钱包计费可以不预扣，后结算再扣。

订阅不启用信任旁路，因为订阅预扣记录必须创建，用于锁定订阅和记录 request id。

异步任务 `ForcePreConsume=true` 时也不允许信任旁路。

### 6. 后结算

不同请求类型会在响应完成后调用：

- `service.PostTextConsumeQuota()`
- `service.PostAudioConsumeQuota()`
- `service.PostWssConsumeQuota()`
- task 相关结算函数

结算会：

1. 根据真实 usage 算实际 quota。
2. 更新用户 used quota、渠道 used quota。
3. `SettleBilling()` 对比 actual 和 pre-consumed，补扣或返还差额。
4. 写消费日志。

### 7. tiered expression 计费

设计文档在 `pkg/billingexpr/expr.md`。核心原则是表达式就是计费合同。

配置存储在：

- `setting/billing_setting/tiered_billing.go`
- `ModelBillingMode`
- `ModelBillingExpr`

预扣阶段：

1. `ModelPriceHelper()` 判断 billing mode 为 `tiered_expr`。
2. `modelPriceHelperTiered()` 读取表达式。
3. 构造 `billingexpr.RequestInput`，让表达式可使用 header/body 参数。
4. 用估算 prompt token 和 max tokens 执行表达式。
5. 表达式结果按 `$ / 1M tokens` 转 quota。
6. 创建 `BillingSnapshot` 冻结表达式、group ratio、估算 token、tier 等信息。

后结算阶段：

1. `BuildTieredTokenParams()` 从真实 `dto.Usage` 构造 token 参数。
2. 根据表达式实际使用的变量，对 GPT/OpenAI 语义 usage 扣除 cache/image/audio 子类，避免重复计费。
3. Claude 语义 usage 不做同样扣除，因为 Claude 的 input_tokens 本身不含 cache。
4. `TryTieredSettle()` 使用冻结 snapshot 和真实 token 重新计算。
5. 失败时回退到预扣额度。

常用变量：

- `p`：输入 token，自动排除单独定价的子类别。
- `c`：输出 token，自动排除单独定价的子类别。
- `len`：完整上下文长度，用于阶梯判断。
- `cr`：cache read。
- `cc`, `cc1h`：cache creation。
- `img`, `img_o`：图片输入/输出。
- `ai`, `ao`：音频输入/输出。

### 8. 计费日志展示信息

`service/log_info_generate.go` 会把以下信息写入 log `Other`：

- `model_ratio`
- `group_ratio`
- `completion_ratio`
- `cache_tokens`
- `cache_ratio`
- `model_price`
- `user_group_ratio`
- `billing_source`
- 订阅明细
- 请求转换链
- 模型映射
- admin_info
- stream_status
- tiered billing 的表达式和 matched tier

前端日志页根据 `Other` 渲染更详细的计费说明。

## 十三、日志、审计与用量统计

### 1. 消费日志

消费完成后由 `model.RecordConsumeLog()` 写入 `logs` 表。

日志内容包含：

- 用户、token、模型、渠道、分组。
- prompt/completion tokens。
- quota。
- 用时和是否 stream。
- `Other` JSON。

日志库可以是主库，也可以独立配置为 MySQL/PostgreSQL/SQLite/ClickHouse。

### 2. 错误日志

relay 上游错误时，`processChannelError()` 在满足条件时记录错误日志：

- 用户 id。
- token name/id。
- 模型。
- 分组。
- 渠道。
- request path。
- 错误内容。

同时可能触发：

- 自动禁用渠道。
- 上游余额不足通知。
- 模型对渠道的 negative cache。

### 3. 管理审计

`middleware/auth.go` 在 `AdminAuth()` 和 `RootAuth()` 链路中自动开启审计兜底。

handler 如果手动记录了审计，会设置 context 标记跳过兜底。否则中间件会根据方法、路由、状态记录管理操作日志。

`model.RecordOperationAuditLog()` 将：

- 面向用户的稳定 action 和 params 写入 `Other.op`。
- 管理员详情写入 `Other.admin_info`。
- 路由、方法、结果等写入 `Other.audit_info`。

普通用户查询会剥离管理员字段。

### 4. 用量看板和排行

相关模块：

- `model/usedata.go`
- `model/usedata_flow.go`
- `model/usedata_rankings.go`
- `controller/usedata.go`
- `controller/rankings.go`

后台 `model.UpdateQuotaData()` 定期更新 quota 数据缓存。退出时如果开启数据导出，会保存内存缓存。

### 5. 性能指标

`pkg/perf_metrics/` 记录 relay 性能样本。

在 `controller.Relay` 失败时会 `perfmetrics.RecordRelaySample(relayInfo, false, 0)`，成功路径也会在响应处理处记录。前端通过 `/api/perf-metrics` 展示。

## 十四、异步任务、视频、音乐和系统任务

new-api 不只代理同步文本请求，也支持任务式上游，如 Midjourney、Suno、视频生成。

### 1. Task 模型

`model.Task` 存储异步任务：

- 平台。
- 上游 task id。
- 状态。
- 进度。
- 失败原因。
- data。
- origin model。
- quota。
- channel。

### 2. 提交流程

任务提交通常走：

```text
TokenAuth
  -> Distribute
  -> controller.RelayTask / RelayMidjourney / video handler
  -> TaskAdaptor.ValidateRequestAndSetAction
  -> 估算并强制预扣费
  -> TaskAdaptor.BuildRequest*
  -> TaskAdaptor.DoResponse 解析上游 task id
  -> model.Task 保存
```

异步任务必须 `ForcePreConsume=true`，因为请求返回时任务还未完成，必须先锁定额度。

### 3. 轮询流程

`service/task_polling.go` 的 `RunTaskPollingOnce()`：

1. `sweepTimedOutTasks()` 处理超时任务。
2. `model.GetAllUnFinishSyncTasks()` 查询未完成任务。
3. 按平台分组。
4. 每个平台调用对应 adaptor：
   - Suno -> `UpdateSunoTasks()`
   - 视频类 -> `UpdateVideoTasks()`
   - Midjourney 由自身轮询处理。
5. 任务成功、失败、超时时更新状态。
6. 失败或超时时按规则退款。
7. 终态时可通过 `AdjustBillingOnComplete()` 做最终计费调整。

### 4. 系统任务框架

系统任务在 `service/system_task.go` 和 `model/system_task.go`。

核心目标是：多 master 部署时通过 DB lease 去重，避免多个实例重复执行定时任务。

关键接口：

```go
type SystemTaskHandler interface {
    Type() string
    Run(ctx context.Context, task *model.SystemTask, runnerID string)
}

type ScheduledSystemTaskHandler interface {
    SystemTaskHandler
    Enabled() bool
    Interval() time.Duration
    NewPayload() any
}
```

系统任务 runner：

1. master 节点启动。
2. 周期性清理 stale lock。
3. 调 scheduler 创建 due task。
4. 按 task type claim pending task。
5. 每个 claimed task 单独 goroutine 执行。
6. 执行期间 heartbeat 延长 lease。
7. handler 必须调用 `model.FinishSystemTask()` 进入终态。

已注册 scheduled handler：

- `channel_test`
- `model_update`
- `midjourney_poll`
- `async_task_poll`

非 scheduled handler 示例：

- `log_cleanup`

## 十五、用户、令牌、订阅与支付

### 1. 注册登录

路由在 `/api/user`：

- `/register`
- `/login`
- `/login/2fa`
- `/passkey/login/begin`
- `/passkey/login/finish`
- OAuth 路由在 `/api/oauth/*`

控制器分布：

- `controller/user.go`
- `controller/twofa.go`
- `controller/passkey.go`
- `controller/oauth.go`
- `controller/custom_oauth.go`

登录成功后写 session，前端还会保存用户信息到 localStorage。

### 2. 令牌管理

`/api/token`：

- 列表、搜索、详情。
- 新建、更新、删除。
- 获取单个或批量 key。

令牌用于 relay API，关键约束：

- 额度。
- 过期时间。
- IP 限制。
- 模型限制。
- 分组。
- 是否允许 auto 分组跨分组重试。

### 3. 订阅

订阅路由在 `/api/subscription` 和 `/api/subscription/admin`。

核心模型：

- `SubscriptionPlan`
- `SubscriptionOrder`
- `UserSubscription`
- `SubscriptionPreConsumeRecord`

定时任务：

- `service.StartSubscriptionQuotaResetTask()` 按日/周/月/自定义规则重置额度。

计费时：

- `SubscriptionFunding.PreConsume()` 预扣。
- `SubscriptionFunding.Settle()` 后结算。
- 订阅日志信息写入 log `Other`。

### 4. 支付与充值

支持多种支付方式：

- Epay
- Stripe
- Creem
- Waffo
- Waffo Pancake

相关文件：

- `controller/topup*.go`
- `controller/subscription_payment*.go`
- `service/epay.go`
- `service/webhook.go`
- `service/waffo_pancake.go`
- `setting/payment_*.go`
- `setting/operation_setting/payment_setting*.go`

支付 webhook 多数是不登录路由，但会做签名/环境校验。

## 十六、缓存、Redis 与性能能力

### 1. Redis

`common.InitRedisClient()` 初始化 Redis。启用 Redis 后：

- `common.RedisEnabled = true`
- 为兼容旧版本，会启用 `common.MemoryCacheEnabled`

Redis 常用于：

- token cache。
- user cache。
- rate limit。
- 分布式状态。

### 2. 内存渠道缓存

`model.InitChannelCache()` 启动时加载渠道能力、模型、分组等。

`model.SyncChannelCache(frequency)` 定时同步。

`model.CacheGetChannel()`、`model.CacheGetChannelInfo()` 等用于请求链路快速读取。

### 3. 请求体复用

很多中间件和 controller 都需要读 body。项目用 `common.BodyStorage` 支持可复用 body：

- distributor 读取 model。
- controller 解析请求 DTO。
- retry 时重新设置 `c.Request.Body`。
- pass-through 时直接转发原始 body。

相关文件：

- `common/body_storage.go`
- `middleware/body_cleanup.go`

### 4. 限流

中间件包括：

- `GlobalAPIRateLimit()`
- `GlobalWebRateLimit()`
- `CriticalRateLimit()`
- `SearchRateLimit()`
- `EmailVerificationRateLimit()`
- `ModelRequestRateLimit()`

渠道自身还有 RPM 限制：

- `service.CheckAndReserveChannelRPM(channel)`

### 5. 诊断

支持：

- pprof：`ENABLE_PPROF=true`。
- Pyroscope：`common.StartPyroScope()`。
- 系统监控：`common.StartSystemMonitor()`。
- 性能配置：`setting/performance_setting`。

## 十七、前端 web/default

默认前端是 `web/default`，技术栈：

- React 19。
- TypeScript。
- Rsbuild。
- TanStack Router。
- TanStack Query。
- Axios。
- Zustand。
- Base UI。
- Tailwind CSS。
- i18next。

### 1. 入口

`web/default/src/main.tsx`：

1. 初始化前端缓存和构建元数据。
2. 创建 `QueryClient`。
3. 配置全局 query retry、mutation error、queryCache error。
4. 创建 TanStack Router。
5. 从 `/api/status` 和 localStorage 设置 document title/favicon。
6. 渲染 Provider 链：

```text
QueryClientProvider
  -> ThemeProvider
  -> FontProvider
  -> DirectionProvider
  -> RouterProvider
```

### 2. 路由

路由文件在 `web/default/src/routes/`，由 TanStack Router 文件路由生成 `routeTree.gen`。

关键路由：

- `__root.tsx`：根路由，装全局布局、Toaster、Devtools、setup 检查。
- `_authenticated/route.tsx`：认证布局，检查 localStorage 用户和 session。
- `(auth)/*`：登录、注册、找回密码、OTP。
- `_authenticated/channels`：渠道管理。
- `_authenticated/keys`：令牌。
- `_authenticated/usage-logs`：日志。
- `_authenticated/system-settings`：系统设置。
- `_authenticated/models`：模型与价格。
- `_authenticated/playground`：调试。
- `pricing`, `rankings`, `about`, `privacy-policy`, `user-agreement`：公开页面。
- `setup`：初始化向导。

### 3. API client

`web/default/src/lib/api.ts`：

- 创建 axios 实例 `api`。
- `baseURL` 为空，默认同源请求。
- `withCredentials: true` 携带 cookie。
- GET 请求做 in-flight 去重。
- 请求拦截器自动加 `New-Api-User`。
- 响应拦截器处理 `{ success: false, message }`。
- 401 时清理 auth store。

各 feature 也有自己的 `api.ts`，用统一 `api` 实例调用后端。

### 4. 认证状态

`web/default/src/stores/auth-store.ts`：

- Zustand 保存 `auth.user`。
- 用户信息持久化到 localStorage `user`。
- 用户 id 持久化到 localStorage，供 `New-Api-User` header 使用。

`_authenticated/route.tsx`：

1. 本地没有 user 直接跳登录。
2. 每个浏览器会话首次进入认证路由时调用 `/api/user/self` 验 session。
3. 成功则更新本地 user。
4. 401 则清空并跳登录。

### 5. i18n

`web/default/src/i18n/config.ts`：

- 支持 `en`, `zh`, `fr`, `ru`, `ja`, `vi`。
- `fallbackLng: 'en'`。
- `load: 'languageOnly'`。
- `nsSeparator: false`，允许英文 key 中有冒号。
- 通过 localStorage 和 navigator 检测语言。

locale 文件在：

```text
web/default/src/i18n/locales/{lang}.json
```

### 6. 功能模块

`web/default/src/features/` 按领域组织：

- `auth`：登录注册、OAuth、2FA、Passkey 相关前端。
- `channels`：渠道列表、编辑、测试、复制、排序、能力设置。
- `models`：模型、供应商、定价、动态计费。
- `keys`：API key 管理。
- `usage-logs`：消费、错误、充值、管理日志。
- `system-settings`：选项配置。
- `dashboard`：仪表盘。
- `playground`：在线调试。
- `subscriptions`：订阅计划和购买。
- `wallet`：充值。
- `users`：用户管理。
- `pricing`：公开模型价格。
- `rankings`：排行。
- `performance-metrics`：性能指标。

### 7. 构建和开发

`web/default/package.json`：

- `bun run dev`
- `bun run build`
- `bun run build:check`
- `bun run typecheck`
- `bun run lint`
- `bun run format`
- `bun run i18n:sync`

`web/default/rsbuild.config.ts`：

- React plugin。
- Tailwind plugin。
- TanStack Router plugin。
- 开发代理 `/api`, `/mj`, `/pg` 到 `VITE_REACT_APP_SERVER_URL` 或 `http://localhost:3000`。
- 生产启用 split chunks。

## 十八、常见开发改动入口

### 1. 新增普通后台 API

一般路径：

1. 在 `router/api-router.go` 选择合适 group 和认证中间件。
2. 在 `controller/` 添加 handler。
3. 复杂业务放 `service/`。
4. DB 操作放 `model/`。
5. 前端 feature 的 `api.ts` 调用新接口。
6. 新增 UI 文案时更新 i18n。

### 2. 新增 provider/channel

一般路径：

1. 在 `constant/channel.go` / `constant/api_type.go` 增加类型。
2. 在 `common/api_type.go` 补 `ChannelType2APIType()` 映射。
3. 在 `relay/channel/<provider>/` 实现 `Adaptor`。
4. 在 `relay/relay_adaptor.go` 的 `GetAdaptor()` 注册。
5. 实现模型列表、URL、header、请求转换、响应处理。
6. 如果支持 `StreamOptions`，添加到 `relay/common/relay_info.go` 的 `streamSupportedChannels`。
7. 补后台渠道配置和前端展示。
8. 补真实协议语义相关测试，尤其 optional scalar 和 zero value。

### 3. 新增模型定价规则

倍率/价格方式：

1. 修改 `setting/ratio_setting` 默认 map 或通过后台配置 Option。
2. 确认 `ModelPriceHelper()` 能找到模型价格或倍率。
3. 检查日志展示是否需要新增字段。

tiered expression：

1. 先读 `pkg/billingexpr/expr.md`。
2. 配置 `ModelBillingMode` 为 `tiered_expr`。
3. 配置 `ModelBillingExpr`。
4. 确认表达式变量符合 usage 语义。
5. 补 `pkg/billingexpr`、`service/tiered_settle.go` 或前端编辑器相关测试。

### 4. 新增异步任务平台

一般路径：

1. 定义平台常量。
2. 实现 `relay/channel/task/<platform>/TaskAdaptor`。
3. 在 `relay.GetTaskAdaptor()` 注册。
4. 提交 handler 创建 `model.Task`。
5. 在 `service/task_polling.go` 的分发中加入轮询逻辑。
6. 明确失败、超时、成功时的退款和差额结算。

### 5. 修改前端页面

一般路径：

1. 找 `web/default/src/routes` 对应路由。
2. 找 `web/default/src/features/<feature>` 对应业务模块。
3. API 调用放 feature `api.ts` 或通用 `lib/api.ts`。
4. 状态优先 React Query，跨页面持久状态用 Zustand。
5. 文案使用 `useTranslation()` 的 `t()`。
6. 运行 `bun run typecheck` 和 lint。

## 十九、关键流程速查

### 1. OpenAI chat completions 同步非流式

```text
POST /v1/chat/completions
  router/relay-router.go
  -> TokenAuth
  -> ModelRequestRateLimit
  -> Distribute
     -> getModelRequest
     -> CacheGetRandomSatisfiedChannel
     -> SetupContextForSelectedChannel
  -> controller.Relay(RelayFormatOpenAI)
     -> GetAndValidateRequest
     -> GenRelayInfoOpenAI
     -> EstimateRequestToken
     -> ModelPriceHelper
     -> PreConsumeBilling
     -> retry loop
  -> relay.TextHelper
     -> InitChannelMeta
     -> ModelMappedHelper
     -> adaptor.ConvertOpenAIRequest
     -> ApplyParamOverride
     -> adaptor.DoRequest
     -> adaptor.DoResponse
  -> service.PostTextConsumeQuota
  -> service.SettleBilling
  -> model.RecordConsumeLog
```

### 2. OpenAI chat completions 流式

与非流式相同，但：

- `request.IsStream(c)` 设置 `RelayInfo.IsStream`。
- 支持的渠道会保留或强制设置 `StreamOptions.IncludeUsage`。
- `OpenAI adaptor` 调 `OaiStreamHandler()`。
- SSE chunk 一边转发给下游，一边累计 usage。
- 没有 usage 时使用响应文本 fallback 估算。
- 最后写 `[DONE]` 和 usage 相关信息。

### 3. Claude Messages 请求

```text
POST /v1/messages
  -> TokenAuth 支持 x-api-key
  -> Distribute 提取 model
  -> controller.Relay(RelayFormatClaude)
  -> relay.ClaudeHelper
```

如果上游是 Claude adaptor：

- Claude request 原样上游。
- Claude response 直接转下游。

如果上游是 OpenAI 兼容 adaptor：

- `openai.Adaptor.ConvertClaudeRequest()` 把 Claude request 转 OpenAI chat。
- OpenAI response 再转 Claude response。

### 4. Gemini native 请求

```text
POST /v1beta/models/{model}:generateContent
  -> TokenAuth 支持 key query 和 x-goog-api-key
  -> Distribute 从路径提取 model
  -> controller.Relay(RelayFormatGemini)
  -> relay.GeminiHelper 或 GeminiEmbeddingHandler
```

如果上游不是 Gemini，可以通过 adaptor 转成对应上游格式。

### 5. 图片、音频、embedding、rerank

共同模式：

```text
路由设置 RelayFormat
  -> controller.Relay
  -> relayHandler 根据 RelayMode 分派
  -> 对应 Helper 初始化 adaptor
  -> Convert*Request
  -> DoRequest
  -> DoResponse
  -> Post*ConsumeQuota
```

图片和音频会额外使用 `ImageRatio`、`AudioRatio`、`AudioCompletionRatio` 或模型价格。

### 6. 失败、退款和自动禁用

```text
adaptor.DoRequest / DoResponse 返回 NewAPIError
  -> service.ResetStatusCode 可映射状态码
  -> processChannelError
     -> model not found: 标记渠道模型不可用并排除
     -> insufficient balance: 通知并可自动禁用
     -> ShouldDisableChannel: 自动禁用
     -> 记录 error log
  -> shouldRetry 决定是否换渠道
  -> 所有重试失败
     -> Billing.Refund
     -> ChargeViolationFeeIfNeeded
     -> 返回 OpenAI/Claude 格式错误
```

### 7. 后台系统设置更新

```text
前端 system-settings
  -> /api/option/*
  -> RootAuth
  -> controller/option.go
  -> model.UpdateOption / UpdateOptionsBulk
  -> DB options 表
  -> updateOptionMap
  -> setting/common 全局变量更新
  -> 其他节点通过 SyncOptions 定时同步
```

### 8. 用户登录和前端会话

```text
前端登录表单
  -> /api/user/login
  -> controller.Login
  -> 写 session
  -> 返回 user
  -> auth-store 保存 user 和 uid 到 localStorage
  -> axios 请求自动加 New-Api-User
  -> _authenticated 路由首次进入调用 /api/user/self 验证 session
```

### 9. 系统任务调度

```text
main.go
  -> RegisterScheduledSystemTasks
  -> StartSystemTaskRunner
  -> scheduler 判断 Enabled + Interval
  -> CreateSystemTask
  -> runner claim pending task
  -> heartbeat lease
  -> handler Run
  -> FinishSystemTask
```

### 10. 前端请求后台 API

```text
feature component
  -> React Query useQuery/useMutation
  -> feature/api.ts
  -> lib/api.ts axios instance
  -> request interceptor 加 New-Api-User
  -> backend /api/*
  -> response interceptor 处理 success=false 和 401
```

## 二十、精读索引与兼容性注意

这一节把并行阅读中最值得二次精读的细节集中列出，适合在掌握主流程后按需查阅。

### 1. 后端精读索引

| 主题 | 文件/函数 | 为什么重要 |
| --- | --- | --- |
| 启动资源 | `main.go` 的 `InitResources()` | 理解 DB、Option、Redis、i18n、OAuth、tokenizer 初始化顺序 |
| 路由入口 | `router/api-router.go`, `router/relay-router.go` | 看清后台 API 和 relay API 如何分流 |
| session 鉴权 | `middleware/auth.go` 的 `authHelper()` | 后台接口依赖 session/access token 和 `New-Api-User` |
| API token 鉴权 | `middleware/auth.go` 的 `TokenAuth()` | relay API key、Anthropic/Gemini 兼容 key 都在这里归一化 |
| 渠道分发 | `middleware/distributor.go` 的 `Distribute()` | 读取模型、模型白名单、亲和性、选渠道、写 context |
| 渠道缓存 | `model/channel_cache.go` | `group2model2channels`、ability、priority/weight 随机的底层来源 |
| relay 编排 | `controller/relay.go` 的 `Relay()` | 请求解析、预扣费、重试、退款、错误返回的总控 |
| adaptor 抽象 | `relay/channel/adapter.go` | 所有 provider 都要满足这个接口 |
| 上游请求 | `relay/channel/api_request.go` 的 `DoApiRequest()` | URL/header/proxy/body/SSE header 的统一处理 |
| OpenAI 兼容 | `relay/compatible_handler.go`, `relay/channel/openai/` | 最常见请求路径和兼容转换集中在这里 |
| 错误处理 | `service/error.go`, `controller/relay.go` 的 `processChannelError()` | 非 200、状态码映射、自动禁用、错误日志、重试 |
| 计费预扣 | `relay/helper/price.go`, `service/billing_session.go` | 价格估算、钱包/订阅资金源、信任额度旁路 |
| 计费结算 | `service/text_quota.go`, `service/quota.go`, `service/billing.go` | usage 到 quota、补扣/返还、日志记录 |
| tiered 计费 | `pkg/billingexpr/`, `service/tiered_settle.go` | 动态计费表达式、token 语义归一化、matched tier |
| 日志 | `model/log.go`, `service/log_info_generate.go` | 消费日志、管理审计、用户可见字段过滤 |
| 系统任务 | `service/system_task.go`, `controller/system_task_handlers.go` | 多 master DB lease 去重的任务框架 |
| 异步任务 | `service/task_polling.go`, `relay/channel/task/` | Suno/视频/MJ 类任务的提交、轮询、退款和终态结算 |

### 2. 前端精读索引

| 主题 | 文件/函数 | 为什么重要 |
| --- | --- | --- |
| 应用入口 | `web/default/src/main.tsx` | QueryClient、Router、全局 Provider、状态初始化 |
| 根路由 | `web/default/src/routes/__root.tsx` | setup 检查、全局布局、错误页、Toaster |
| 认证路由 | `web/default/src/routes/_authenticated/route.tsx` | 本地 user、session 验证、登录重定向 |
| API client | `web/default/src/lib/api.ts` | axios 实例、GET 去重、`New-Api-User`、401 处理 |
| auth store | `web/default/src/stores/auth-store.ts` | user/uid localStorage 持久化 |
| 系统配置 | `web/default/src/stores/system-config-store.ts`, `hooks/use-system-config.ts` | 系统名、logo、footer、货币展示 |
| 权限 | `web/default/src/lib/roles.ts`, `lib/admin-permissions.ts` | route guard 和管理员能力判断 |
| 侧边栏 | `hooks/use-sidebar-data.ts`, `hooks/use-sidebar-config.ts` | 根导航和系统设置二级导航 |
| Playground SSE | `features/playground/hooks/use-stream-request.ts` | 使用 `sse.js` 以 POST 接收 `/pg/chat/completions` 流 |
| 系统设置 | `features/system-settings/` | Option 编辑、分组配置、section registry |
| i18n | `i18n/config.ts`, `i18n/static-keys.ts`, `scripts/sync-i18n.mjs` | 多语言配置和静态 key 同步 |

### 3. 数据库兼容性注意

当前项目目标是 SQLite、MySQL、PostgreSQL 同时可用，日志库可额外使用 ClickHouse。改数据层时重点注意：

- 保留字列：`group`、`key` 用 `commonGroupCol`、`commonKeyCol`，日志库用 `logGroupCol`、`logKeyCol`。
- 布尔字面量：必要时使用 `commonTrueVal`、`commonFalseVal`。
- 优先 GORM API，少写 raw SQL。
- SQLite 不支持很多 `ALTER COLUMN`，补列通常用 `ALTER TABLE ... ADD COLUMN`。
- 日志查询需要考虑 ClickHouse 排序和 LIKE 转义差异。
- 现有部分历史字段使用 JSON 类型标签，新增类似字段时要确认三库行为，必要时使用 TEXT fallback。
- 一些事务路径使用类似 `FOR UPDATE` 的行锁意图；SQLite 锁语义不同，重要并发路径需要条件更新或 CAS 兜底。

## 建议阅读顺序

第一次读代码建议按这个顺序：

1. `main.go`：理解启动和后台任务。
2. `router/relay-router.go` 与 `router/api-router.go`：理解入口。
3. `middleware/auth.go` 与 `middleware/distributor.go`：理解认证和渠道选择。
4. `controller/relay.go`：理解 relay 主编排。
5. `relay/compatible_handler.go` 与 `relay/channel/adapter.go`：理解 adaptor 抽象。
6. `relay/channel/openai/`、`relay/channel/claude/`、`relay/channel/gemini/`：读三个代表性 provider。
7. `relay/helper/price.go`、`service/billing_session.go`、`service/quota.go`：理解计费。
8. `model/main.go`、`model/channel.go`、`model/token.go`、`model/user.go`、`model/log.go`：理解数据层。
9. `setting/ratio_setting/`、`setting/operation_setting/`、`setting/billing_setting/`：理解配置。
10. `web/default/src/main.tsx`、`web/default/src/routes/`、`web/default/src/features/`：理解前端。
