# new-api 数据模型与状态流学习指南

这份文档从 `model/` 目录切入，解释 new-api 的核心数据对象、表之间的关系、状态流转和读源码路线。它的目标不是替代数据库文档，而是帮助你读 Go 源码时知道：

- 哪些 struct 是系统的核心实体。
- 每张表大概解决什么业务问题。
- controller/service/relay 为什么会读写这些字段。
- 哪些状态变化是请求链路、计费链路、后台任务链路的关键。
- 学 Go 时可以从这些 model 代码里学到什么。

## 一、先建立一张对象地图

new-api 的后端是典型的 layered 架构：Router -> Controller -> Service -> Model。`model/` 是数据库和缓存的核心层，既包含 GORM model，也包含一些紧贴数据的查询、更新、缓存失效和兼容迁移逻辑。

可以先把核心对象分成八类：

| 类别 | 主要 struct | 主要文件 | 一句话理解 |
| --- | --- | --- | --- |
| 用户和权限 | `User` | `model/user.go` | 账号、角色、状态、余额、分组、设置 |
| API Key | `Token` | `model/token.go` | 用户调用 relay 的 key、额度、模型限制、IP 限制 |
| 渠道 | `Channel`、`ChannelInfo` | `model/channel.go` | 上游 provider 配置、模型、分组、多 key 状态 |
| 能力索引 | `Ability` | `model/ability.go` | 哪个 group 的哪个 model 可由哪个 channel 提供 |
| 模型元数据 | `Model`、`Vendor` | `model/model_meta.go`、`model/vendor_meta.go` | 管理台展示和模型归属信息 |
| 日志 | `Log` | `model/log.go` | 消费、错误、登录、管理、充值等审计和统计记录 |
| 异步任务 | `Task` | `model/task.go` | MJ/Suno/视频等异步 provider 任务状态 |
| 系统任务 | `SystemTask`、`SystemTaskLock` | `model/system_task.go` | 后台周期任务和多节点租约 |
| 配置 | `Option` | `model/option.go` | DB 中可热更新的系统配置 |
| 订阅 | `SubscriptionPlan`、`UserSubscription`、`SubscriptionPreConsumeRecord` | `model/subscription.go` | 订阅套餐、用户订阅、预扣费幂等记录 |

读源码时最重要的关系是：

```text
User
  -> Token
     -> relay 请求鉴权和 token 额度
  -> Log
     -> 消费、错误、登录、管理审计
  -> UserSubscription
     -> 订阅额度和用户组升级/回退

Channel
  -> Ability
     -> group + model + channel_id 的可用性索引
  -> Log
     -> 记录渠道消耗和错误
  -> Task
     -> 异步任务提交到哪个渠道

Option
  -> common/setting/ratio_setting 全局配置
  -> relay、计费、限流、前端展示等运行时行为

SystemTask
  -> SystemTaskLock
     -> 多节点下同类后台任务只由一个 runner 执行
```

## 二、User：账号、角色、分组和钱包额度

`model/user.go` 的 `User` 是用户主表。核心字段可以按业务分组理解：

| 字段 | 含义 |
| --- | --- |
| `Id`、`Username`、`Password`、`DisplayName`、`Email` | 账号基础信息 |
| `Role` | 角色，普通用户、管理员、root |
| `Status` | 启用/禁用 |
| `Quota` | 用户钱包剩余额度 |
| `UsedQuota` | 用户历史已用额度 |
| `RequestCount` | 请求次数统计 |
| `Group` | 用户所在分组，影响可用模型/渠道/倍率 |
| `Setting` | 用户个性化 JSON 设置 |
| `AccessToken` | 后台管理 access token，不是 relay key |
| `AdminPermissions` | 运行时附加字段，不入库 |

`User` 上有几个方法适合 Go 初学者精读：

- `ToBaseUser()`：把完整用户裁剪成缓存用的 `UserBase`。
- `GetSetting()` / `SetSetting()`：用 `common.Unmarshal` / `common.Marshal` 读写 JSON 字符串。
- `UpdateUserSetting()`：更新 DB 后同步用户设置缓存。

注意 `OriginalPassword`、`VerificationCode`、`AdminPermissions` 的 GORM tag 都是 `gorm:"-:all"`，表示这些字段只在内存或请求中使用，不落库。

### 2.1 User 在请求链路里的作用

登录链路：

```text
controller.Login()
  -> model.User.ValidateAndFill()
  -> setupLogin()
  -> session 写入 user id
  -> model.RecordLoginLog()
```

后台页面链路：

```text
middleware.UserAuth()
  -> 从 session/access token 识别用户
  -> controller.GetSelf()
  -> model.GetUserById()
  -> 返回 quota/group/permissions/sidebar 等
```

relay 链路：

```text
middleware.TokenAuth()
  -> model.ValidateUserToken()
  -> model.GetUserCache()
  -> 检查 User.Status、User.Group、User.Quota
  -> 写入 gin.Context
```

计费链路：

```text
service.PreConsumeBilling()
  -> WalletFunding.PreConsume()
  -> model.DecreaseUserQuota()
  -> PostTextConsumeQuota()
  -> model.UpdateUserUsedQuotaAndRequestCount()
```

### 2.2 Go 学习点

- struct tag：同一个字段同时有 `json`、`gorm`、`validate` tag。
- 指针字段：`AccessToken *string` 表示可空。
- 方法接收者：`func (user *User) SetSetting(...)` 会修改对象；`ToBaseUser()` 返回新对象。
- JSON 规则：项目业务代码用 `common.Marshal` / `common.Unmarshal`，不是直接调 `encoding/json`。

## 三、Token：relay API key、额度和模型限制

`model/token.go` 的 `Token` 是用户调用 AI API 的 key。它和 `User.AccessToken` 不是一回事：

- `User.AccessToken` 用于后台管理 API。
- `Token.Key` 用于 `/v1/chat/completions` 这类 relay 请求。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `UserId` | token 属于哪个用户 |
| `Key` | `sk-` 去前缀后存储的 key |
| `Status` | enabled / expired / exhausted 等 |
| `RemainQuota` | token 剩余额度 |
| `UnlimitedQuota` | 是否无限额度 |
| `ModelLimitsEnabled`、`ModelLimits` | 是否限制可用模型 |
| `AllowIps` | IP 白名单 |
| `Group` | token 指定使用组 |
| `CrossGroupRetry` | auto 分组下是否允许跨组重试 |

### 3.1 Token 鉴权状态流

relay 请求进入 `TokenAuth()` 后，会调用 `model.ValidateUserToken(key)`：

```text
ValidateUserToken(key)
  -> GetTokenByKey(key, false)
     -> Redis 命中则直接返回
     -> Redis 未命中查 DB
     -> 异步写回 Redis
  -> 检查 Status
  -> 检查 ExpiredTime
  -> 检查 RemainQuota
  -> 返回 token 或 ErrTokenInvalid
```

如果 token 过期或额度耗尽，在未启用 Redis 时会直接把 token 状态更新为 expired/exhausted。启用 Redis 时要考虑缓存一致性，相关读写分散在 `model/token_cache.go`。

### 3.2 搜索和安全细节

`SearchUserTokens()` 支持按名称和 token key 搜索，但对 LIKE 模式做了严格清洗：

- 用 `!` 作为 ESCAPE 字符，兼容 MySQL/PostgreSQL/SQLite。
- 拒绝连续 `%`。
- `%` 最多 2 个。
- 模糊搜索去掉 `%` 后至少 2 个字符。
- token 数量超上限时只允许精确搜索。

这是数据库兼容和防滥用的好例子。读这段时可以顺便复习 SQL LIKE、ESCAPE 和跨数据库差异。

### 3.3 Go 学习点

- 错误哨兵：`ErrTokenNotProvided`、`ErrTokenInvalid`、`ErrDatabase`。
- `errors.Is`：区分 `gorm.ErrRecordNotFound`。
- `defer`：`GetTokenByKey()` 用 defer 在函数返回后异步写缓存。
- 字符串处理：`strings.TrimPrefix(key, "sk-")`、`strings.Split`、`strings.TrimSpace`。

## 四、Channel 和 Ability：渠道配置与可用性索引

`Channel` 是上游 provider 配置表，`Ability` 是为了快速按 group/model 找可用渠道而维护的索引表。理解渠道选择时，一定要把它们一起看。

### 4.1 Channel 核心字段

| 字段 | 含义 |
| --- | --- |
| `Type` | 渠道类型，对应 `constant.ChannelType*` |
| `Key` | 上游 API key，可能是多行多 key |
| `BaseURL` | 上游地址 |
| `Models` | 该渠道支持的模型 CSV |
| `Group` | 该渠道开放给哪些组，CSV |
| `Status` | 启用、禁用、手动禁用等 |
| `Priority` | 优先级，越高越优先 |
| `Weight` | 同优先级下随机权重 |
| `ModelMapping` | 客户端模型名到上游模型名映射 |
| `StatusCodeMapping` | 上游状态码到下游状态码映射 |
| `Setting` | 渠道设置 |
| `OtherSettings` | 其他设置，例如 Azure 版本、上游 RPM |
| `ParamOverride`、`HeaderOverride` | 请求参数/请求头覆盖 |
| `ChannelInfo` | 多 key 状态 |

`ChannelInfo` 实现了 `driver.Valuer` 和 `sql.Scanner`：

```go
func (c ChannelInfo) Value() (driver.Value, error) {
    return common.Marshal(&c)
}

func (c *ChannelInfo) Scan(value interface{}) error {
    bytesValue, _ := value.([]byte)
    return common.Unmarshal(bytesValue, c)
}
```

这让 GORM 可以把 Go struct 自动存进 JSON/text 字段，再从 DB 读回 struct。

### 4.2 多 key 状态机

`Channel.GetNextEnabledKey()` 是多 key 逻辑核心：

```text
if !IsMultiKey:
  return channel.Key

keys := channel.GetKeys()
enabledIdx := 过滤 MultiKeyStatusList 中启用的 key
if 没有 enabled key:
  return ChannelNoAvailableKey

switch MultiKeyMode:
  random  -> 随机选一个 enabled key
  polling -> 从 MultiKeyPollingIndex 开始轮询
  default -> 返回第一个 enabled key
```

轮询模式会用 `GetChannelPollingLock(channel.Id)` 做 channel 级锁，避免并发请求同时拿到同一个轮询 index。

### 4.3 Ability 是渠道选择的索引表

`Ability` 的主键是：

```text
Group + Model + ChannelId
```

它的字段很少，但非常关键：

| 字段 | 含义 |
| --- | --- |
| `Group` | 使用组 |
| `Model` | 模型名 |
| `ChannelId` | 渠道 ID |
| `Enabled` | 该 group/model/channel 是否可用 |
| `Priority` | 选择优先级 |
| `Weight` | 权重 |
| `Tag` | 标签 |

当渠道新增或更新时，`Channel.Insert()` 会调用 `AddAbilities()`，`Channel.Update()` 会调用 `UpdateAbilities()`。当渠道状态变化时，`UpdateChannelStatus()` 会同步 ability 状态。

relay 选择渠道时不是直接扫 `channels.models`，而是依赖 ability/cache：

```text
middleware.Distribute()
  -> service.CacheGetRandomSatisfiedChannel()
  -> model.GetRandomSatisfiedChannelWithExclusions()
  -> channel cache 中按 group/model/priority/weight 找候选
```

### 4.4 Channel/Ability 和 Model/Vendor 的关系

`model/model_meta.go` 的 `Model` 不是 relay 请求里的唯一模型来源。它更多用于管理台展示、供应商归类、模型描述、图标、标签、端点和绑定渠道信息。

`Vendor` 是模型供应商元数据，给 `Model.VendorID` 引用。实际请求是否可用，仍看 `Channel + Ability + Group + Model`。

所以不要误解：

- `Model` 表存在不等于一定可调用。
- `Ability` enabled 才是 group/model/channel 可用性的关键。
- `Channel.Models` 是渠道配置源之一，更新后会生成/同步 ability。

### 4.5 Go 学习点

- 自定义 DB 序列化：`Value()` / `Scan()`。
- 指针字段表达“未设置”和“零值”差异，例如 `Priority *int64`、`Weight *uint`。
- 并发锁：多 key polling 用 channel 级 mutex。
- 跨数据库 SQL：`channelGroupFilterCondition()` 对 MySQL 和 SQLite/PostgreSQL 使用不同字符串拼接语法。

## 五、Option：运行时配置表

`Option` 很简单：

```go
type Option struct {
    Key   string `json:"key" gorm:"primaryKey"`
    Value string `json:"value"`
}
```

复杂点不在表结构，而在“写入 DB 后如何同步到全局运行时状态”。

### 5.1 配置加载流程

```text
InitOptionMap()
  -> 加锁 common.OptionMapRWMutex
  -> 创建 common.OptionMap
  -> 写入 common/setting/ratio_setting 默认值
  -> config.GlobalConfig.ExportAllConfigs()
  -> 解锁
  -> loadOptionsFromDatabase()
     -> 先处理 QuotaPerUnit
     -> 再处理其他 options
     -> updateOptionMap(key, value)
        -> 更新 common/setting/ratio_setting/config 全局变量
```

为什么先处理 `QuotaPerUnit`？因为一些额度类配置可能以金额形式出现，需要先确定 quota 和货币单位的换算关系。

### 5.2 配置更新流程

```text
controller.UpdateOption()
  -> model.UpdateOption(key, value)
     -> DB.FirstOrCreate()
     -> DB.Save()
     -> updateOptionMap()
```

多节点情况下，其他节点靠 `go model.SyncOptions(common.SyncFrequency)` 周期从 DB 重新加载。

### 5.3 Go 学习点

- 全局 map 加锁：`common.OptionMapRWMutex.Lock()` / `Unlock()`。
- 类型转换：大量 `strconv.Itoa`、`strconv.FormatBool`、`strconv.ParseFloat`。
- 反射配置：分层配置通过 `setting/config` 注册，再由 `handleConfigUpdate()` 写入。
- 启动顺序：`InitOptionMap()` 必须在 `model.InitDB()` 后调用，因为它要读 DB options。

## 六、Log：消费、错误和审计的统一记录

`model/log.go` 的 `Log` 是统一日志表。日志库可能和主库分离，也可能是 ClickHouse。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `UserId`、`Username` | 日志归属用户 |
| `CreatedAt` | 时间戳 |
| `Type` | 日志类型 |
| `Content` | 可读文本 |
| `TokenName`、`TokenId` | 使用的 token |
| `ModelName` | 模型名 |
| `Quota` | 本次额度变化 |
| `PromptTokens`、`CompletionTokens` | token 用量 |
| `ChannelId`、`ChannelName` | 渠道 |
| `Group` | 使用组 |
| `RequestId`、`UpstreamRequestId` | 请求追踪 |
| `Other` | 扩展 JSON |

日志类型没有用 `iota` 自动递增，而是显式常量：

```text
0 unknown
1 topup
2 consume
3 manage
4 system
5 error
6 refund
7 login
```

这是一个稳定性设计：日志类型已经存入数据库，不能因为插入新常量导致历史含义改变。

### 6.1 consume log

文本请求成功后：

```text
service.PostTextConsumeQuota()
  -> calculateTextQuotaSummary()
  -> service.SettleBilling()
  -> model.RecordConsumeLog()
```

音频请求类似走 `service.PostAudioConsumeQuota()`。

`RecordConsumeLog()` 会写 prompt/completion tokens、quota、模型、渠道、分组、stream 标记和 `other`。如果 `common.LogConsumeEnabled=false`，消费日志可以被关闭。

### 6.2 error log

渠道错误由 `controller.processChannelError()` 处理。当 `constant.ErrorLogEnabled` 且 `types.IsRecordErrorLog(err)` 时，会调用 `model.RecordErrorLog()` 写 `LogTypeError`。

错误日志的 `Other` 会包含 error type/code、status code、渠道、请求路径、admin info 等。普通用户查询日志时，`formatUserLogs()` 会移除 `admin_info`、`audit_info`、`stream_status` 等内部字段。

### 6.3 Go 学习点

- 稳定枚举：不要随意改变已落库常量的值。
- 可见性裁剪：同一条日志，管理员和普通用户看到的 `Other` 字段不同。
- 多数据库：日志库可独立配置，ClickHouse 下排序和 LIKE 处理都有差异。

## 七、Task：异步 provider 任务

`model/task.go` 的 `Task` 用于 MJ、Suno、视频生成等异步任务。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `TaskID` | 对外暴露的 `task_` 前缀 ID |
| `Platform` | 平台，例如 Suno、Kling、Gemini task |
| `UserId`、`Group` | 用户和计费组 |
| `ChannelId` | 提交任务的渠道 |
| `Quota` | 任务额度 |
| `Action` | 任务动作 |
| `Status` | NOT_START/SUBMITTED/QUEUED/IN_PROGRESS/SUCCESS/FAILURE |
| `Progress` | 进度 |
| `Properties` | 对外可见的补充信息 |
| `PrivateData` | 内部敏感信息，不返回给用户 |
| `Data` | provider 返回数据 |

`TaskPrivateData` 很重要，它保存轮询阶段需要的内部信息：

- 上游真实 task id
- 结果 URL
- 上游 key
- 计费来源
- subscription id
- token id
- 发起节点名
- 任务计费快照

这也是为什么 `PrivateData` 的 JSON tag 是 `json:"-"`：它可能包含 key，不允许直接返回给用户。

### 7.1 Task 状态流

典型异步任务流：

```text
controller.RelayTask()
  -> relay.GetTaskAdaptor()
  -> adaptor.ValidateRequestAndSetAction()
  -> helper.ModelPriceHelperPerCall()
  -> service.PreConsumeBilling()
  -> model.InitTask()
  -> adaptor.DoRequest()
  -> adaptor.DoResponse()
  -> task 状态 SUBMITTED/QUEUED
  -> 后台 async_task_poll
     -> adaptor.FetchTask()
     -> adaptor.ParseTaskResult()
     -> 更新 Task.Status/Progress/Data
     -> 完成时可能做差额结算或退款
```

`TaskStatus.ToVideoStatus()` 把内部任务状态转换成 OpenAI video 兼容状态，例如 queued、in_progress、completed、failed。

### 7.2 Go 学习点

- 自定义 JSON 字段：`Properties`、`TaskPrivateData` 都实现了 `Scan()` / `Value()`。
- `json.RawMessage`：`Task.Data` 用来保存 provider 原始结构或转换后的结构。
- 状态常量：任务状态用 string 类型，便于 DB 和 API 直接读。
- 隐私字段：`json:"-"` 是防止敏感字段输出的常用方式。

## 八、SystemTask：系统级后台任务和租约

`SystemTask` 是后台任务表，和 `Task` 不同：

- `Task` 是用户发起的 provider 异步任务。
- `SystemTask` 是系统内部周期任务，例如日志清理、渠道测试、模型更新、异步任务轮询。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `TaskID` | `systask_` 前缀 ID |
| `Type` | `log_cleanup`、`channel_test` 等 |
| `Status` | pending/running/succeeded/failed |
| `ActiveKey` | 防止同类 active 任务重复创建 |
| `Payload` | 任务输入 |
| `State` | 任务中间状态 |
| `Result` | 成功结果 |
| `Error` | 失败信息 |
| `LockedBy` | 哪个 runner 持有 |

`SystemTaskLock` 是租约表：

| 字段 | 含义 |
| --- | --- |
| `Type` | 任务类型，主键 |
| `TaskID` | 当前持有租约的任务 |
| `LockedBy` | runner id |
| `LockedUntil` | 租约过期时间 |

### 8.1 Claim 和 lease 流程

```text
service.StartSystemTaskRunner()
  -> model.FindEarliestPendingSystemTasks()
  -> model.ClaimSystemTask()
     -> acquireSystemTaskLock()
     -> pending -> running
  -> runWithLeaseHeartbeat()
     -> 周期 ExtendSystemTaskLock()
  -> handler.Handle()
  -> model.FinishSystemTask()
     -> running -> succeeded/failed
     -> ReleaseSystemTaskLock()
```

这套设计用于多节点部署：即使多个 master 同时运行，也尽量保证同一类系统任务只有一个 runner 执行。

### 8.2 Go 学习点

- GORM hooks：`BeforeCreate()` 自动写时间戳。
- 乐观状态更新：`WHERE id=? AND status=pending` 再更新为 running。
- 租约模型：不是永久锁，而是 `LockedUntil`，runner 需要续租。
- JSON payload：`Payload`、`State`、`Result` 存字符串，由 service 层 marshal/unmarshal。

## 九、Subscription：订阅、预扣和幂等退款

订阅相关模型集中在 `model/subscription.go`。

### 9.1 订阅套餐和用户订阅

`SubscriptionPlan` 是套餐配置：

- `PriceAmount`、`Currency`：展示价格。
- `DurationUnit`、`DurationValue`、`CustomSeconds`：有效期。
- `TotalAmount`：总额度，0 表示无限。
- `QuotaResetPeriod`：never/daily/weekly/monthly/custom。
- `UpgradeGroup`、`DowngradeGroup`：购买后升级用户组，过期后降级。
- `AllowWalletOverflow`：订阅额度不足时是否允许钱包兜底。

`UserSubscription` 是用户持有的订阅实例：

- `AmountTotal` / `AmountUsed`：订阅额度总量和已用量。
- `StartTime` / `EndTime`：有效期。
- `Status`：active/expired/cancelled。
- `LastResetTime` / `NextResetTime`：周期重置额度。
- `PrevUserGroup`：过期后恢复用户组用。

### 9.2 订阅预扣幂等记录

`SubscriptionPreConsumeRecord` 是订阅预扣费幂等表：

| 字段 | 含义 |
| --- | --- |
| `RequestId` | 请求 ID，唯一 |
| `UserId` | 用户 |
| `UserSubscriptionId` | 扣的是哪条订阅 |
| `PreConsumed` | 预扣额度 |
| `Status` | consumed/refunded |

预扣流程：

```text
PreConsumeUserSubscription(requestId, userId, modelName, quotaType, amount)
  -> DB.Transaction()
  -> 按 request_id 查 existing record
  -> 若已存在且未退款，返回已有预扣结果
  -> 查询 active subscription
  -> maybeResetUserSubscriptionWithPlanTx()
  -> 检查剩余额度
  -> 创建 SubscriptionPreConsumeRecord
  -> sub.AmountUsed += amount
  -> 保存订阅
```

退款流程：

```text
RefundSubscriptionPreConsume(requestId)
  -> DB.Transaction()
  -> FOR UPDATE 锁定 record
  -> 已 refunded 则直接返回
  -> PostConsumeUserSubscriptionDelta(subId, -preConsumed)
  -> record.Status = refunded
```

这就是订阅资金来源能做到失败退款幂等的原因。

### 9.3 Go 学习点

- 事务：预扣和退款都用 `DB.Transaction()` 包住。
- 幂等：`request_id` 唯一索引让同一个请求不会重复扣订阅。
- 时间计算：`calcPlanEndTime()`、`calcNextResetTime()` 展示了按年/月/日/小时/custom 处理有效期。
- 缓存：套餐缓存使用 `pkg/cachex.HybridCache`，同时支持内存和 Redis。

## 十、读源码时的状态流总表

### 10.1 relay 请求状态

```text
TokenAuth: 未认证 -> 已认证
Distribute: 未选渠道 -> context 中有 channel
Relay: 未解析请求 -> RelayInfo ready
Pricing: 未计费 -> PriceData ready
Billing: 未预扣 -> BillingSession ready
Adaptor: 未发送 -> 上游响应
Quota: usage -> actual quota -> SettleBilling
Log: RecordConsumeLog 或 RecordErrorLog
```

### 10.2 token 状态

```text
enabled
  -> expired   到期
  -> exhausted 额度耗尽
  -> disabled 用户/管理员禁用
```

### 10.3 channel 状态

```text
enabled
  -> disabled 自动禁用或手动禁用
  -> enabled 自动/手动启用

multi-key:
  key enabled
    -> key disabled
    -> key enabled
```

### 10.4 task 状态

```text
NOT_START
  -> SUBMITTED
  -> QUEUED
  -> IN_PROGRESS
  -> SUCCESS / FAILURE / UNKNOWN
```

### 10.5 system task 状态

```text
pending
  -> running
  -> succeeded / failed
```

### 10.6 subscription pre-consume 状态

```text
consumed
  -> refunded
```

## 十一、最容易踩错的点

| 容易误解 | 正确理解 |
| --- | --- |
| `User.AccessToken` 是 relay key | relay key 是 `Token.Key`，`AccessToken` 是后台管理 access token |
| `Model` 表有模型就能调用 | 实际可调用由 `Ability` 和 `Channel` 决定 |
| `Channel.Models` 是唯一索引 | 请求选渠主要查 ability/cache，`Models` 是配置源 |
| `Token.Group` 一定等于 `User.Group` | token 可以指定使用组，用户组还影响权限和可用组 |
| `Channel.Key` 一定是一把 key | 多 key 模式下可能是多行 key 或特殊 JSON |
| `Log.Other` 可以直接展示给所有用户 | 普通用户查询会裁剪 admin/debug 字段 |
| `Task.PrivateData` 可以返回给前端 | 它可能含 key 和计费上下文，必须隐藏 |
| `SystemTask` 和 `Task` 是同一类任务 | `Task` 是用户异步任务，`SystemTask` 是系统后台任务 |
| 订阅扣费失败可以简单加回额度 | 项目用 `SubscriptionPreConsumeRecord` 做 request_id 幂等退款 |
| 修改 option 只写 DB 就行 | 还要 `updateOptionMap()` 更新当前进程内存 |

## 十二、建议练习

1. 从 `TokenAuth()` 开始，跟到 `model.ValidateUserToken()`，画出 token 失效的三个原因。
2. 从渠道管理的 `AddChannel()` 开始，跟到 `Channel.Insert()` 和 `AddAbilities()`，理解 channel 如何变成 ability。
3. 从 `/v1/chat/completions` 开始，记录哪些字段从 `Token`、`User`、`Channel` 写入了 `gin.Context`。
4. 从 `PostTextConsumeQuota()` 开始，跟到 `RecordConsumeLog()`，列出一条消费日志里哪些字段来自 usage，哪些来自 `RelayInfo`。
5. 从 `PreConsumeUserSubscription()` 开始，说明为什么同一个 request id 不会重复扣订阅额度。
6. 从 `StartSystemTaskRunner()` 开始，解释 `SystemTask.ActiveKey` 和 `SystemTaskLock` 分别防什么。
7. 找一个 `json:"-"` 字段，说明它为什么不能返回给前端。
8. 找一个实现 `Scan()` / `Value()` 的类型，解释 GORM 如何把它存进数据库。
