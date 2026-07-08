# new-api 测试与调试学习指南

这份文档面向已经掌握 Go 基本语法、正在通过 new-api 学习真实项目源码的读者。目标不是把所有测试文件逐行解释一遍，而是教你看懂这个项目如何保护关键行为、如何搭本地调试环境、如何在一次请求失败时顺着日志和断点找到原因。

new-api 是一个 AI API 网关，最容易出问题的地方集中在几类链路：

- 请求进入 Gin 后，中间件是否正确放行、拒绝或写入上下文。
- relay 请求是否正确选择渠道、构造上游请求、处理流式响应。
- 计费是否正确预扣、结算、退款和记录日志。
- 后台任务是否能防重、抢占锁、续租、失败恢复。
- DTO 是否能保留用户显式传入的 `0`、`false` 等值。
- 前端构建和类型检查是否能在默认 UI 中提前暴露问题。

读测试和调试代码时，可以把它们看成“源码设计的反向说明书”：生产代码告诉你系统怎么写，测试告诉你哪些行为绝不能变。

## 一、先掌握怎么跑

### 1. 后端测试入口

Go 后端测试分布在各个 package 的 `*_test.go` 文件里，可以从仓库根目录运行：

```bash
go test ./...
```

如果只想跑某个 package：

```bash
go test ./service
go test ./relay/helper
go test ./model
```

如果只想跑某个测试函数：

```bash
go test ./service -run TestTryTieredSettle
go test ./relay/helper -run TestStreamScannerHandler
go test ./model -run TestSystemTask
```

Go 的 `-run` 参数是正则匹配。比如 `TestStreamScannerHandler` 会匹配所有函数名包含这个片段的测试。

调试某个失败测试时，建议加上：

```bash
go test ./relay/helper -run TestStreamScannerHandler_Timeout -v
```

`-v` 会显示每个子测试和日志输出，适合学习测试执行顺序。

### 2. 前端检查入口

默认前端在 `web/default/`，包管理器按项目约定优先使用 Bun：

```bash
cd web/default
bun run dev
bun run build
bun run build:check
bun run typecheck
bun run lint
bun run i18n:sync
```

这些命令来自 `web/default/package.json`：

- `dev`: 启动 Rsbuild 开发服务器。
- `build`: 生产构建。
- `build:check`: 先跑 `tsgo -b`，再构建。
- `typecheck`: TypeScript 项目检查。
- `lint`: 使用 oxlint 检查。
- `format:check`: 检查格式并保护头部版权信息。
- `copyright:check`: 检查版权头。
- `i18n:sync`: 同步默认前端 i18n key。

当前后端文档学习阶段通常不需要跑前端命令；但如果你改了 `web/default/src`，至少应考虑 `bun run typecheck` 或 `bun run build:check`。

## 二、本地调试环境

本地调试说明记录在 `docs/local-development.md`。对 GoLand 或命令行调试来说，最重要的是这三个环境变量：

```text
DEBUG=true
GIN_MODE=debug
LOG_CALLER_ENABLED=true
```

它们分别对应：

- `DEBUG=true`: 开启 `common.DebugEnabled`，让 `logger.LogDebug(...)` 真的输出。
- `GIN_MODE=debug`: 让 Gin 保持 debug 模式。
- `LOG_CALLER_ENABLED=true`: 日志里带上 `file.go:line`，方便从日志跳回源码。

注意：`LOG_LEVEL=debug` 当前不会开启 debug 日志。项目里真正判断 debug 的变量是 `common.DebugEnabled`，它在 `common/init.go` 读取 `DEBUG` 环境变量。

### 1. 启动时 debug 的读取位置

启动入口在 `main.go`。你可以按这个顺序读：

1. `main.go`
2. `common/init.go`
3. `logger/logger.go`

关键关系是：

```text
DEBUG=true
  -> common.DebugEnabled = true
  -> logger.LogDebug(...) 输出日志
```

`logger.LogDebug(ctx, msg, args...)` 内部会检查 `common.DebugEnabled`。这意味着生产代码里可以到处调用 `LogDebug`，但是否输出由运行环境决定。

### 2. pprof

`main.go` 还会根据环境变量开启 pprof：

```text
ENABLE_PPROF=true
```

项目也有 `common/pprof.go`，用于 CPU 使用率超过阈值时输出 pprof 文件。学习时不需要一开始就掌握 pprof，但要知道它是定位性能问题的入口。

## 三、测试目录地图

先用一个命令看测试分布：

```bash
rg --files -g '*_test.go'
```

目前比较值得精读的测试文件包括：

| 文件 | 学习主题 |
| --- | --- |
| `common/json_test.go` | 项目 JSON wrapper 的行为 |
| `dto/openai_request_zero_value_test.go` | pointer + `omitempty` 如何保留显式零值 |
| `relay/helper/stream_scanner_test.go` | SSE 流式响应扫描、超时、PING、状态记录 |
| `relay/helper/openai_image_request_test.go` | multipart 请求、`httptest`、Gin test context |
| `relay/channel/api_request_test.go` | 上游 HTTP 请求、header 覆盖、SSE ping |
| `relay/common/relay_info_test.go` | `RelayInfo` 字段和计时行为 |
| `relay/common/override_test.go` | 参数覆盖逻辑 |
| `controller/relay_retry_test.go` | relay 重试判断 |
| `middleware/header_nav_test.go` | 中间件如何读写 header/context |
| `model/system_task_test.go` | 系统任务生命周期、锁、防重 |
| `model/task_cas_test.go` | DB fixture、CAS 更新、`TestMain` |
| `model/clickhouse_log_test.go` | 日志 SQL、request id 回填 |
| `service/text_quota_test.go` | 文本计费结算 |
| `service/tiered_settle_test.go` | tiered expression 计费不变量 |
| `pkg/billingexpr/billingexpr_test.go` | 表达式计费语言 |
| `service/channel_affinity_test.go` | 渠道亲和与选择策略 |
| `service/system_task_test.go` | 后台系统任务服务层 |
| `router/channel_router_test.go` | router 级别行为 |

学习顺序建议：

1. 先读纯逻辑测试：`common/json_test.go`、`dto/openai_request_zero_value_test.go`。
2. 再读 HTTP/Gin 测试：`relay/helper/openai_image_request_test.go`、`middleware/header_nav_test.go`。
3. 再读 relay 流式和上游请求：`relay/helper/stream_scanner_test.go`、`relay/channel/api_request_test.go`。
4. 再读数据库状态机：`model/system_task_test.go`、`model/task_cas_test.go`。
5. 最后读计费和表达式：`service/tiered_settle_test.go`、`service/text_quota_test.go`、`pkg/billingexpr/billingexpr_test.go`。

## 四、Go 测试写法在本项目中的模式

### 1. `require` 和 `assert`

项目约定：新的或大幅改写的 Go 后端测试应使用：

- `require`: 用于 setup、前置条件、失败后不能继续的断言。
- `assert`: 用于非致命的值检查。

例子来自 `dto/openai_request_zero_value_test.go`：

```go
err := common.Unmarshal(raw, &req)
require.NoError(t, err)

encoded, err := common.Marshal(req)
require.NoError(t, err)

require.True(t, gjson.GetBytes(encoded, "stream").Exists())
```

这里反序列化失败后，后面的检查都没有意义，所以用 `require.NoError`。

### 2. 表驱动测试

Go 项目里常见的测试写法是“表驱动”：

```go
tests := []struct {
    name  string
    model string
    want  string
}{
    {name: "o1 uses developer", model: "o1", want: "developer"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        req := GeneralOpenAIRequest{Model: tt.model}
        require.Equal(t, tt.want, req.GetSystemRoleName())
    })
}
```

这类测试适合读“输入到输出”的规则，例如模型名到 system role 的映射。

### 3. `httptest` 和 Gin test context

HTTP 测试一般会组合：

- `httptest.NewRecorder()`
- `httptest.NewRequest(...)`
- `gin.CreateTestContext(recorder)`
- `gin.SetMode(gin.TestMode)`

`relay/helper/stream_scanner_test.go` 里有一个典型 helper：

```go
recorder := httptest.NewRecorder()
c, _ := gin.CreateTestContext(recorder)
c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
```

它创建了一个不需要真实网络端口的 Gin 请求上下文。你可以在这个上下文里设置 header、context key、请求体，然后调用 handler 或底层函数。

### 4. `t.Cleanup`

当测试会修改全局变量或数据库状态时，要在测试结束时恢复。比如流式测试会临时改 `constant.StreamingTimeout`：

```go
oldTimeout := constant.StreamingTimeout
constant.StreamingTimeout = 1
t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
```

这比 `defer` 更贴合测试语义，尤其在有子测试时更清晰。

### 5. `TestMain`

`model/task_cas_test.go` 使用 `TestMain(m *testing.M)` 初始化测试数据库：

```go
func TestMain(m *testing.M) {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    ...
    os.Exit(m.Run())
}
```

`TestMain` 是整个 package 测试运行前的入口。它适合做一次性的重型初始化，比如：

- 打开内存 SQLite。
- 设置全局 `model.DB`、`model.LOG_DB`。
- 关闭 Redis 和批量更新。
- 跑 `AutoMigrate`。

读这类测试时要意识到：测试函数本身没有显式打开数据库，是因为 package 级 `TestMain` 已经做了。

### 6. 并发和原子变量

流式测试里会出现：

- `sync.Mutex`
- `sync/atomic`
- `io.Pipe`
- goroutine
- `select` + `time.After`

例如 `TestStreamScannerHandler_PingSentDuringSlowUpstream` 用 `io.Pipe` 模拟一个慢速上游，goroutine 一边写 chunk，一边 sleep。测试主 goroutine 等待 scanner 结束，并检查响应体里是否写出了 `: PING`。

这类测试适合学习 Go 并发，但也要注意项目测试规范：不要为了“看起来压力大”写随机、sleep-heavy、日志-only 的测试。只有当超时、并发、流式协议本身就是业务行为时，才值得这样测。

## 五、代表性测试精读

### 1. DTO 零值保留

文件：`dto/openai_request_zero_value_test.go`

这个测试保护的是 relay 请求 DTO 的一个关键规则：用户显式传入的 `0`、`false` 不能在转发给上游时丢失。

背景是 Go 的 `omitempty`：

```go
MaxTokens int `json:"max_tokens,omitempty"`
```

如果字段是非 pointer，`0` 会被视为 empty，被 marshal 时省略。对 API 网关来说，这会改变用户请求语义。

所以可选 scalar 应该写成 pointer：

```go
MaxTokens *int `json:"max_tokens,omitempty"`
Stream    *bool `json:"stream,omitempty"`
```

这样：

- 客户端没传字段 -> `nil` -> marshal 时省略。
- 客户端传了 `0` -> `*int(0)` -> marshal 时保留。
- 客户端传了 `false` -> `*bool(false)` -> marshal 时保留。

测试流程是：

```text
原始 JSON 含 0/false
  -> common.Unmarshal 到 DTO
  -> common.Marshal 回 JSON
  -> gjson 检查字段仍然 Exists()
```

这里也体现了项目 JSON 规则：业务代码不要直接调用 `encoding/json.Marshal` 和 `encoding/json.Unmarshal`，而是通过 `common.Marshal`、`common.Unmarshal`。

### 2. SSE 流扫描

文件：`relay/helper/stream_scanner_test.go`

这个文件是学习流式 relay 的好入口。它测试 `StreamScannerHandler` 如何处理上游 SSE：

```text
上游响应 body
  -> scanner 按行读取
  -> 只处理 data: 行
  -> trim 掉 data: 前缀和空白
  -> 遇到 [DONE] 停止
  -> 调用 handler(data, streamResult)
  -> 记录 RelayInfo.StreamStatus
```

重点测试点：

- nil 输入不能 panic。
- 空 body 不调用 handler。
- 1000 个 chunk 能按顺序处理。
- `[DONE]` 后面的数据不能再处理。
- handler 调用 `sr.Stop(err)` 后停止。
- 非 `data:` 行要跳过。
- 慢速上游期间可以输出 ping。
- `DisablePing=true` 时不输出 ping。
- EOF without `[DONE]` 和正常 `[DONE]` 的 stream status 不同。
- timeout 会记录 `StreamEndReasonTimeout`。
- 每个 chunk 的 soft error 会累计到 `StreamStatus`。

这个测试文件能帮你理解三个 Go 技术点：

1. `bufio.Scanner` 如何按行读取。
2. goroutine 如何模拟慢速输入和异步完成。
3. `sync/atomic` 如何在线程安全地计数。

### 3. relay 重试判断

文件：`controller/relay_retry_test.go`

这个文件很短，但它保护的是高风险行为。relay 失败时是否重试，会影响：

- 用户看到的是失败还是成功 fallback。
- 某个渠道是否被误判为不可用。
- 请求是否造成重复扣费或重复上游调用。

`TestShouldRetryDoesNotTreatClearedRateLimitFlagAsUnavailable` 检查一个细节：

```go
c.Set(channelRateLimitedContextKey, true)
c.Set(channelRateLimitedContextKey, false)
```

同一个 context key 先被设为 true，又被清为 false。测试确认“清掉后的限流标记”不会继续强制重试。

这个测试提醒你：Gin context 是请求链路里的共享状态，读写 context key 时必须明确语义，不能只判断 key 是否存在。

### 4. 系统任务生命周期

文件：`model/system_task_test.go`

系统任务是后台管理逻辑的重要状态机，比如日志清理、渠道测试、自动同步等。核心模型包括：

- `SystemTask`
- `SystemTaskLock`

典型生命周期：

```text
CreateSystemTask
  -> pending active task
  -> ClaimSystemTask
  -> running with lock
  -> FinishSystemTask
  -> succeeded/failed
  -> ActiveKey cleared
```

测试保护的行为包括：

- 同一类型同时只能有一个 active task。
- `ActiveKey` 能阻止重复 active run。
- 已经被一个 runner claim 的任务不能被另一个 runner 抢走。
- lock 过期后，旧任务会被标记失败。
- 清理过期 lock 后可以创建新的 active task。
- 能按类型查最早 pending 任务。
- 能查最新任务。

这些测试适合学习数据库状态机。读的时候要特别注意：

- 哪些字段表示业务状态。
- 哪些字段表示并发控制。
- 哪些函数必须在事务或锁语义下保持一致。

### 5. Task CAS 更新

文件：`model/task_cas_test.go`

CAS 是 compare-and-swap 的意思。`Task.UpdateWithStatus(expectedStatus)` 的语义大致是：

```text
只有数据库里的任务状态仍等于 expectedStatus 时，才允许本次更新成功。
```

这个模式常用于异步任务，避免两个 worker 同时把一个任务写成不同结果。

这个文件还展示了数据库测试 fixture：

- `TestMain` 初始化内存 SQLite。
- `truncateTables(t)` 在测试 cleanup 阶段清表。
- `insertTask(t, task)` 统一插入任务。
- 测试里显式构造任务状态和数据。

对 Go 初学者来说，这是学习“真实项目怎么写数据库测试”的好样本。

### 6. tiered expression 计费结算

文件：`service/tiered_settle_test.go`

这个测试保护动态计费表达式。核心函数是 `TryTieredSettle`。

代表性表达式：

```text
p <= 200000 ? tier("standard", p * 1.5 + c * 7.5) : tier("long_context", p * 3 + c * 11.25)
```

这里：

- `p`: prompt tokens。
- `c`: completion tokens。
- `tier(...)`: 记录命中的价格档位。
- `param("service_tier")`: 从请求体里读取参数影响计费。

测试覆盖：

- 结算使用请求时冻结的 input，而不是后来变化的请求体。
- 表达式错误时 fallback 到预扣费。
- 预扣和后结算一致。
- 实际用量高于预估时补扣。
- 实际用量低于预估时退款。
- 档位边界，比如 `p == 200000` 和 `p == 200001`。
- cache tokens 会影响费用。
- request probe 会影响匹配档位。

如果你要改 tiered/dynamic billing，必须先读 `pkg/billingexpr/expr.md`。这份测试只是行为入口，完整设计在表达式文档里。

## 六、请求调试链路

调试 new-api 最重要的技能是：从一个 URL 找到 router，再沿中间件、controller、service、model 或 relay 往下走。

### 1. 登录链路

典型入口：

```text
POST /api/user/login
```

阅读顺序：

```text
router/api-router.go
  -> controller.Login
  -> model.User.ValidateAndFill
  -> setupLogin
  -> session / access token / response
```

建议断点：

- `controller/user.go` 的 `Login`
- `model.User.ValidateAndFill`
- `controller/user.go` 的 `setupLogin`

你要观察：

- Gin 如何绑定 JSON。
- 用户名、密码如何校验。
- 禁用用户如何被拒绝。
- 登录成功后 cookie/session/token 如何写回。
- response 通过哪个 helper 返回。

### 2. API Key relay 链路

典型入口：

```text
POST /v1/chat/completions
```

阅读顺序：

```text
router/relay-router.go
  -> middleware.TokenAuth()
  -> middleware.Distribute()
  -> controller.Relay
  -> relay.TextHelper
  -> provider adaptor
  -> relay/channel/api_request.go
  -> upstream response handler
  -> service.PostConsumeQuota / model.RecordConsumeLog
```

建议断点：

- `middleware/auth.go` 的 `TokenAuth`
- `middleware/distributor.go` 的 `Distribute`
- `controller/relay.go` 的 `Relay`
- `relay/compatible_handler.go` 的 `TextHelper`
- `relay/common/relay_info.go` 的 `GenRelayInfo`
- `relay/channel/api_request.go` 的上游请求发送函数
- `service/text_quota.go` 的结算逻辑
- `model/log.go` 的 `RecordConsumeLog`

你要观察：

- token 如何从 Authorization header 解析。
- token 对应用户如何加载。
- 请求模型如何映射到渠道能力。
- 渠道选择结果如何写入 Gin context。
- `RelayInfo` 如何承载 user、token、channel、model、billing 等信息。
- 上游响应 usage 如何转换成 quota。
- 成功和失败分别如何记录日志。

### 3. request id 调试

每个请求会通过 `middleware/request-id.go` 生成 request id：

```text
common.RequestIdKey = "X-Oneapi-Request-Id"
```

它会：

- 写入 Gin context。
- 写入 request context。
- 写入响应 header。

relay 错误返回时，`controller/relay.go` 会把 request id 拼进错误消息，方便用户拿错误来查日志。

日志模型 `model.Log` 也有：

- `RequestId`
- `UpstreamRequestId`

`model.RecordErrorLog` 和 `model.RecordConsumeLog` 会从 Gin context 里读取这些值。上游 HTTP 响应如果带 request id，`relay/channel/api_request.go` 会写入 `common.UpstreamRequestIdKey`。

所以线上排查时，优先拿：

```text
X-Oneapi-Request-Id
X-Upstream-Request-Id
```

去查日志和上游记录。

## 七、RelayInfo timing

`RelayInfo` 是 relay 链路的调试中枢之一。它在 `relay/common/relay_info.go` 里定义。

`controller/relay.go` 和 `relay/responses_handler.go` 会多次调用：

```go
relayInfo.MarkTiming("stage_name")
```

常见 stage 包括：

- `relay_info_ready`
- `token_meta_ready`
- `sensitive_check_done`
- `estimate_tokens_done`
- `price_ready`
- `preconsume_done`
- `retry_0_channel_ready`
- `retry_0_body_ready`
- `upstream_client_do_start`
- `upstream_client_do_done`
- `upstream_first_response_byte`
- `first_response`

这套 timing 能回答：

- 慢在鉴权、渠道选择、预扣费，还是上游网络。
- DNS、TCP connect、TLS、首字节分别耗时多少。
- retry 前后是否换了渠道。

读源码时可以把 `MarkTiming` 当成“调试路标”。如果你不知道请求走到了哪里，就搜 stage 名。

## 八、失败排查方法

### 1. 用户说“请求失败”

先问或查：

- HTTP status code。
- response body 里的错误码和 request id。
- 是否流式请求。
- model、token、group、channel。

源码定位顺序：

```text
controller/relay.go
  -> shouldRetry
  -> relay helper returned error
  -> types.NewAPIError
  -> model.RecordErrorLog
```

重点看错误是否带这些选项：

- `ErrOptionWithSkipRetry`
- `ErrOptionWithNoRecordErrorLog`
- `ErrOptionWithNoRecordErrorLog`

这些 option 会影响是否重试、是否记录错误日志。

### 2. 用户说“扣费不对”

定位顺序：

```text
relay/helper/price.go
  -> service/pre_consume_quota.go
  -> service/text_quota.go
  -> service/tiered_settle.go
  -> model/log.go
```

重点字段：

- `RelayInfo.FinalPreConsumedQuota`
- `RelayInfo.TieredBillingSnapshot`
- `BillingRequestInput`
- usage tokens
- group ratio
- model ratio
- `RecordConsumeLogParams`

如果是表达式计费，先读：

```text
pkg/billingexpr/expr.md
pkg/billingexpr/billingexpr_test.go
service/tiered_settle_test.go
```

### 3. 用户说“流式卡住”

定位顺序：

```text
relay/helper/stream_scanner.go
relay/helper/stream_scanner_test.go
relay/channel/api_request.go
relay/channel/<provider>/...
```

重点检查：

- `constant.StreamingTimeout`
- ping 设置是否开启。
- `RelayInfo.DisablePing`
- 上游是否真的返回 `data: [DONE]`。
- `StreamStatus.EndReason`
- 是否出现 handler stop 或 timeout。

### 4. 后台任务重复或卡住

定位顺序：

```text
model/system_task.go
service/system_task.go
model/system_task_test.go
service/system_task_test.go
```

重点检查：

- active task 是否存在。
- `ActiveKey` 是否清理。
- `SystemTaskLock` 是否过期。
- runner id 是否匹配。
- 任务状态是否 pending/running/succeeded/failed。

## 九、自己新增测试时的判断标准

不要为了覆盖率而写测试。这个项目更看重真实行为保护。

适合新增测试的场景：

- API contract 发生变化。
- relay 请求/响应转换有兼容风险。
- 计费、扣费、退款、日志有资金或账务风险。
- 数据库逻辑要同时兼容 SQLite、MySQL、PostgreSQL。
- 中间件鉴权、权限、限流、分发逻辑有安全风险。
- 新 provider 的 DTO 有可选字段、零值、流式协议差异。
- 修复了一个真实 bug，需要防回归。

不适合新增测试的场景：

- 只是证明函数能跑。
- 随机输入但没有明确不变量。
- 大循环、sleep、性能阈值，却没有稳定业务意义。
- 只断言私有 helper 的内部实现。
- 和已有测试覆盖同一分支，没有新增合同。

写测试时优先选择：

- 明确输入。
- 明确期望输出。
- 表驱动。
- `require` 做 setup。
- `assert` 做多项值检查。
- 明确初始化 DB、context、settings、cache。
- 用 `t.Cleanup` 恢复全局状态。

## 十、调试练习路线

### 练习 1：读懂零值保留

1. 打开 `dto/openai_request_zero_value_test.go`。
2. 找到 `GeneralOpenAIRequest` 定义。
3. 确认可选 scalar 是否是 pointer。
4. 修改脑内模型：为什么 `*bool(false)` 不会被 `omitempty` 丢掉。
5. 跑：

```bash
go test ./dto -run TestGeneralOpenAIRequestPreserveExplicitZeroValues -v
```

### 练习 2：跟一次流式扫描

1. 打开 `relay/helper/stream_scanner_test.go`。
2. 从 `buildSSEBody` 看测试输入。
3. 从 `StreamScannerHandler` 看生产逻辑。
4. 观察 `[DONE]`、EOF、timeout 的不同。
5. 跑：

```bash
go test ./relay/helper -run TestStreamScannerHandler_StreamStatus -v
```

### 练习 3：跟一次 API Key 请求

1. 在 `router/relay-router.go` 找 `/v1/chat/completions`。
2. 跳到 `middleware.TokenAuth()`。
3. 跳到 `middleware.Distribute()`。
4. 跳到 `controller.Relay`。
5. 跳到 `relay.TextHelper`。
6. 找 `RelayInfo.MarkTiming`。
7. 找 `RecordConsumeLog` 和 `RecordErrorLog`。

### 练习 4：读系统任务状态机

1. 先读 `model/system_task_test.go`。
2. 再读 `model/system_task.go`。
3. 画出 pending/running/succeeded/failed 的转换。
4. 确认 `ActiveKey` 和 `SystemTaskLock` 各负责什么。
5. 跑：

```bash
go test ./model -run TestSystemTask -v
```

### 练习 5：读计费边界

1. 读 `service/tiered_settle_test.go`。
2. 找 `TryTieredSettle`。
3. 找 `BillingSnapshot`。
4. 看 `p == 200000` 和 `p == 200001` 的不同。
5. 再读 `pkg/billingexpr/expr.md`。

## 十一、把测试当成源码地图

当你不知道某个模块怎么实现时，可以反过来找测试：

```bash
rg -n "函数名|类型名|字段名" -g '*_test.go'
```

例如：

```bash
rg -n "TryTieredSettle" -g '*_test.go'
rg -n "StreamScannerHandler" -g '*_test.go'
rg -n "TokenAuth" -g '*_test.go'
rg -n "RecordConsumeLog" -g '*_test.go'
```

测试文件通常会给你三个答案：

1. 这个函数最重要的业务语义是什么。
2. 哪些边界条件不能改坏。
3. 调用它之前需要准备哪些上下文和状态。

对学习 Go 的人来说，这比直接钻进生产代码更友好。先从测试建立输入输出模型，再回到生产代码看实现细节，理解速度会快很多。

