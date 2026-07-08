# new-api 源码术语与函数速查

这份文档是读源码时的索引卡片。它不按完整章节教学，而是把 new-api 中高频出现的类型、函数、字段、配置和日志概念集中解释，帮助你一边学习 Go，一边快速定位源码。

适合场景：

- 读 `controller.Relay()`、`relay.TextHelper()` 这种大函数时，不清楚某个字段或函数代表什么。
- 跟调用链跳转时，需要判断下一步应该看哪个包。
- 学 Go 时想知道某个语言特性在项目中的真实用法。

## 一、核心对象速查

| 名称 | 定义位置 | 一句话理解 | Go 学习点 |
| --- | --- | --- | --- |
| `RelayInfo` | `relay/common/relay_info.go` | relay 请求全链路上下文，集中保存用户、token、渠道、模型、计费、stream 状态 | 大 struct、指针传递、状态聚合 |
| `ChannelMeta` | `relay/common/relay_info.go` | `RelayInfo` 内的渠道元信息，从 Gin context 初始化 | struct 嵌套、字段分组 |
| `PriceData` | `types/price_data.go` | 请求前价格估算结果，后续结算继续使用 | 跨包 DTO、业务中间态 |
| `BillingSession` | `service/billing_session.go` | 单次请求的预扣费、结算、退款状态机 | 方法、互斥锁、幂等状态 |
| `FundingSource` | `service/funding_source.go` | 钱包或订阅的资金来源抽象 | interface、多态 |
| `RetryParam` | `service/channel_select.go` | 渠道选择和重试参数 | 指针字段、map set、方法封装 |
| `TokenCountMeta` | `types/request_meta.go` | token 估算需要的文本、max tokens、图片/音频信息 | DTO、估算输入 |
| `dto.Usage` | `dto/openai_response.go` | 上游返回或本地估算的 token 用量 | JSON DTO、嵌套详情 |
| `NewAPIError` | `types/error.go` | 项目统一错误类型，可转 OpenAI/Claude 错误响应 | error 封装、选项模式 |
| `TaskAdaptor` | `relay/channel/adapter.go` | 异步任务 provider 的接口 | interface、任务抽象 |
| `SystemTaskHandler` | `service/system_task.go` | 系统任务 runner 调用的 handler 接口 | interface、后台任务 |
| `RWMap[K,V]` | `types/rw_map.go` | 带读写锁的泛型 map | 泛型、并发安全 |

## 二、核心函数速查

### 启动与路由

| 函数 | 位置 | 做什么 | 下一步读哪里 |
| --- | --- | --- | --- |
| `main()` | `main.go` | 进程入口，初始化资源、启动任务和 HTTP server | `InitResources()`、`router.SetRouter()` |
| `InitResources()` | `main.go` | 初始化 env、日志、DB、Option、Redis、i18n、OAuth | `model.InitDB()`、`model.InitOptionMap()` |
| `router.SetRouter()` | `router/main.go` | 注册 API、dashboard、relay、video、web 路由 | `SetApiRouter()`、`SetRelayRouter()` |
| `SetRelayRouter()` | `router/relay-router.go` | 注册 `/v1`、`/v1beta`、`/mj`、`/suno` 等 relay 路由 | `controller.Relay()` |
| `SetApiRouter()` | `router/api-router.go` | 注册后台 API | 对应 `controller/*` |

### 认证与分发

| 函数 | 位置 | 做什么 | Go 学习点 |
| --- | --- | --- | --- |
| `authHelper()` | `middleware/auth.go` | 后台 session/access token 鉴权 | middleware、session、early return |
| `TokenAuth()` | `middleware/auth.go` | relay API key 鉴权，兼容 OpenAI/Claude/Gemini key 来源 | 字符串处理、context 写入 |
| `SetupContextForToken()` | `middleware/auth.go` | 把 token 信息写入 Gin context | context value |
| `Distribute()` | `middleware/distributor.go` | 读取模型、检查 token 模型限制、选择渠道 | 大型 middleware |
| `getModelRequest()` | `middleware/distributor.go` | 从不同协议/路径/body 中提取模型名 | 分支解析 |
| `SetupContextForSelectedChannel()` | `middleware/distributor.go` | 把选中渠道写入 context | context 到 RelayInfo 的前置状态 |

### Relay 主流程

| 函数 | 位置 | 做什么 | 下一步读哪里 |
| --- | --- | --- | --- |
| `controller.Relay()` | `controller/relay.go` | relay 请求总控：解析、估算、预扣、重试、错误响应 | `relayHandler()`、`getChannel()` |
| `helper.GetAndValidateRequest()` | `relay/helper/valid_request.go` | 根据 RelayFormat 解析请求 DTO | `dto/*` |
| `relaycommon.GenRelayInfo()` | `relay/common/relay_info.go` | 从 Gin context 和 request 构造 RelayInfo | `genBaseRelayInfo()` |
| `RelayInfo.InitChannelMeta()` | `relay/common/relay_info.go` | 从 context 复制渠道元信息到 RelayInfo | `ChannelMeta` |
| `relay.TextHelper()` | `relay/compatible_handler.go` | OpenAI 兼容文本请求主流程 | provider adaptor |
| `relay.ImageHelper()` | `relay/image_handler.go` | 图片请求主流程 | image adaptor |
| `relay.AudioHelper()` | `relay/audio_handler.go` | 音频请求主流程 | audio adaptor |
| `relay.ResponsesHelper()` | `relay/responses_handler.go` | Responses API 主流程 | responses adaptor |

### 渠道与 provider

| 函数 | 位置 | 做什么 | Go 学习点 |
| --- | --- | --- | --- |
| `service.CacheGetRandomSatisfiedChannel()` | `service/channel_select.go` | 按分组、模型、priority、weight 选择渠道 | retry 状态、map set |
| `model.GetRandomSatisfiedChannelWithExclusions()` | `model/channel_cache.go` | 从渠道缓存中取满足条件的候选 | 缓存、随机权重 |
| `relay.GetAdaptor()` | `relay/relay_adaptor.go` | APIType 到 provider adaptor 的工厂 | interface 工厂 |
| `channel.DoApiRequest()` | `relay/channel/api_request.go` | 构造并发送上游 HTTP 请求 | `http.Request`、`io.Reader` |
| `openai.Adaptor.GetRequestURL()` | `relay/channel/openai/adaptor.go` | OpenAI/Azure/OpenRouter URL 构造 | provider 差异 |
| `openai.OpenaiHandler()` | `relay/channel/openai/relay-openai.go` | OpenAI 非流式响应解析和写回 | JSON、usage |
| `openai.OaiStreamHandler()` | `relay/channel/openai/relay-openai.go` | OpenAI SSE 流式响应处理 | stream scanner |

### 计费与日志

| 函数 | 位置 | 做什么 | 下一步读哪里 |
| --- | --- | --- | --- |
| `helper.ModelPriceHelper()` | `relay/helper/price.go` | 请求前价格估算，生成 PriceData | `modelPriceHelperTiered()` |
| `service.PreConsumeBilling()` | `service/billing.go` | 创建 BillingSession 并预扣费 | `NewBillingSession()` |
| `BillingSession.preConsume()` | `service/billing_session.go` | 预扣 token 和资金源，处理信任额度 | `FundingSource` |
| `service.SettleBilling()` | `service/billing.go` | 按实际 quota 补扣或返还 | `BillingSession.Settle()` |
| `service.PostTextConsumeQuota()` | `service/text_quota.go` | 文本请求实际计费、结算、记录日志 | `calculateTextQuotaSummary()` |
| `service.TryTieredSettle()` | `service/tiered_settle.go` | tiered expression 后结算 | `pkg/billingexpr` |
| `service.GenerateTextOtherInfo()` | `service/log_info_generate.go` | 生成消费日志 Other 字段 | `model.RecordConsumeLog()` |
| `model.RecordConsumeLog()` | `model/log.go` | 写消费日志 | `Log` model |

### 系统任务

| 函数 | 位置 | 做什么 | Go 学习点 |
| --- | --- | --- | --- |
| `service.StartSystemTaskRunner()` | `service/system_task.go` | 启动系统任务 runner | `sync.Once`、goroutine |
| `runSystemTaskScheduler()` | `service/system_task.go` | 创建到期任务 | ticker、时间判断 |
| `runSystemTaskClaimPass()` | `service/system_task.go` | 抢占 pending task 并分发执行 | DB lease、闭包 |
| `runWithLeaseHeartbeat()` | `service/system_task.go` | 执行任务并续租 lease | context、ticker、channel |
| `service.RunTaskPollingOnce()` | `service/task_polling.go` | 异步任务轮询一轮 | 分组、外部请求 |

## 三、高频字段速查

### RelayInfo 字段

| 字段 | 含义 | 典型赋值位置 | 典型使用位置 |
| --- | --- | --- | --- |
| `UserId` | 用户 id | `genBaseRelayInfo()` | 计费、日志 |
| `TokenId` | API token id | `genBaseRelayInfo()` | 预扣 token quota、日志 |
| `TokenGroup` | token 指定分组 | `genBaseRelayInfo()` | 渠道选择、计费 |
| `UsingGroup` | 实际使用分组 | `genBaseRelayInfo()` / `HandleGroupRatio()` | 分组倍率、日志 |
| `OriginModelName` | 客户端请求模型 | `genBaseRelayInfo()` | 日志、模型映射 |
| `UpstreamModelName` | 上游实际模型 | `InitChannelMeta()` / `ModelMappedHelper()` | adaptor 请求 |
| `PricingModelName` | 计费模型名 | `genBaseRelayInfo()` / fallback 映射 | `ModelPriceHelper()` |
| `IsStream` | 是否流式 | `genBaseRelayInfo()` / response header | stream handler |
| `RelayMode` | 具体请求类型 | `Path2RelayMode()` | `relayHandler()` |
| `RelayFormat` | 下游协议格式 | `GenRelayInfo*()` | 响应转换 |
| `PriceData` | 价格估算结果 | `ModelPriceHelper()` | 后结算 |
| `Billing` | 计费会话 | `PreConsumeBilling()` | `SettleBilling()` / 失败退款 |
| `TieredBillingSnapshot` | tiered 计费快照 | `modelPriceHelperTiered()` | `TryTieredSettle()` |
| `RequestConversionChain` | 请求格式转换链 | `InitRequestConversionChain()` / helper append | 日志展示 |
| `ChannelMeta` | 渠道元信息 | `InitChannelMeta()` | adaptor |

### Channel 字段

| 字段 | 含义 | 学习点 |
| --- | --- | --- |
| `Type` | 渠道类型 | 常量映射到 APIType |
| `Key` | 上游 key，可能多 key | 多行拆分、JSON 数组 |
| `BaseURL` | 上游基础地址 | 指针字段 |
| `Models` | 支持模型列表 | 能力缓存 |
| `Group` | 可用分组 | 保留字列兼容 |
| `ModelMapping` | 模型映射配置 | JSON/string 解析 |
| `StatusCodeMapping` | 上游状态码映射 | 错误处理 |
| `Priority` | 优先级 | 渠道选择 |
| `Weight` | 同优先级权重 | 随机选择 |
| `AutoBan` | 错误时自动禁用 | processChannelError |
| `Setting` | 渠道设置 | DTO JSON |
| `ParamOverride` | 请求参数覆盖 | JSON patch / audit |
| `HeaderOverride` | 请求头覆盖 | placeholder、透传规则 |
| `ChannelInfo` | 多 key 状态 | `driver.Valuer` / `sql.Scanner` |

### Token 字段

| 字段 | 含义 | 使用位置 |
| --- | --- | --- |
| `Key` | API key 主体 | `TokenAuth()` |
| `Status` | token 状态 | `ValidateUserToken()` |
| `RemainQuota` | token 剩余额度 | 预扣 token quota |
| `UnlimitedQuota` | 是否无限额度 | trust quota 判断 |
| `ExpiredTime` | 过期时间 | `ValidateUserToken()` |
| `ModelLimitsEnabled` | 是否启用模型白名单 | `Distribute()` |
| `ModelLimits` | 模型白名单 JSON/string | `GetModelLimitsMap()` |
| `AllowIps` | IP 限制 | `TokenAuth()` |
| `Group` | token 绑定分组 | `TokenAuth()` |
| `CrossGroupRetry` | auto 分组跨组重试 | 渠道选择 |

## 四、项目术语速查

| 术语 | 含义 |
| --- | --- |
| relay | AI API 代理请求链路，不只是转发，还包括鉴权、选渠道、转换、计费、日志 |
| channel | 一个上游 provider 配置，如 OpenAI/Azure/Claude/Gemini 某个 key 和 baseURL |
| APIType | provider 协议类型，用于选择 adaptor |
| ChannelType | 数据库/配置中的渠道类型，可能多个 ChannelType 映射同一 APIType |
| adaptor | provider 适配器，实现请求转换、URL/header、上游请求和响应处理 |
| RelayFormat | 下游请求/响应协议格式，如 OpenAI、Claude、Gemini、Responses |
| RelayMode | 具体接口模式，如 chat、image、audio、embedding、responses |
| group | 用户/token/渠道分组，影响可用渠道和价格倍率 |
| auto group | 自动分组，按用户可用分组尝试选择渠道 |
| model mapping | 客户端模型名到上游模型名的映射 |
| fallback model | 当前渠道某模型不可用时可替代的模型 |
| pre-consume | 请求发上游前预扣费 |
| settle | 上游返回 usage 后按实际 quota 补扣或返还 |
| trust quota | 用户/token 额度足够时跳过钱包预扣的信任阈值 |
| tiered expr | 表达式计费模式，按真实价格表达式结算 |
| BillingSnapshot | 预扣时冻结的 tiered 计费规则 |
| usage semantic | usage 字段语义，如 OpenAI 总量语义或 Claude 文本/缓存分离语义 |
| stream options | OpenAI stream_options，通常用于请求流式 usage |
| affinity | 渠道亲和性，优先复用符合规则的渠道 |
| system task | 持久化到 DB 的后台任务，有 lease 和运行记录 |
| task adaptor | 视频/Suno/MJ 等异步任务上游适配器 |

## 五、Go 语法到源码例子速查

| Go 语法/概念 | 源码例子 | 读法 |
| --- | --- | --- |
| `package main` | `main.go` | 进程入口 |
| import 分组 | `main.go` | 标准库、项目包、第三方包 |
| struct tag | `model/user.go`, `model/channel.go` | JSON/GORM/validate |
| 指针接收者 | `model.Token.Clean()` | 修改对象状态 |
| 多返回值 | `model.ValidateUserToken()` | 值 + error |
| `errors.Is` | `controller.Login()` | sentinel error 判断 |
| interface | `relay/channel/adapter.go` | provider 多态 |
| 工厂函数 | `relay.GetAdaptor()` | switch 返回接口实现 |
| 闭包 middleware | `middleware.RequestId()` | 返回 `func(*gin.Context)` |
| context value | `middleware/auth.go` | request scoped 状态 |
| defer | `controller.Relay()` | 错误响应/退款收口 |
| recover | `common/go-channel.go` | 捕获 send closed channel panic |
| goroutine | `main.go`, `service/system_task.go` | 后台任务 |
| channel | `service/system_task.go` | wakeup/done 信号 |
| mutex | `types/rw_map.go`, `BillingSession` | 并发保护 |
| 泛型 | `types.RWMap[K,V]` | 类型安全容器 |
| `io.Reader` | `relay/channel/api_request.go` | 上游请求体 |
| `json.RawMessage` | `dto/*` | 延迟解析/透传 JSON |
| GORM chain | `model/token.go` | 查询构造 |
| transaction | `model/*` | 原子更新 |
| table test | `*_test.go` | 行为测试 |

## 六、读代码时的“下一跳”规则

当你在源码中看到这些调用，可以按下面方式跳：

| 看到 | 下一跳 |
| --- | --- |
| `common.GetContextKey...` | 回到 middleware，看是谁写入 context |
| `relayInfo.PriceData` | 回到 `helper.ModelPriceHelper()` 看如何生成 |
| `relayInfo.Billing` | 回到 `service.PreConsumeBilling()` 和 `NewBillingSession()` |
| `adaptor.Convert...` | 跳到具体 provider 的 `adaptor.go` |
| `DoApiRequest` | 读公共上游 HTTP 请求逻辑 |
| `PostTextConsumeQuota` | 读 usage 到 quota 的实际结算 |
| `RecordConsumeLog` | 读日志表和 `Other` 字段 |
| `types.NewError` | 读 `types/error.go` 的错误选项 |
| `GetRandomSatisfiedChannel...` | 读渠道缓存和优先级/权重 |
| `UpdateOption` | 读 `model/option.go` 和对应 `setting` 包 |
| `RegisterSystemTaskHandler` | 读系统任务 runner 和 handler |

## 七、常见困惑

### 1. 为什么有 Gin context，又有 RelayInfo？

Gin context 适合 middleware 之间传递 request-scoped 值；`RelayInfo` 适合 relay 主流程和 provider adaptor 之间传递强结构化状态。前者灵活，后者可读性和类型约束更好。

### 2. OriginModelName、UpstreamModelName、PricingModelName 有什么区别？

- `OriginModelName`：客户端请求的模型。
- `UpstreamModelName`：发给上游的模型，可能被映射。
- `PricingModelName`：计费用的模型，fallback 或特殊模式下可能和前两者不同。

### 3. 为什么请求前要预扣费？

为了防止用户额度不足却已经消耗上游成本。请求成功后根据真实 usage 后结算，多退少补；请求失败则退款或收违规费。

### 4. 为什么有 PriceData 还要 BillingSession？

`PriceData` 是价格估算数据；`BillingSession` 是资金生命周期状态。前者回答“预计多少钱”，后者负责“怎么扣、怎么退、是否已结算”。

### 5. 为什么 provider 不直接自己发 HTTP？

公共的 `DoApiRequest()` 统一处理代理、header override、ContentLength、SSE header、HTTP client、trace 等横切逻辑。provider 只负责差异化的 URL/header/转换。

### 6. 为什么有些请求 DTO 要用指针标量？

因为可选字段要区分 absent 和 explicit zero。比如客户端显式传 `temperature: 0`，上游应该收到 0；如果用非指针加 `omitempty`，0 会被丢掉。

### 7. 为什么日志 Other 字段这么复杂？

日志既服务用户账单展示，也服务管理员排障。它需要记录模型映射、渠道选择、计费倍率、订阅、tiered 表达式、流状态、param override 等上下文。

## 八、自测题

1. `TokenAuth()` 最终写入了哪些后续 relay 必需的 context？
2. `Distribute()` 为什么要读取请求 body？它读完后如何保证后续还能读？
3. `RelayInfo.InitChannelMeta()` 从哪里取渠道信息？
4. `ModelPriceHelper()` 在什么情况下走 tiered expression？
5. `BillingSession.Refund()` 为什么要判断 `settled` 和 `fundingSettled`？
6. `OpenaiHandler()` 在 usage 缺失时如何补齐？
7. `OaiStreamHandler()` 如何决定是否用本地 token 估算？
8. 新增一个 provider 时，哪些方法必须实现？
9. `systemTaskWakeup` channel 的作用是什么？
10. `RWMap.ReadAll()` 为什么不直接返回内部 map？

