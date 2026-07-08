# new-api 包级源码学习地图

这份文档按 new-api 的 Go package/目录组织，帮助你把“Go 语言点”和“项目业务模块”连起来。它适合日常学习时使用：每天选择一个目录，先知道这个目录负责什么，再带着 Go 主题去读源码。

推荐搭配：

- `go-source-learning-guide.md`：按 Go 语言点学习。
- `source-walkthroughs-for-go-learners.md`：按真实调用链精读。
- `new-api-implementation-guide.md`：按系统架构理解全局。

## 学习方法

每个包都按这四个问题阅读：

1. 这个包在架构中属于哪一层？
2. 这个包暴露了哪些核心类型和函数？
3. 它主要练习哪些 Go 能力？
4. 它被谁调用，又调用谁？

建议顺序：

```text
common/constant/types/dto
  -> model
  -> setting
  -> middleware
  -> router/controller
  -> service
  -> relay
  -> pkg
```

## 一、基础工具层：common

### 职责

`common/` 是项目公共工具层，放全局配置、JSON wrapper、Redis、请求体复用、环境变量、配额常量、日志辅助、系统监控等。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `common/env.go` | 环境变量读取、全局变量初始化 |
| `common/json.go` | JSON wrapper、`any`、`json.RawMessage` |
| `common/body_storage.go` | 请求体复用、内存/磁盘存储、`io.Reader` |
| `common/gin.go` | Gin 请求体辅助、context 取值 |
| `common/redis.go` | Redis 初始化、外部资源连接 |
| `common/database.go` | 数据库类型枚举、跨库判断 |
| `common/go-channel.go` | channel send、defer、recover |
| `common/quota.go` | quota 单位和格式 |

### Go 学习点

- package 全局变量如何初始化和被其他包读取。
- wrapper 函数如何统一第三方/标准库调用。
- `io.Reader`、`io.Seeker`、临时文件的使用。
- `defer` + `recover` 的边界。
- context key 和类型转换辅助。

### 阅读任务

1. 读 `common/json.go`，理解为什么项目要求业务代码用 `common.Marshal` / `common.Unmarshal`。
2. 读 `common/body_storage.go`，画出小请求和大请求分别存在哪里。
3. 读 `common/database.go`，找出主库和日志库类型如何保存。

### 小练习

解释一个 relay 请求为什么不能直接读取一次 `c.Request.Body` 后丢弃。至少从 `Distribute`、`controller.Relay`、重试三个角度说明。

## 二、常量层：constant

### 职责

`constant/` 放稳定的业务常量，例如渠道类型、API 类型、context key、任务平台、Azure 默认版本、finish reason 等。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `constant/channel.go` | 渠道类型常量、默认 base URL |
| `constant/api_type.go` | provider API 类型 |
| `constant/context_key.go` | request context key |
| `constant/task.go` | 异步任务平台和开关 |
| `constant/multi_key_mode.go` | 多 key 模式 |

### Go 学习点

- `const` 分组。
- 类型别名或自定义类型承载枚举语义。
- 常量值一旦用于数据库/配置，就不要随意改动。

### 阅读任务

1. 找 `ChannelTypeOpenAI`、`ChannelTypeAnthropic`、`ChannelTypeGemini`。
2. 再跳到 `common/api_type.go` 看 channel type 如何映射到 API type。
3. 查一个 context key 从写入到读取的完整路径。

### 小练习

如果新增一个 provider，列出至少三个需要补的常量或映射位置。

## 三、跨模块类型层：types

### 职责

`types/` 放跨层共享的类型，不绑定具体 model 或 controller。例如 relay format、错误类型、价格数据、集合类型、请求 metadata。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `types/error.go` | 统一错误结构、OpenAI/Claude 错误转换 |
| `types/price_data.go` | 计费中间数据 |
| `types/relay_format.go` | relay 请求/响应格式 |
| `types/rw_map.go` | 泛型、并发安全 map |
| `types/channel_error.go` | 渠道错误上下文 |

### Go 学习点

- struct 如何作为跨层数据契约。
- 泛型类型参数：`RWMap[K comparable, V any]`。
- 方法集和 JSON marshal/unmarshal hook。
- 错误类型如何携带 HTTP 状态、错误码、重试选项。

### 阅读任务

1. 读 `types.RWMap`，理解 `ReadAll()` 为什么返回 copy。
2. 读 `types.NewAPIError` 相关方法，找 `ToOpenAIError()` 和 `ToClaudeError()`。
3. 读 `types.PriceData`，对照 `relay/helper/price.go` 看字段如何填充。

### 小练习

画出 `NewAPIError` 从创建到返回给客户端的路径：创建位置、包装位置、最终响应位置。

## 四、DTO 层：dto

### 职责

`dto/` 是请求/响应对象层。它描述外部 API 协议和内部传输结构，尤其是 OpenAI、Claude、Gemini、音频、图片、embedding、rerank、任务等格式。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `dto/openai_request.go` | OpenAI 请求结构、stream 判断、token meta |
| `dto/openai_response.go` | OpenAI 响应和 usage |
| `dto/claude.go` | Claude Messages 结构 |
| `dto/gemini.go` | Gemini 请求响应结构 |
| `dto/channel_settings.go` | 渠道配置 DTO |
| `dto/user_settings.go` | 用户设置 |

### Go 学习点

- JSON tag 和 `omitempty`。
- 指针标量保留显式零值。
- interface-like DTO 方法，例如 `Request` 接口。
- `json.RawMessage` 延迟解析。

### 阅读任务

1. 找 `GeneralOpenAIRequest` 中的可选字段，观察哪些是指针。
2. 找 `IsStream(c)`，理解 stream 判断为什么在 DTO 层。
3. 读 `openai_request_zero_value_test.go`，理解为什么显式 `0` 不能被丢掉。

### 小练习

任选一个请求字段，判断它应该是值类型还是指针类型，并说明 absent、0、false 三种情况如何表现。

## 五、数据层：model

### 职责

`model/` 是 GORM 数据模型、数据库初始化、迁移、缓存和 CRUD 层。它直接面对 SQLite/MySQL/PostgreSQL/ClickHouse 兼容问题。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `model/main.go` | DB 初始化、迁移、跨库列名 |
| `model/user.go` | 用户模型、登录校验、用户查询 |
| `model/token.go` | API token 模型、额度和状态校验 |
| `model/channel.go` | 渠道模型、多 key、模型映射字段 |
| `model/channel_cache.go` | 渠道内存缓存和选择候选 |
| `model/log.go` | 日志模型、查询、字段过滤 |
| `model/task.go` | 异步任务模型 |
| `model/system_task.go` | 系统任务和 DB lease |
| `model/subscription.go` | 订阅模型和预扣记录 |

### Go 学习点

- GORM struct tag。
- `DB.Where(...).Find(...)` 查询链式调用。
- `DB.Transaction(...)`。
- `gorm.DeletedAt` 软删除。
- `sql.Scanner` / `driver.Valuer` 自定义字段序列化。
- 跨库 SQL 差异。

### 阅读任务

1. 从 `model.InitDB()` 读到 `migrateDB()`。
2. 从 `model.ValidateUserToken()` 读到 `GetTokenByKey()`。
3. 从 `model.InitChannelCache()` 读到 `GetRandomSatisfiedChannelWithExclusions()`。
4. 从 `model.RecordConsumeLog()` 读到用户日志过滤。

### 小练习

解释 `commonGroupCol` 和 `commonKeyCol` 解决了什么问题。再找一个没有用 GORM 自动生成 SQL、必须自己关心方言的查询。

## 六、配置层：setting

### 职责

`setting/` 保存运行时配置域。数据库 `options` 表加载后，会写入这些包里的全局配置或并发安全 map。

### 重点目录

| 目录 | 学习重点 |
| --- | --- |
| `setting/ratio_setting/` | 模型倍率、模型价格、分组倍率、缓存/音频/图片倍率 |
| `setting/operation_setting/` | 运营开关、支付、重试状态码、自动禁用规则 |
| `setting/billing_setting/` | tiered expression 配置 |
| `setting/system_setting/` | OIDC、Passkey、主题、法律文本 |
| `setting/model_setting/` | provider/model 特定行为 |
| `setting/config/` | 配置注册和导出 |

### Go 学习点

- 包级变量和热更新。
- map 配置从 JSON 字符串加载。
- `init()` 注册配置。
- 并发读写配置时如何避免 data race。

### 阅读任务

1. 从 `model.InitOptionMap()` 跳到某个 `ratio_setting` 更新函数。
2. 读 `setting/billing_setting/tiered_billing.go`，理解表达式配置如何存储。
3. 读 `setting/operation_setting/status_code_ranges.go`，理解重试/禁用状态码规则。

### 小练习

选择一个 Option key，追踪：默认值在哪里写入、数据库值在哪里覆盖、业务代码在哪里读取。

## 七、中间件层：middleware

### 职责

`middleware/` 处理请求横切能力。它在 controller 前后执行，负责认证、限流、渠道分发、日志、CORS、i18n、审计、请求体清理等。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `middleware/auth.go` | UserAuth/AdminAuth/RootAuth/TokenAuth |
| `middleware/distributor.go` | 读取模型、检查 token 限制、选择渠道 |
| `middleware/logger.go` | 请求日志 |
| `middleware/rate-limit.go` | 限流 |
| `middleware/model-rate-limit.go` | 模型请求限流 |
| `middleware/audit.go` | 管理操作审计 |
| `middleware/body_cleanup.go` | 请求体资源清理 |
| `middleware/request-id.go` | request id 注入 |

### Go 学习点

- Gin middleware 闭包。
- `c.Next()`、`c.Abort()`。
- request-scoped context。
- early return 控制链路。
- 中间件之间通过 context 协作。

### 阅读任务

1. 读 `RequestId()`，理解最小 middleware。
2. 读 `TokenAuth()`，找出 key 归一化步骤。
3. 读 `Distribute()`，找出它写入了哪些 channel context。

### 小练习

列出 `/v1/chat/completions` 的 middleware 顺序，并说明每个 middleware 给后续步骤准备了什么信息。

## 八、路由层：router

### 职责

`router/` 负责挂载 URL、group、中间件和 controller。它不应该承载复杂业务逻辑。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `router/main.go` | 总路由装配 |
| `router/api-router.go` | 后台 API |
| `router/relay-router.go` | OpenAI/Claude/Gemini/MJ/Suno relay |
| `router/channel-router.go` | 渠道管理和权限 |
| `router/video-router.go` | 视频任务接口 |
| `router/web-router.go` | embedded 前端资源 |

### Go 学习点

- Gin group。
- middleware 组合。
- 匿名函数 handler。
- 路由顺序和 wildcard。

### 阅读任务

1. 从 `SetRouter()` 看整个 HTTP surface。
2. 对比 `/api/user/self` 和 `/v1/chat/completions` 的认证方式。
3. 读 `SetWebRouter()`，理解 SPA fallback。

### 小练习

找一个带 `:id` 参数的路由和一个带 wildcard 的路由，说明 controller 如何读取参数。

## 九、控制器层：controller

### 职责

`controller/` 是 HTTP handler 层。它负责解析参数、调用 service/model、返回 JSON 或 relay 响应。复杂业务应下沉到 service。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `controller/relay.go` | relay 主编排 |
| `controller/user.go` | 登录、注册、用户信息 |
| `controller/token.go` | API key 管理 |
| `controller/channel.go` | 渠道 CRUD |
| `controller/option.go` | 系统设置 |
| `controller/log.go` | 日志查询 |
| `controller/system_task.go` | 系统任务 API |
| `controller/system_task_handlers.go` | 定时系统任务 handler |

### Go 学习点

- handler 函数签名。
- 参数解析和校验。
- 统一响应结构。
- `defer` 收口错误响应。
- controller 和 service 的职责边界。

### 阅读任务

1. 读 `controller.Login()`，理解最简单后台 handler。
2. 读 `controller.Relay()`，理解复杂 handler 如何拆主线。
3. 读一个 channel controller，观察如何调用 authz 和 model。

### 小练习

把 `controller.Relay()` 分成 8 个逻辑块，每块写一句话，不要超过 20 字。

## 十、业务服务层：service

### 职责

`service/` 是业务编排层，放跨 controller/model/relay 的逻辑：计费、渠道选择、任务轮询、权限、转换、通知、文件、订阅、用量等。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `service/channel_select.go` | 渠道选择和 auto group |
| `service/billing_session.go` | 预扣费/结算/退款生命周期 |
| `service/billing.go` | 计费入口 |
| `service/text_quota.go` | 文本 quota 计算和日志 |
| `service/quota.go` | 音频/realtime quota |
| `service/tiered_settle.go` | tiered expression 结算 |
| `service/error.go` | 上游错误解析 |
| `service/task_polling.go` | 异步任务轮询 |
| `service/system_task.go` | 系统任务 runner |
| `service/authz/` | 权限系统 |
| `service/relayconvert/` | OpenAI/Responses 转换 |

### Go 学习点

- 业务状态机。
- interface 隔离资金源、任务 adaptor 等变化点。
- context cancellation。
- goroutine worker。
- decimal 计算。
- 错误回滚和幂等。

### 阅读任务

1. 读 `BillingSession`，画出 preConsume、Settle、Refund 三个状态变化。
2. 读 `CacheGetRandomSatisfiedChannel()`，理解 retry 如何影响 priority。
3. 读 `RunTaskPollingOnce()`，理解任务按平台分组处理。

### 小练习

解释为什么 `BillingSession` 需要 `settled`、`refunded`、`fundingSettled` 三个布尔字段，而不是只用一个状态字段。

## 十一、Relay 层：relay

### 职责

`relay/` 是 AI API 代理核心。它不只是转发 HTTP，还负责协议转换、stream 处理、图片/音频/embedding/rerank/Responses/task 分流。

### 重点文件

| 文件 | 学习重点 |
| --- | --- |
| `relay/compatible_handler.go` | OpenAI 兼容文本主流程 |
| `relay/relay_adaptor.go` | APIType 到 adaptor 映射 |
| `relay/common/relay_info.go` | RelayInfo 全链路上下文 |
| `relay/helper/price.go` | 请求前价格估算 |
| `relay/helper/model_mapped.go` | 模型映射 |
| `relay/helper/stream_scanner.go` | SSE 扫描 |
| `relay/responses_handler.go` | Responses API |
| `relay/relay_task.go` | 异步任务 submit |

### Go 学习点

- 大型业务主流程拆分。
- interface 动态分派。
- `io.Reader` 请求体。
- SSE 流式处理。
- 状态集中到 struct，减少 context 依赖。

### 阅读任务

1. 读 `RelayInfo`，把字段分成用户、token、渠道、请求、计费、stream 六类。
2. 读 `TextHelper()`，标出 pass-through 和转换请求两条路径。
3. 读 `streamSupportedChannels`，理解新渠道支持 stream options 时要补哪里。

### 小练习

从 `RelayInfo` 里选择 10 个字段，说明它们分别在哪一步被赋值、在哪一步被使用。

## 十二、Provider 层：relay/channel

### 职责

`relay/channel/` 下每个 provider 实现 adaptor。它们负责把统一请求转换成上游协议，并把上游响应转换回下游期望格式。

### 重点目录

| 目录 | 学习重点 |
| --- | --- |
| `relay/channel/openai/` | OpenAI/Azure/OpenRouter 等兼容实现 |
| `relay/channel/claude/` | Anthropic Messages |
| `relay/channel/gemini/` | Gemini native 和 Responses 兼容 |
| `relay/channel/aws/` | AWS Bedrock |
| `relay/channel/vertex/` | Vertex AI |
| `relay/channel/advancedcustom/` | 高级自定义渠道 |
| `relay/channel/task/` | 视频/Suno 等任务型 adaptor |

### Go 学习点

- 每个 provider 用 struct 实现同一个 interface。
- request/response DTO 转换。
- provider 特殊 header 和 URL。
- stream 与非 stream 双路径。
- 测试 provider 特殊语义。

### 阅读任务

1. 对比 `openai.Adaptor` 和 `claude.Adaptor` 的 `GetRequestURL()`。
2. 对比它们的 `SetupRequestHeader()`。
3. 找 `DoResponse()` 如何区分 stream 和非 stream。

### 小练习

设计一个新 provider adaptor 的文件结构。至少列出：`adaptor.go`、`constants.go`、`dto.go`、`relay-*.go` 中分别放什么。

## 十三、内部包：pkg

### 职责

`pkg/` 放相对独立的内部能力。

### 重点目录

| 目录 | 学习重点 |
| --- | --- |
| `pkg/billingexpr/` | 表达式计费引擎 |
| `pkg/cachex/` | Redis/内存混合缓存 |
| `pkg/perf_metrics/` | 性能指标聚合 |
| `pkg/ionet/` | io/network 相关工具 |

### Go 学习点

- 独立 package 的 API 设计。
- 编译缓存。
- AST introspection。
- 泛型/接口/锁。
- 单元测试保护核心逻辑。

### 阅读任务

1. 先读 `pkg/billingexpr/expr.md`，再读代码。
2. 读 `CompileFromCache()`，理解表达式缓存。
3. 读 `RunExprWithRequest()`，理解运行环境变量和函数。
4. 读 `settle.go`，理解 quota 转换。

### 小练习

写出一个表达式：

```text
tier("base", p * 2 + c * 8 + cr * 0.2)
```

解释 `p`、`c`、`cr` 的含义，以及为什么使用 `cr` 后 cache read token 要从 `p` 中扣除。

## 十四、OAuth 和认证扩展：oauth、service/passkey

### 职责

- `oauth/` 处理 GitHub、Discord、OIDC、LinuxDO、自定义 OAuth provider。
- `service/passkey/` 处理 WebAuthn/Passkey。

### Go 学习点

- provider registry。
- interface 或结构化配置扩展认证方式。
- 外部 OAuth 回调和 state 校验。
- 安全相关代码如何隔离。

### 阅读任务

1. 读 `oauth/registry.go`，理解 provider 如何注册。
2. 读一个 provider 实现，例如 `oauth/github.go`。
3. 读 `service/passkey/service.go`，理解 passkey begin/finish 两阶段。

### 小练习

比较密码登录、OAuth 登录、Passkey 登录：它们最终是否都会走类似 `setupLogin` 的 session 写入逻辑？

## 十五、按学习天数安排

### 第 1 天：入口和路由

- 读 `go.mod`、`main.go`、`router/main.go`。
- 输出：启动流程图。

### 第 2 天：认证和 context

- 读 `middleware/request-id.go`、`middleware/auth.go`。
- 输出：session auth 和 token auth 对比表。

### 第 3 天：渠道选择

- 读 `middleware/distributor.go`、`service/channel_select.go`、`model/channel_cache.go`。
- 输出：渠道选择流程图。

### 第 4 天：Relay 主线

- 读 `controller/relay.go`、`relay/compatible_handler.go`、`relay/common/relay_info.go`。
- 输出：`RelayInfo` 字段分类表。

### 第 5 天：OpenAI provider

- 读 `relay/channel/openai/adaptor.go`、`relay/channel/openai/relay-openai.go`。
- 输出：stream 与非 stream 差异表。

### 第 6 天：计费

- 读 `relay/helper/price.go`、`service/billing_session.go`、`service/text_quota.go`。
- 输出：预扣费/结算/退款状态图。

### 第 7 天：数据层

- 读 `model/main.go`、`model/user.go`、`model/token.go`、`model/log.go`。
- 输出：数据库兼容注意清单。

### 第 8 天：系统任务和并发

- 读 `service/system_task.go`、`controller/system_task_handlers.go`、`model/system_task.go`。
- 输出：DB lease + heartbeat 时序图。

### 第 9 天：表达式计费

- 读 `pkg/billingexpr/expr.md`、`pkg/billingexpr/*.go`、`service/tiered_settle.go`。
- 输出：一个模型表达式的预扣和结算示例。

### 第 10 天：前端连接点

- 读 `web/default/src/lib/api.ts`、`web/default/src/stores/auth-store.ts`、相关 feature `api.ts`。
- 输出：一个后台页面从按钮到后端 controller 的链路。

## 十六、完成一轮学习后的自测

能独立回答下面问题，说明你已经能一边学 Go 一边读 new-api：

1. `TokenAuth()` 和 `UserAuth()` 的区别是什么？
2. `Distribute()` 为什么必须在 `controller.Relay()` 之前？
3. `RelayInfo` 解决了什么问题？
4. provider adaptor interface 的核心方法有哪些？
5. OpenAI 非流式响应在哪里解析 usage？
6. stream 响应没有 usage 时如何估算？
7. 预扣费和后结算分别在哪里发生？
8. tiered expression 为什么需要 `BillingSnapshot`？
9. `commonGroupCol` 为什么存在？
10. 系统任务为什么需要 DB lease？
11. `RWMap` 为什么返回 map copy？
12. 新增 provider 至少要改哪些包？

