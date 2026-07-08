# 边学 Go 边读 new-api 源码

这份文档面向已经掌握 Go 基本语法、但希望通过 new-api 源码继续进阶的读者。它不是 Go 语法手册，而是一条源码学习路线：每一章先说明要学的 Go 概念，再指向项目真实代码，最后给出阅读任务和小练习。

配合阅读：

- 全景架构文档：`docs/architecture/new-api-implementation-guide.md`
- Go 模块入口：`go.mod`
- 后端入口：`main.go`

## 使用方法

建议按章节顺序读。每章都采用同一结构：

```text
要学什么 -> 在 new-api 哪些文件里看 -> 如何读这段源码 -> 做一个小练习
```

阅读时不要只看结论。最好打开对应源码，把函数签名、类型定义、调用链都跳一遍。对 Go 学习最有效的方式是：先读一条完整链路，再回头总结语言特性为什么这样用。

## 学习路线总览

| 阶段 | Go 能力 | new-api 源码入口 |
| --- | --- | --- |
| 1 | module、package、import、入口函数 | `go.mod`, `main.go` |
| 2 | 函数、方法、指针接收者、返回 error | `model/token.go`, `model/channel.go` |
| 3 | struct、tag、嵌入字段、DTO | `model/user.go`, `model/channel.go`, `dto/openai_request.go` |
| 4 | interface 和多态 | `relay/channel/adapter.go`, `relay/relay_adaptor.go` |
| 5 | context、中间件、闭包 | `middleware/auth.go`, `middleware/request-id.go` |
| 6 | error 设计与统一错误返回 | `types/error.go`, `service/error.go`, `controller/relay.go` |
| 7 | map、slice、泛型、并发安全 | `types/rw_map.go`, `setting/ratio_setting/` |
| 8 | goroutine、channel、defer、recover | `main.go`, `service/system_task.go`, `common/go-channel.go` |
| 9 | JSON、io.Reader、请求体复用 | `common/json.go`, `common/body_storage.go`, `relay/compatible_handler.go` |
| 10 | GORM、事务、迁移、跨库兼容 | `model/main.go`, `model/user.go`, `model/log.go` |
| 11 | HTTP handler 和 Gin 路由 | `router/`, `controller/` |
| 12 | 测试与回归保护 | `*_test.go`, `pkg/billingexpr/billingexpr_test.go` |

## 一、从 module 和入口开始

### 要学什么

- `go.mod` 如何定义模块路径。
- 一个 Go 进程从 `package main` 的 `main()` 开始。
- import 分组如何体现标准库、项目内包、第三方包。
- 初始化顺序为什么重要。

### 源码入口

- `go.mod`
- `main.go`

### 读法

先看 `go.mod`：

```go
module github.com/QuantumNous/new-api
```

这决定了项目内部 import 都从 `github.com/QuantumNous/new-api/...` 开始。例如 `main.go` 中：

```go
import (
    "context"
    "net/http"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/model"
    "github.com/QuantumNous/new-api/router"
)
```

读 `main.go` 时先不要陷入每个函数细节，先画出启动主线：

```text
main()
  -> InitResources()
  -> 启动缓存/配置/任务 goroutine
  -> gin.New()
  -> 挂全局 middleware
  -> router.SetRouter()
  -> http.Server.ListenAndServe()
  -> signal graceful shutdown
```

重点观察 `InitResources()` 的顺序。比如必须先 `model.InitDB()`，才能 `model.InitOptionMap()` 从数据库读配置；必须先初始化 `ratio_setting`，默认模型倍率才存在。

### 小练习

在本地画一张启动顺序图，标出：

- 哪些步骤只执行一次。
- 哪些步骤启动后台 goroutine。
- 哪些步骤依赖数据库。
- 哪些步骤只在 master 节点执行。

## 二、函数、方法、指针和 error

### 要学什么

- 普通函数和方法的区别。
- 指针接收者什么时候用于修改对象。
- Go 的错误返回模式。
- 多返回值如何表达“值 + 是否存在 + 错误”。

### 源码入口

- `model/token.go`
- `model/channel.go`
- `service/channel_select.go`

### 读法

先看 `model.Token` 的方法：

```go
func (token *Token) Clean() {
    token.Key = ""
}
```

这里用指针接收者，因为要修改 token 对象本身。

再看 `ValidateUserToken()`：

```go
func ValidateUserToken(key string) (token *Token, err error)
```

这是典型 Go 风格：

- 返回业务对象。
- 返回 `error`。
- 调用方必须判断错误。

它的阅读重点不是每个 if，而是判断顺序：

1. key 是否为空。
2. token 是否存在。
3. token 状态是否启用。
4. token 是否过期。
5. token 额度是否足够。

再看 `service.RetryParam`：

```go
func (p *RetryParam) GetRetry() int
func (p *RetryParam) IncreaseRetry()
func (p *RetryParam) ResetRetryNextTry()
```

这些方法把 retry 计数、跳过下一次递增等细节包在类型内部。学习点是：当一组字段总是一起变化时，Go 代码通常用 struct + 方法来维护不变量。

### 小练习

给自己解释清楚这三个问题：

1. `Token.Clean()` 为什么不是值接收者？
2. `ValidateUserToken()` 为什么返回 `(*Token, error)`，而不是 panic？
3. `RetryParam.Retry` 为什么是 `*int`，代码如何处理 nil？

## 三、struct、tag 和数据模型

### 要学什么

- struct 字段如何对应 JSON 和数据库列。
- tag 的含义：`json:"..."`、`gorm:"..."`、`validate:"..."`。
- 指针字段和零值字段的区别。
- 运行时字段如何用 `gorm:"-"` 排除持久化。

### 源码入口

- `model/user.go`
- `model/token.go`
- `model/channel.go`
- `dto/openai_request.go`
- `dto/channel_settings.go`

### 读法

看 `model.User`：

```go
type User struct {
    Id          int    `json:"id"`
    Username    string `json:"username" gorm:"unique;index" validate:"max=20"`
    Password    string `json:"password" gorm:"not null;" validate:"min=8,max=20"`
    AccessToken *string `json:"-" gorm:"type:char(32);column:access_token;uniqueIndex"`
}
```

这里同时承载三层语义：

- `json`：返回给前端或 API 时如何命名；`json:"-"` 表示不输出。
- `gorm`：数据库索引、列类型、默认值。
- `validate`：参数校验规则。

看 `model.Channel` 时特别注意指针字段：

```go
BaseURL *string `json:"base_url" gorm:"column:base_url;default:''"`
Weight  *uint   `json:"weight" gorm:"default:0"`
```

指针字段可以区分“没有传”和“传了零值”。这个思想在 relay 请求 DTO 中更重要。项目规则要求：从客户端 JSON 解析并重新 marshal 到上游的可选标量字段，应使用指针 + `omitempty`，避免显式 `0`、`false` 被误删。

### 小练习

任选一个 model struct，给每个字段标注：

- 是否会入库。
- 是否会返回 JSON。
- 是否允许为空。
- 是否需要跨 SQLite/MySQL/PostgreSQL 兼容。

## 四、interface：provider adaptor 的核心抽象

### 要学什么

- interface 定义行为，不关心具体类型。
- 不同 provider 通过同一接口被主流程调用。
- 工厂函数如何根据类型返回不同实现。

### 源码入口

- `relay/channel/adapter.go`
- `relay/relay_adaptor.go`
- `relay/compatible_handler.go`
- `relay/channel/openai/adaptor.go`
- `relay/channel/claude/adaptor.go`

### 读法

先读接口：

```go
type Adaptor interface {
    Init(info *relaycommon.RelayInfo)
    GetRequestURL(info *relaycommon.RelayInfo) (string, error)
    SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
    ConvertOpenAIRequest(...)
    DoRequest(...)
    DoResponse(...)
}
```

这相当于 new-api 对“上游 provider”的协议约束：只要你能初始化、构造 URL/header、转换请求、发请求、处理响应，就能接入主 relay 流程。

再看 `relay.GetAdaptor(apiType)`：

```go
func GetAdaptor(apiType int) channel.Adaptor {
    switch apiType {
    case constant.APITypeOpenAI:
        return &openai.Adaptor{}
    case constant.APITypeAnthropic:
        return &claude.Adaptor{}
    }
    return nil
}
```

这就是简单工厂模式。主流程不需要知道 provider 的内部细节，只拿到 `channel.Adaptor` 接口。

最后看 `TextHelper()` 怎么使用：

```text
adaptor := GetAdaptor(info.ApiType)
adaptor.Init(info)
convertedRequest, err := adaptor.ConvertOpenAIRequest(...)
resp, err := adaptor.DoRequest(...)
usage, newApiErr := adaptor.DoResponse(...)
```

这里是 Go interface 的典型价值：主流程稳定，provider 差异放到实现里。

### 小练习

把 OpenAI 和 Claude adaptor 对比一遍：

- URL 怎么构造。
- header 怎么设置。
- OpenAI request 到上游格式怎么转换。
- response 怎么写回下游。

## 五、Gin middleware、闭包和 context

### 要学什么

- Gin 中间件的函数类型。
- 闭包如何返回 handler。
- `gin.Context` 和标准库 `context.Context` 的区别。
- request-scoped value 如何贯穿调用链。

### 源码入口

- `middleware/request-id.go`
- `middleware/auth.go`
- `middleware/distributor.go`
- `constant/context_key.go`

### 读法

看最小中间件 `RequestId()`：

```go
func RequestId() func(c *gin.Context) {
    return func(c *gin.Context) {
        id := common.NewRequestId()
        c.Set(common.RequestIdKey, id)
        ctx := context.WithValue(c.Request.Context(), common.RequestIdKey, id)
        c.Request = c.Request.WithContext(ctx)
        c.Header(common.RequestIdKey, id)
        c.Next()
    }
}
```

学习点：

- 外层函数返回内层函数，这就是闭包式 middleware 写法。
- `c.Set()` 写 Gin context。
- `context.WithValue()` 写标准库 request context。
- `c.Next()` 表示继续执行后续 middleware/handler。

再看 `TokenAuth()` 和 `Distribute()`：

- `TokenAuth()` 负责把用户、token、分组写入 context。
- `Distribute()` 读取这些 context，再写入渠道信息。
- `controller.Relay()` 从 context 生成 `RelayInfo`。

这就是 new-api 中最重要的 request-scoped 状态传递方式。

### 小练习

沿着一个字段追踪，比如 `channel_id`：

1. 它在哪里写入 context？
2. `RelayInfo` 在哪里读它？
3. 日志里在哪里使用它？

## 六、错误设计：从 error 到 API 响应

### 要学什么

- Go 中错误是值，不是异常。
- 项目如何把内部 error 包装成统一 API error。
- defer 如何统一处理错误响应。
- 错误码、HTTP 状态码、是否重试如何解耦。

### 源码入口

- `types/error.go`
- `service/error.go`
- `controller/relay.go`
- `relay/channel/openai/relay-openai.go`

### 读法

从 `controller.Relay()` 的 defer 开始读：

```text
如果 newAPIError != nil:
  -> log
  -> request id 拼入 message
  -> 根据 RelayFormat 输出 OpenAI 或 Claude 错误格式
```

这是一种常见 Go 写法：主流程中只给 `newAPIError` 赋值并 return，最终响应集中在 defer 中完成。

再看 `service.RelayErrorHandler()`：

- 读取上游非 200 响应。
- 尝试解析上游错误 body。
- 转成项目统一的 `NewAPIError`。

再看 `shouldRetry()`：

- 指定渠道不重试。
- skip retry 错误不重试。
- 429/5xx 按 operation setting 判断。
- 模型不可用、渠道错误等会触发换渠道。

### 小练习

选择一个错误场景，例如上游 429：

1. 上游响应在哪里被识别为错误？
2. 状态码映射在哪里发生？
3. 什么时候会重试？
4. 最终返回给客户端是什么格式？

## 七、map、slice、泛型和并发安全

### 要学什么

- map 和 slice 在业务中的常见用法。
- 泛型类型参数的写法。
- `sync.RWMutex` 保护共享 map。
- 返回 map copy 避免调用方修改内部状态。

### 源码入口

- `types/rw_map.go`
- `setting/ratio_setting/`
- `model/channel_cache.go`
- `service/channel_select.go`

### 读法

`types.RWMap` 是非常适合学习 Go 泛型的文件：

```go
type RWMap[K comparable, V any] struct {
    data  map[K]V
    mutex sync.RWMutex
}
```

这里：

- `K comparable` 表示 key 必须可比较，因为 map key 有这个要求。
- `V any` 表示值可以是任意类型。
- `sync.RWMutex` 允许多读单写。

读 `ReadAll()`：

```go
func (m *RWMap[K, V]) ReadAll() map[K]V {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    copiedMap := make(map[K]V)
    for k, v := range m.data {
        copiedMap[k] = v
    }
    return copiedMap
}
```

重点是它返回 copy，而不是直接返回 `m.data`。这是并发安全设计：内部状态不能被调用方绕过锁直接修改。

### 小练习

在 `setting/ratio_setting` 中找一个使用 `RWMap` 的配置，回答：

- key 是什么类型？
- value 是什么类型？
- 配置如何从 JSON 字符串加载？
- 读取时是否返回 copy？

## 八、goroutine、channel、defer、recover

### 要学什么

- goroutine 用于后台任务。
- defer 用于资源释放、统一收尾和 recover。
- channel send 到关闭 channel 会 panic。
- recover 只能在 defer 中捕获 panic。

### 源码入口

- `main.go`
- `service/system_task.go`
- `common/go-channel.go`
- `controller/relay.go`

### 读法

`main.go` 中有很多后台任务：

```go
go model.SyncOptions(common.SyncFrequency)
go authz.StartPolicySync(common.SyncFrequency)
go model.UpdateQuotaData()
```

这些 goroutine 和 HTTP server 同时运行，负责周期性维护系统状态。

`common/go-channel.go` 展示了 defer + recover：

```go
func SafeSendBool(ch chan bool, value bool) (closed bool) {
    defer func() {
        if recover() != nil {
            closed = true
        }
    }()
    ch <- value
    return false
}
```

向已关闭 channel 发送会 panic。这里用 recover 把 panic 转成布尔返回值。

`service/system_task.go` 展示了更复杂的并发模型：

- runner ticker 定期扫描任务。
- claim 成功后每个任务一个 goroutine。
- heartbeat goroutine 延长 lease。
- context cancel 通知任务停止。

### 小练习

读 `runWithLeaseHeartbeat()`，画出三个事件：

- 任务正常完成。
- heartbeat 失败。
- context 被 cancel。

## 九、JSON、io.Reader 和请求体复用

### 要学什么

- 项目要求使用 `common.Marshal` / `common.Unmarshal` 包装 JSON。
- `io.Reader` 是 Go 中流式读取抽象。
- HTTP request body 默认只能读一次。
- 如何通过 body storage 支持中间件和 handler 多次读取。

### 源码入口

- `common/json.go`
- `common/body_storage.go`
- `common/gin.go`
- `middleware/body_cleanup.go`
- `relay/compatible_handler.go`
- `relay/common/outbound_body.go`

### 读法

先看 `common/json.go`：

```go
func Unmarshal(data []byte, v any) error {
    return json.Unmarshal(data, v)
}

func Marshal(v any) ([]byte, error) {
    return json.Marshal(v)
}
```

虽然现在只是薄封装，但项目规则要求业务代码通过这些 wrapper 调用 JSON。这样未来替换 JSON 实现或统一行为时不需要全局改业务代码。

再看 relay 请求体：

```text
Distribute 需要读 body 提取 model
controller.Relay 需要读 body 解析 DTO
retry 时需要重新发同一个 body
pass-through 时需要把原始 body 发上游
```

所以项目不能直接 `io.ReadAll(c.Request.Body)` 后丢掉，而是使用 `common.GetBodyStorage(c)` 提供可 seek、可复用的 body storage。

`relay/compatible_handler.go` 最终把转换后的 JSON 包成上游请求体：

```text
common.Marshal(convertedRequest)
-> relaycommon.NewOutboundJSONBody(jsonData)
-> adaptor.DoRequest(..., requestBody)
```

### 小练习

找一次 `common.UnmarshalBodyReusable()` 的调用，回答：

- 调用前谁可能已经读过 body？
- 调用后 body 是否还能被下一层读取？
- 如果请求体很大，项目如何避免一直占内存？

## 十、GORM、迁移和跨数据库兼容

### 要学什么

- GORM model 和 AutoMigrate。
- 查询、更新、事务基本写法。
- SQLite/MySQL/PostgreSQL 差异。
- raw SQL 必须小心方言。

### 源码入口

- `model/main.go`
- `model/user.go`
- `model/token.go`
- `model/channel.go`
- `model/log.go`
- `common/database.go`

### 读法

从 `model.InitDB()` 看数据库选择：

```text
SQL_DSN empty/local -> SQLite
postgres:// 或 postgresql:// -> PostgreSQL
其他 -> MySQL
```

`migrateDB()` 里用 `DB.AutoMigrate()` 迁移大量 model。学习 GORM 时先看简单查询：

```go
DB.Where("user_id = ?", userId).Order("id desc").Limit(num).Offset(startIdx).Find(&tokens)
```

再看兼容性变量：

```go
if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
    commonGroupCol = `"group"`
    commonKeyCol = `"key"`
} else {
    commonGroupCol = "`group`"
    commonKeyCol = "`key`"
}
```

因为 `group`、`key` 是保留字，不同数据库引用方式不同。

### 小练习

找一个使用 `commonGroupCol` 的查询，说明：

- 如果直接写 `group = ?` 可能在哪些库出问题？
- 为什么 GORM struct tag 不能解决所有 raw SQL 场景？

## 十一、HTTP 路由和 handler

### 要学什么

- Gin router group。
- 中间件组合顺序。
- handler 如何读参数、调用业务、返回 JSON。
- relay API 和后台 API 的差别。

### 源码入口

- `router/main.go`
- `router/api-router.go`
- `router/relay-router.go`
- `controller/user.go`
- `controller/relay.go`

### 读法

看 `router.SetRouter()`：

```go
SetApiRouter(router)
SetDashboardRouter(router)
SetRelayRouter(router)
SetVideoRouter(router)
SetWebRouter(router, assets)
```

后台 API 通常返回统一业务 JSON：

```json
{ "success": true, "message": "", "data": ... }
```

relay API 则要模拟 OpenAI/Claude/Gemini 等协议，错误格式也要兼容客户端 SDK。

对比：

- `/api/user/self`：面向控制台，session 用户认证。
- `/v1/chat/completions`：面向 API 客户端，API key 认证，返回 OpenAI 兼容结果。

### 小练习

任选一个 `/api` 路由和一个 `/v1` 路由，分别写出：

- 中间件列表。
- 认证方式。
- handler 函数。
- 响应格式。

## 十二、读懂 relay 主链路

### 要学什么

- 如何把前面学到的 Go 语法串成业务调用链。
- 如何识别主线和分支。
- 如何读一个大函数而不迷路。

### 源码入口

- `controller/relay.go`
- `relay/compatible_handler.go`
- `relay/channel/openai/adaptor.go`
- `relay/channel/openai/relay-openai.go`
- `service/text_quota.go`

### 读法

建议先只读 OpenAI 非流式 chat completions：

```text
POST /v1/chat/completions
  -> TokenAuth
  -> Distribute
  -> controller.Relay
  -> relay.TextHelper
  -> openai.Adaptor
  -> OpenaiHandler
  -> PostTextConsumeQuota
```

每一层只问三个问题：

1. 输入是什么？
2. 输出是什么？
3. 它把什么状态写进 context 或 RelayInfo？

等非流式读通，再读流式：

- `info.IsStream` 如何设置。
- `StreamOptions` 如何处理。
- `OaiStreamHandler()` 如何扫描 SSE。
- 如果上游没返回 usage，项目如何估算。

### 小练习

写一份自己的“十步 relay 流程”，每步只写一个函数名和一句话。然后和 `docs/architecture/new-api-implementation-guide.md` 的关键流程速查对照。

## 十三、计费代码作为 Go 综合练习

### 要学什么

- 业务规则如何拆成多个类型和函数。
- 预扣费、后结算、退款如何保持状态一致。
- interface/struct/method/error/defer 如何一起使用。
- 小数、四舍五入、表达式引擎如何封装。

### 源码入口

- `relay/helper/price.go`
- `service/billing.go`
- `service/billing_session.go`
- `service/quota.go`
- `service/text_quota.go`
- `service/tiered_settle.go`
- `pkg/billingexpr/`

### 读法

先读普通计费：

```text
ModelPriceHelper
  -> PriceData
  -> PreConsumeBilling
  -> BillingSession.preConsume
  -> 上游请求
  -> PostTextConsumeQuota
  -> SettleBilling
```

再读 tiered expression：

```text
modelPriceHelperTiered
  -> RunExprWithRequest
  -> BillingSnapshot
  -> BuildTieredTokenParams
  -> TryTieredSettle
```

学习重点：

- `BillingSession` 用字段记录生命周期，防止重复结算或重复退款。
- `FundingSource` 把钱包和订阅差异隐藏起来。
- `TryTieredSettle()` 返回 `(ok bool, quota int, result *TieredResult)`，这是 Go 中常见的“是否适用 + 结果”模式。

### 小练习

模拟一个请求：

- 预扣 100。
- 实际消耗 80。
- 看 `SettleBilling()` 如何返还 20。

再模拟：

- 预扣 100。
- 实际消耗 150。
- 看它如何补扣 50。

## 十四、系统任务：并发和分布式思维

### 要学什么

- 后台 worker 如何设计。
- ticker、goroutine、context cancel 如何配合。
- DB lease 如何避免多实例重复执行。
- handler interface 如何让任务可扩展。

### 源码入口

- `service/system_task.go`
- `controller/system_task_handlers.go`
- `model/system_task.go`
- `service/task_polling.go`

### 读法

先读接口：

```go
type SystemTaskHandler interface {
    Type() string
    Run(ctx context.Context, task *model.SystemTask, runnerID string)
}
```

再读 runner：

```text
StartSystemTaskRunner
  -> ticker/wakeup
  -> runSystemTaskScheduler
  -> runSystemTaskClaimPass
  -> runWithLeaseHeartbeat
```

这里的 Go 学习点非常集中：

- interface 表示任务能力。
- map 保存 handler registry。
- goroutine 并发执行任务。
- context 取消任务。
- defer 保证 ticker stop、cancel。
- DB lock 负责跨进程协调。

### 小练习

用伪代码描述“新增一个定时任务”需要实现什么方法、在哪里注册、如何判断 Enabled。

## 十五、测试：读项目测试学 Go

### 要学什么

- Go 测试文件命名。
- table-driven tests。
- `require` 和 `assert` 的区别。
- 测试应保护行为，而不是锁死实现细节。

### 源码入口

- `dto/openai_request_zero_value_test.go`
- `pkg/billingexpr/billingexpr_test.go`
- `service/tiered_settle_test.go`
- `controller/relay_retry_test.go`
- `model/channel_cache_test.go`

### 读法

先找 table test：

```go
tests := []struct {
    name string
    input ...
    want ...
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        ...
    })
}
```

这个模式适合学习：

- 如何把输入和期望显式列出来。
- 如何用 `t.Run` 分子场景。
- 如何避免复制粘贴测试函数。

项目规则要求新 Go 测试：

- setup 和 fatal 断言用 `require`。
- 非 fatal 值检查用 `assert`。
- 不写只为覆盖率服务的 smoke test。

### 小练习

找一个你理解的纯函数，为它设计 table test，不一定立刻写代码。先列出：

- 正常输入。
- 边界输入。
- 错误输入。
- 期望输出或期望错误。

## 十六、按功能读源码的路线

### 1. 想理解用户登录

阅读顺序：

1. `router/api-router.go` 中 `/api/user/login`。
2. `controller/user.go` 的 `Login()`。
3. `model/user.go` 的用户查询和密码校验。
4. `middleware/auth.go` 的 `UserAuth()`。
5. 前端 `web/default/src/features/auth/`。
6. 前端 `web/default/src/stores/auth-store.ts`。

重点学习：

- Gin handler。
- session。
- struct JSON 输出。
- 前后端 session + localStorage 协作。

### 2. 想理解 API key 调用

阅读顺序：

1. `router/relay-router.go`。
2. `middleware/auth.go` 的 `TokenAuth()`。
3. `middleware/distributor.go`。
4. `controller/relay.go`。
5. `relay/compatible_handler.go`。
6. `relay/channel/openai/`。

重点学习：

- middleware 链。
- context 状态传递。
- interface adaptor。
- error 和 retry。

### 3. 想理解渠道管理

阅读顺序：

1. `router/channel-router.go`。
2. `controller/channel.go`。
3. `model/channel.go`。
4. `model/channel_cache.go`。
5. `service/channel_select.go`。
6. 前端 `web/default/src/features/channels/`。

重点学习：

- GORM model。
- 缓存构建。
- map/slice。
- 权限中间件。

### 4. 想理解计费

阅读顺序：

1. `relay/helper/price.go`。
2. `service/billing_session.go`。
3. `service/billing.go`。
4. `service/text_quota.go`。
5. `service/quota.go`。
6. `pkg/billingexpr/expr.md`。
7. `pkg/billingexpr/`。

重点学习：

- 业务状态机。
- interface 和实现。
- decimal/rounding。
- error 回滚。

### 5. 想理解异步任务

阅读顺序：

1. `router/video-router.go`、`router/relay-router.go` 的 `/suno` 和 `/mj`。
2. `relay/relay_task.go`。
3. `relay/channel/task/`。
4. `model/task.go`。
5. `service/task_polling.go`。
6. `service/system_task.go`。

重点学习：

- 任务状态。
- 轮询。
- goroutine。
- context cancellation。
- 退款和终态结算。

## 十七、读源码时的 Go 思维清单

每读一个函数，按这张清单问自己：

1. 这个函数属于哪一层：router、middleware、controller、service、model、relay？
2. 输入参数是值、指针、interface，还是 context？
3. 返回值是否有 error？调用方如何处理？
4. 是否修改了传入对象？
5. 是否写了 Gin context 或标准 context？
6. 是否启动 goroutine？
7. 是否需要 defer 释放资源？
8. 是否涉及数据库事务？
9. 是否需要跨数据库兼容？
10. 是否影响计费、日志、权限或重试？

## 十八、建议练习项目

这些练习不要求立即提交代码，可以先做读源码笔记。

### 练习 1：给一个请求画调用链

选择 `/v1/chat/completions`，画出从路由到日志的完整链路。要求每一层写出一个关键函数。

### 练习 2：解释一个 struct 的 tag

选择 `model.Channel` 或 `model.Token`，解释每个 tag 的作用。

### 练习 3：新增一个只读后台 API 的设计草稿

不要写代码，先设计：

- 路由放在哪里。
- 用哪个认证中间件。
- controller 函数叫什么。
- service 是否需要新函数。
- model 查询怎么写。
- 前端 feature API 怎么调用。

### 练习 4：读一个测试并改写成表驱动说明

选择一个 `*_test.go`，把每个 case 用自然语言解释出来。

### 练习 5：追踪一个错误

选择“上游 500”或“token 额度不足”，追踪：

- error 在哪里创建。
- 有没有包装成 `NewAPIError`。
- 是否重试。
- 是否记录日志。
- 是否退款。
- 下游看到什么响应。

## 十九、源码阅读节奏

推荐每天按这个节奏推进：

1. 读 30 分钟源码，只读一条链路。
2. 写 10 分钟笔记，记录函数名和职责。
3. 回头总结 1 个 Go 语言点。
4. 找 1 个测试验证理解。
5. 第二天从昨天的调用链继续往下一层读。

不要试图一次读完整个项目。new-api 的复杂度来自“多个稳定小机制叠在一起”：认证、分发、转换、计费、日志、任务。每次只吃下一条链路，很快就会连成网。

