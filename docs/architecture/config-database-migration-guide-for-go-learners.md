# 配置、数据库与迁移学习指南

这份文档专门梳理 new-api 的启动初始化、数据库选择、GORM 迁移、`options` 表、`setting` 包和配置热更新。它适合在读完 `go-source-learning-guide.md` 和 `package-study-map-for-go-learners.md` 之后阅读。

如果把 relay 看成项目的业务心脏，那么配置和数据库初始化就是骨架。很多线上行为，例如模型价格、分组倍率、支付开关、重试状态码、主题、性能保护、订阅计费，都不是写死在请求函数里，而是通过启动默认值、数据库 `options`、后台热同步共同决定。

## 一、启动顺序总览

入口在 `main.go`：

```text
main()
  -> InitResources()
  -> 启动缓存同步、配置同步、后台任务
  -> 创建 Gin server
  -> 注册中间件和 session
  -> router.SetRouter()
  -> ListenAndServe()
  -> 等待 SIGINT/SIGTERM 优雅关闭
```

`InitResources()` 是启动阶段最值得精读的函数。核心顺序：

```text
godotenv.Load(".env")
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
  -> OAuth 初始化
```

这里有几个必须保持的前后关系：

- `common.InitEnv()` 必须早，因为它读取端口、调试、Redis、节点角色等环境变量。
- `logger.SetupLogger()` 要在大量日志输出前执行。
- `ratio_setting.InitRatioSettings()` 要在价格和倍率相关配置使用前执行。
- `model.InitDB()` 必须早于 `model.InitOptionMap()`，因为后者要读 `options` 表。
- `model.InitOptionMap()` 要早于运行时业务请求，因为它会把数据库配置写入 `common`、`setting`、`ratio_setting` 等包。
- `model.InitLogDB()` 在 `model.InitDB()` 后执行，因为日志库可能复用主库，也可能使用独立 `LOG_SQL_DSN`。
- `common.InitRedisClient()` 在 DB/Option 后执行，后续用户、token、缓存才有 Redis 能力。

## 二、环境变量层

启动时首先读取 `.env`，然后 `common.InitEnv()` 解析环境变量。

调试相关：

```text
DEBUG=true
GIN_MODE=debug
LOG_CALLER_ENABLED=true
ENABLE_PPROF=true
```

数据库相关：

```text
SQL_DSN
LOG_SQL_DSN
SQL_MAX_IDLE_CONNS
SQL_MAX_OPEN_CONNS
SQL_MAX_LIFETIME
```

节点和同步相关：

```text
SYNC_FREQUENCY
NODE_TYPE
CHANNEL_UPDATE_FREQUENCY
BATCH_UPDATE_ENABLED
```

注意：环境变量一般决定“进程启动时的基础能力”，而 `options` 表决定“运行时可热更新的业务配置”。两者不要混淆。

## 三、数据库类型选择

数据库选择逻辑在 `model/main.go` 的 `chooseDB(envName, isLog)`。

主库使用：

```text
SQL_DSN
```

日志库使用：

```text
LOG_SQL_DSN
```

支持情况：

| 数据库 | 主库 | 日志库 |
| --- | --- | --- |
| SQLite | 支持 | 支持 |
| MySQL | 支持 | 支持 |
| PostgreSQL | 支持 | 支持 |
| ClickHouse | 不支持 | 支持 |

`chooseDB` 判断规则：

- DSN 为空：默认 SQLite，路径来自 `common.SQLitePath`。
- DSN 以 `postgres://` 或 `postgresql://` 开头：PostgreSQL。
- DSN 以 `local` 开头：SQLite。
- ClickHouse DSN 只允许日志库使用。
- 其他非空 DSN 默认按 MySQL 处理。

MySQL 会自动补 `parseTime=true`，PostgreSQL 使用 `PreferSimpleProtocol: true`，SQLite 使用 `glebarez/sqlite`。

数据库类型保存在 `common/database.go`：

```go
func MainDatabaseType() DatabaseType
func LogDatabaseType() DatabaseType
func UsingMainDatabase(databaseType DatabaseType) bool
func UsingLogDatabase(databaseType DatabaseType) bool
```

跨库代码不要自己猜数据库类型，应该用这些函数。

## 四、主库初始化

主库入口：

```text
model.InitDB()
```

流程：

```text
chooseDB("SQL_DSN", false)
  -> common.SetMainDatabaseType(dbType)
  -> 如果没有 LOG_SQL_DSN，则 log DB type = main DB type
  -> initCol()
  -> DEBUG 时 db.Debug()
  -> DB = db
  -> MySQL 中文字符集检查
  -> 设置连接池
  -> 非 master 节点直接返回
  -> master 节点执行 migrateDB()
```

这里体现了多实例部署下的一个原则：

```text
所有节点都连接 DB
只有 master 节点跑迁移
```

否则多个实例同时 AutoMigrate 或执行手工迁移，容易引发竞态。

## 五、日志库初始化

日志库入口：

```text
model.InitLogDB()
```

如果没有 `LOG_SQL_DSN`：

```text
LOG_DB = DB
```

如果有 `LOG_SQL_DSN`：

```text
chooseDB("LOG_SQL_DSN", true)
  -> common.SetLogDatabaseType(dbType)
  -> initCol()
  -> DEBUG 时 db.Debug()
  -> LOG_DB = db
  -> 设置连接池
  -> master 节点 migrateLOGDB()
```

日志库可以是 ClickHouse。主库不能是 ClickHouse，因为主业务需要 GORM model、事务、更新等能力。

日志库迁移入口是 `migrateLOGDB()`。普通日志库只迁移 `Log`，ClickHouse 会走独立的 ClickHouse 建表逻辑，使用 MergeTree 并同步 TTL、排序字段等日志查询需求。

## 六、跨库列名和布尔值

`model/main.go` 里有几个跨库变量：

```go
var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string
```

原因是：

- `group`、`key` 在一些数据库中是保留字，需要不同引用方式。
- PostgreSQL 用 `"group"`，MySQL/SQLite 用 `` `group` ``。
- 布尔值 PostgreSQL 用 `true/false`，MySQL/SQLite 常用 `1/0`。

所以当你看到原始 SQL 时，要特别注意：

```text
不要手写 `group`
优先用 commonGroupCol / logGroupCol
不要假设布尔值跨库一致
```

这也是项目 AGENTS 中强调数据库兼容的原因。

## 七、GORM AutoMigrate

主库迁移入口：

```text
migrateDB()
```

它先执行一些手工兼容迁移，例如：

- subscription plan price amount 类型迁移。
- token model limits 字段迁移。
- upstream identity 迁移。

然后使用：

```go
DB.AutoMigrate(...)
```

迁移大量模型：

- `Channel`
- `Token`
- `User`
- `PasskeyCredential`
- `Option`
- `Ability`
- `Log`
- `Task`
- `Model`
- `Vendor`
- `SubscriptionOrder`
- `UserSubscription`
- `SubscriptionPreConsumeRecord`
- `CustomOAuthProvider`
- `PerfMetric`
- `SystemInstance`
- `SystemTask`
- `SystemTaskLock`
- `CasbinRule`
- `AuthzRole`

SQLite 下某些表有特殊处理，例如 `SubscriptionPlan` 需要独立兼容函数。原因是 SQLite 对部分 ALTER 操作支持有限。

对 Go 初学者来说，理解 GORM 迁移要抓住两点：

1. struct tag 决定大部分表结构。
2. 真实项目里仍然需要手工迁移补充 AutoMigrate 做不到或跨库不一致的部分。

## 八、Option 表是什么

`model.Option` 很简单：

```go
type Option struct {
    Key   string `json:"key" gorm:"primaryKey"`
    Value string `json:"value"`
}
```

但它承载了大量运行时配置。

可以把 `options` 表理解为：

```text
数据库持久化配置中心
```

启动时：

```text
model.InitOptionMap()
```

运行时定期同步：

```text
go model.SyncOptions(common.SyncFrequency)
```

后台管理页面更新：

```text
PUT /api/option
  -> controller/option.go
  -> model.UpdateOption / UpdateOptionsBulk
  -> updateOptionMap
```

## 九、InitOptionMap 的两层加载

`InitOptionMap()` 分两步：

```text
1. 写入代码默认值到 common.OptionMap
2. 从 DB options 表读取已有值覆盖默认值
```

第一步会填充大量默认配置：

- 上传下载权限。
- 登录注册开关。
- OAuth 开关。
- 支付配置。
- 模型倍率。
- 分组倍率。
- 用户可用分组。
- 计费倍率。
- 敏感词。
- 自动禁用状态码。
- 分层配置导出的默认值。

第二步：

```text
loadOptionsFromDatabase()
```

它先处理 `QuotaPerUnit`，再处理其他 option。这个顺序很重要，因为一些额度配置需要依赖 `QuotaPerUnit` 做单位转换。

## 十、OptionMap 和热更新

`common.OptionMap` 是内存中的 string map，受 `common.OptionMapRWMutex` 保护。

写入流程：

```text
model.UpdateOption(key, value)
  -> DB.FirstOrCreate
  -> DB.Save
  -> updateOptionMap(key, value)
```

批量写入：

```text
model.UpdateOptionsBulk(values)
  -> DB.Transaction
  -> 所有 DB 写成功
  -> 逐个 updateOptionMap
```

`UpdateOptionsBulk` 的语义更强：

```text
DB 事务成功后才更新内存
DB 任意失败则内存不变
```

适合支付网关这类必须成组保存的配置。

## 十一、旧式配置分发

`updateOptionMap` 里有大量 switch，把 string 值转换成不同包中的全局变量。

例子：

```text
PasswordLoginEnabled
  -> common.PasswordLoginEnabled

ModelRatio
  -> ratio_setting.UpdateModelRatioByJSONString

PayMethods
  -> operation_setting.UpdatePayMethodsByJsonString

StripeApiSecret
  -> setting.StripeApiSecret

AutomaticRetryStatusCodes
  -> operation_setting.AutomaticRetryStatusCodesFromString
```

这条路径比较直接，但也比较长。读的时候不要试图一次记住所有 key，只需要掌握模式：

```text
option key
  -> 字符串解析
  -> 写入 common/setting/ratio_setting/operation_setting
  -> 运行时立即生效
```

## 十二、分层配置系统

新一些的配置走 `setting/config`。

核心对象：

```go
type ConfigManager struct {
    configs map[string]interface{}
    mutex   sync.RWMutex
}

var GlobalConfig = NewConfigManager()
```

配置包通过 `init()` 注册：

```go
func init() {
    config.GlobalConfig.Register("general_setting", &generalSetting)
}
```

数据库 key 形如：

```text
general_setting.ping_interval_enabled
billing_setting.billing_mode
billing_setting.billing_expr
performance_setting.disk_cache_enabled
theme.logo
```

`updateOptionMap` 会先调用：

```text
handleConfigUpdate(key, value)
```

如果 key 里有 `.`，并且前缀是已注册配置，就走分层配置更新：

```text
拆分 configName 和 configKey
  -> config.GlobalConfig.Get(configName)
  -> config.UpdateConfigFromMap
  -> 特定配置后处理
```

后处理包括：

- `performance_setting`: `UpdateAndSync()`
- `tool_price_setting`: `RebuildToolPriceIndex()`
- `billing_setting`: 清 pricing cache 和暴露数据 cache
- `theme`: `system_setting.UpdateAndSyncTheme()`

这条路径用反射处理 struct 字段，支持 string、bool、int、uint、float、pointer、map、slice、struct 等类型。

## 十三、常见 setting 包

| 包 | 作用 |
| --- | --- |
| `setting/ratio_setting` | 模型倍率、模型价格、分组倍率、缓存倍率、音频/图片倍率 |
| `setting/billing_setting` | `ratio` / `tiered_expr` 计费模式和表达式 |
| `setting/operation_setting` | 支付、重试状态码、自动禁用、渠道亲和、checkin、通用运营配置 |
| `setting/system_setting` | OIDC、Discord、Passkey、主题、法律文本、fetch/SSRF 防护 |
| `setting/model_setting` | Claude、Gemini、Grok、Qwen 等 provider/model 行为配置 |
| `setting/performance_setting` | 磁盘缓存、性能保护等配置 |
| `setting/console_setting` | 控制台配置校验 |
| `setting/perf_metrics_setting` | 性能指标聚合配置 |

读 setting 包时，看三件事：

1. 默认值定义在哪里。
2. 是否通过 `config.GlobalConfig.Register` 注册。
3. 热更新后是否需要额外同步或重建缓存。

还有一个容易漏的点：注册依赖 Go 包初始化图。比如某些 setting 包靠 blank import 触发 `init()`，如果新增配置包却没有被启动路径 import，它就不会注册进 `GlobalConfig`，`options` 表里的 `模块名.字段名` 也不会生效。

## 十四、配置到业务的例子

### 1. 模型倍率

```text
后台修改 ModelRatio
  -> model.UpdateOption("ModelRatio", json)
  -> ratio_setting.UpdateModelRatioByJSONString
  -> relay/helper/price.go 读取倍率
  -> 影响预扣和结算
```

### 2. tiered expression 计费

```text
后台修改 billing_setting.billing_mode / billing_expr
  -> handleConfigUpdate
  -> config.UpdateConfigFromMap
  -> InvalidatePricingCache
  -> ratio_setting.InvalidateExposedDataCache
  -> relay 计费读取 billing_setting
```

### 3. 流式 ping

```text
后台修改 general_setting.ping_interval_enabled
  -> operation_setting.GetGeneralSetting()
  -> relay/helper/stream_scanner.go
  -> 慢速流式响应期间输出 : PING
```

### 4. 支付方式

```text
后台修改 PayMethods
  -> operation_setting.UpdatePayMethodsByJsonString
  -> 前端充值页展示可用支付方式
  -> topup controller 按配置创建订单
```

## 十五、配置同步和多节点

每个进程启动后都会：

```text
go model.SyncOptions(common.SyncFrequency)
```

它定期：

```text
loadOptionsFromDatabase()
```

所以在多节点部署中，一个节点修改 DB option 后，其他节点会在下一次 sync 时更新内存。

这不是强一致，而是周期性最终一致。对配置变更来说通常够用；对权限策略还有单独的：

```text
authz.StartPolicySync(common.SyncFrequency)
```

## 十六、Go 学习点

这一部分源码特别适合学习：

1. 启动函数如何组织依赖顺序。
2. 如何用环境变量选择不同数据库。
3. GORM `AutoMigrate` 的边界。
4. 多数据库兼容时如何封装 dialect 差异。
5. 用 `sync.RWMutex` 保护全局 map。
6. 用 DB 表作为运行时配置中心。
7. 用反射实现通用配置导入导出。
8. 包级 `init()` 如何注册配置模块。
9. 事务成功后再更新内存状态。
10. 多节点配置如何通过定时同步达到最终一致。
11. `types.RWMap`、`sync.RWMutex`、`atomic.Bool`、`atomic.Pointer` 如何分别服务于不同读写模式。

## 十七、阅读练习

### 练习 1：画启动顺序

从 `main.go` 的 `InitResources()` 开始，画出：

```text
env -> logger -> ratio -> DB -> Option -> LogDB -> Redis -> i18n
```

并解释为什么 `InitOptionMap()` 不能放到 `InitDB()` 前面。

### 练习 2：追一个 Option

选择 `ModelRatio`：

```text
model.UpdateOption
  -> updateOptionMap
  -> ratio_setting.UpdateModelRatioByJSONString
  -> relay/helper/price.go
```

说明它如何影响一次请求计费。

### 练习 3：追一个分层配置

选择 `billing_setting.billing_expr`：

```text
setting/billing_setting/tiered_billing.go
  -> config.GlobalConfig.Register
  -> handleConfigUpdate
  -> GetBillingExpr
  -> tiered settle
```

说明它和旧式 `ModelRatio` 配置路径有什么不同。

### 练习 4：看跨库兼容

读 `model/main.go` 的 `initCol()`，回答：

- 为什么 PostgreSQL 用 `"group"`？
- 为什么 MySQL/SQLite 用 `` `group` ``？
- 原始 SQL 里为什么不能直接写布尔值？

### 练习 5：看迁移边界

读 `migrateDB()`，找出：

- 哪些表通过 `AutoMigrate`。
- 哪些迁移是手工函数。
- 为什么 SQLite 需要特殊处理。
