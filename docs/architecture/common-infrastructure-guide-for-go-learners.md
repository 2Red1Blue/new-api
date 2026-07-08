# new-api 通用基础设施学习指南

这份文档讲 `common/`、`pkg/cachex/`、部分 `service/` 和 middleware 基础设施。它们不是最显眼的业务入口，但决定了 new-api 如何处理 JSON、请求体复读、Redis/内存缓存、磁盘缓存、HTTP client、并发安全和资源清理。

如果你已经能读 controller 和 model，下一步就应该读这些公共设施。它们会让你理解“为什么业务代码能安全重试、能多次读 body、能跨 Redis/内存缓存、能追踪上游请求耗时”。

## 一、先看公共设施地图

| 模块 | 文件 | 解决什么问题 |
| --- | --- | --- |
| JSON wrapper | `common/json.go` | 统一 JSON marshal/unmarshal 入口 |
| Gin helper | `common/gin.go` | context helper、响应 helper、请求体复读入口 |
| BodyStorage | `common/body_storage.go` | 请求体存内存或磁盘，支持多次读取 |
| 磁盘缓存 | `common/disk_cache.go`、`common/disk_cache_config.go` | 大请求体/文件缓存、统计和清理 |
| 清理 middleware | `middleware/body_cleanup.go` | 请求结束后释放 body/file 缓存 |
| Redis | `common/redis.go` | Redis 初始化和基础 get/set/hash/incr |
| 泛型并发 map | `types/rw_map.go` | 带 `sync.RWMutex` 的泛型 map |
| HTTP client | `service/http_client.go` | 全局上游 HTTP client、代理、SSRF redirect 检查 |
| HybridCache | `pkg/cachex/*` | Redis 可用时用 Redis，否则用内存 hot cache |

## 二、JSON wrapper：为什么项目要求用 common.Marshal

`common/json.go` 提供统一 wrapper：

```go
func Unmarshal(data []byte, v any) error
func UnmarshalJsonStr(data string, v any) error
func DecodeJson(reader io.Reader, v any) error
func Marshal(v any) ([]byte, error)
func MarshalIndent(v any, prefix, indent string) ([]byte, error)
func GetJsonType(data json.RawMessage) string
```

项目约定：业务代码不要直接调用 `encoding/json.Marshal` / `Unmarshal`，应通过 `common.*`。这样以后如果要替换 JSON 实现、加统一行为、加观测或兼容处理，只需要改一个入口。

注意：`json.RawMessage`、`json.Number` 这类类型仍然可以引用。禁止的是业务代码里直接做 marshal/unmarshal 调用。

`GetJsonType()` 是一个小工具：看 `json.RawMessage` 去掉空白后的第一个字符，判断 object/array/string/boolean/null/number。它适合读 DTO 中“字段类型可能多态”的场景。

### Go 学习点

- `any` 是 `interface{}` 的别名，常用于通用 marshal/unmarshal。
- `io.Reader` 可以流式 decode，避免一次性把大 JSON 读入内存。
- `json.RawMessage` 用于延迟解析或保存原始 JSON 字段。

## 三、Gin helper：统一响应和请求体复读入口

`common/gin.go` 做两类事：Gin context helper 和后台 API response helper。

后台响应 helper：

```go
func ApiError(c *gin.Context, err error)
func ApiErrorMsg(c *gin.Context, msg string)
func ApiSuccess(c *gin.Context, data any)
func ApiErrorI18n(c *gin.Context, key string, args ...map[string]any)
func ApiSuccessI18n(c *gin.Context, key string, data any, args ...map[string]any)
```

普通 `/api/...` 业务失败常见返回是 HTTP 200 + `success:false`，前端 axios interceptor 会按这个结构处理。

context helper：

```go
func GetContextKeyType[T any](c *gin.Context, key constant.ContextKey) (T, bool)
```

这是项目里很实用的泛型函数：从 Gin context 取值后做类型断言，成功返回 `(value, true)`，失败返回零值和 false。

## 四、请求体复读：BodyStorage

HTTP 请求体默认只能读一次。但 new-api 需要多次读取 body：

- `middleware.Distribute()` 要先读取 model/group 来选渠道。
- `controller.Relay()` retry 时要把 body 重新发给另一个渠道。
- helper 可能要解析 JSON、multipart 或透传原始 body。

所以项目定义了 `BodyStorage`：

```go
type BodyStorage interface {
    io.ReadSeeker
    io.Closer
    Bytes() ([]byte, error)
    Size() int64
    IsDisk() bool
}
```

关键是 `io.ReadSeeker`：它既能读，也能 `Seek(0, io.SeekStart)` 回到开头。

### 4.1 内存存储

`memoryStorage` 内部是：

- `data []byte`
- `reader *bytes.Reader`
- `mu sync.Mutex`
- `closed int32`

它用 mutex 保护并发 Read/Seek/Bytes/Close，用 atomic closed 标记防止重复关闭。

### 4.2 磁盘存储

`diskStorage` 内部是：

- `file *os.File`
- `filePath string`
- `size int64`
- `mu sync.Mutex`
- `closed int32`

关闭时会：

```text
file.Close()
os.Remove(filePath)
DecrementDiskFiles(size)
```

也就是说磁盘缓存文件是请求临时文件，不应该长期保留。

### 4.3 创建策略

`CreateBodyStorageFromReader(reader, contentLength, maxBytes)` 的策略：

```text
if 开启磁盘缓存 && ContentLength 超过阈值 && 磁盘缓存容量够:
  直接把 reader 写入临时文件
else:
  io.ReadAll(io.LimitReader(reader, maxBytes+1))
  超过 maxBytes -> ErrRequestBodyTooLarge
  根据大小决定内存或磁盘
```

这里有一个很重要的设计：如果一开始走磁盘写入失败，不会尝试回退到内存，因为 reader 已经被消费，数据可能丢失。

### 4.4 GetRequestBody

`common.GetRequestBody(c)` 是请求体复读入口：

1. 如果 context 中已有 `KeyBodyStorage`，就 seek 到开头并返回。
2. 如果存在旧的 `KeyRequestBody []byte`，转成 BodyStorage。
3. 否则从 `c.Request.Body` 读取，创建 BodyStorage，关闭原始 body，存入 context。

`common.GetBodyStorage(c)` 则进一步保证返回的是 `BodyStorage`。

relay retry 时会这样用：

```text
bodyStorage, _ := common.GetBodyStorage(c)
c.Request.Body = io.NopCloser(bodyStorage)
```

这样同一个请求体可以被多个渠道重试使用。

### 4.5 ReaderOnly 的小技巧

`common.ReaderOnly(r io.Reader)` 用匿名 struct 包一层：

```go
return struct{ io.Reader }{r}
```

目的：隐藏 `io.Closer`。否则 `http.NewRequest` 可能把它识别为 `io.ReadCloser`，上游请求结束后关闭底层 `BodyStorage`，导致后续 retry 无法再读。

这是非常值得学习的 Go 接口技巧：通过包装只暴露你希望调用方看到的方法集合。

## 五、BodyStorageCleanup：请求结束清理

`middleware/body_cleanup.go` 的 `BodyStorageCleanup()` 挂在 API 和 relay 路由上：

```text
c.Next()
common.CleanupBodyStorage(c)
service.CleanupFileSources(c)
```

它确保请求结束后：

- 内存 buffer 统计被扣减。
- 磁盘临时文件被删除。
- URL 下载文件等 file source 被清理。

读这段时要注意 middleware 的执行顺序：请求进入时先执行 `c.Next()` 前面的逻辑，请求返回时执行 `c.Next()` 后面的清理逻辑。

## 六、磁盘缓存配置和统计

`common/disk_cache_config.go` 里有全局配置：

```go
type DiskCacheConfig struct {
    Enabled bool
    ThresholdMB int
    MaxSizeMB int
    Path string
}
```

配置由 `performance_setting` 同步到 `common.SetDiskCacheConfig()`。读写配置用 `sync.RWMutex`，统计用 `sync/atomic`。

统计字段包括：

- active disk files
- current disk usage bytes
- active memory buffers
- current memory usage bytes
- disk cache hits
- memory cache hits
- threshold/max bytes

`IsDiskCacheAvailable(requestSize)` 用当前使用量和最大容量判断还能不能创建新的磁盘缓存。

`common/disk_cache.go` 负责文件操作：

- `CreateDiskCacheFile()`
- `WriteDiskCacheFile()`
- `ReadDiskCacheFile()`
- `RemoveDiskCacheFile()`
- `CleanupOldDiskCacheFiles(maxAge)`
- `GetDiskCacheInfo()`

启动时 `InitResources()` 会调用 `common.CleanupOldCacheFiles()`，清理上次进程异常退出留下的临时文件。

### Go 学习点

- `sync.RWMutex` 保护配置结构体。
- `sync/atomic` 维护高频统计计数。
- 临时文件使用 `0600` 权限创建，避免敏感请求体被其他用户读。
- `os.O_EXCL` 保证文件名冲突时不会覆盖已有文件。

## 七、Redis 基础封装

`common/redis.go` 初始化 Redis：

```text
InitRedisClient()
  -> REDIS_CONN_STRING 为空: RedisEnabled=false
  -> redis.ParseURL()
  -> 设置 PoolSize
  -> redis.NewClient()
  -> Ping 5 秒超时
```

Redis 未配置时项目仍可运行。很多缓存都有 Redis 和内存两套路径。

基础函数：

- `RedisSet`
- `RedisGet`
- `RedisDel`
- `RedisHSetObj`
- `RedisHGetObj`
- `RedisIncr`

`RedisHSetObj()` 和 `RedisHGetObj()` 用反射把 struct 存到 Redis hash。它会跳过 `gorm.DeletedAt`，并处理 string/int/bool/pointer。

### Go 学习点

- `context.WithTimeout()` 控制 Redis ping 超时。
- 反射：`reflect.ValueOf(obj).Elem()` 遍历结构体字段。
- Redis pipeline：`TxPipeline()` 把多个命令组合执行。
- `errors.Is(err, redis.Nil)` 用于判断 key 不存在。

## 八、HybridCache：Redis/内存双模式缓存

`pkg/cachex/hybrid_cache.go` 提供泛型缓存：

```go
type HybridCache[V any] struct {
    ns Namespace
    redis *redis.Client
    redisCodec ValueCodec[V]
    redisEnabled func() bool
    memOnce sync.Once
    memInit func() *hot.HotCache[string, V]
    mem *hot.HotCache[string, V]
}
```

策略：

```text
if Redis 可用:
  用 Redis get/set/scan/unlink
else:
  用 samber/hot 内存 LRU cache
```

`Namespace` 负责给 key 加前缀，避免不同业务缓存冲突：

```text
Namespace("subscription_plan:v1").FullKey("123")
  -> subscription_plan:v1:123
```

`ValueCodec` 负责值编码：

- `IntCodec`
- `StringCodec`
- `JSONCodec[V]`

在订阅套餐缓存里可以看到典型使用：Redis 开启时多个节点共享缓存；Redis 关闭时每个进程使用本地 hot cache。

### Go 学习点

- 泛型：`HybridCache[V any]`。
- 懒初始化：`sync.Once` 保证内存 cache 只创建一次。
- 策略注入：`RedisEnabled func() bool` 让缓存运行时判断 Redis 是否可用。
- 编码接口：`ValueCodec[V]` 把缓存容器和序列化方式解耦。

## 九、RWMap：并发安全泛型 map

`types/rw_map.go` 是项目里最适合学习 Go 泛型和锁的文件之一。

```go
type RWMap[K comparable, V any] struct {
    data map[K]V
    mutex sync.RWMutex
}
```

常用方法：

- `Get`
- `Set`
- `AddAll`
- `Clear`
- `ReadAll`
- `Len`
- `MarshalJSON`
- `UnmarshalJSON`
- `MarshalJSONString`

`ReadAll()` 返回 map 副本，而不是直接返回内部 map。这很重要：如果把内部 map 暴露出去，调用方就可以绕过锁修改数据，锁就失效了。

`ratio_setting` 中大量倍率配置都用 `RWMap` 保存，例如 model ratio、group ratio、cache ratio。

### Go 学习点

- `K comparable` 表示 key 类型必须可比较，才能作为 map key。
- `V any` 表示 value 可以是任意类型。
- `RLock` 用于并发读，`Lock` 用于写。
- 自定义 `MarshalJSON` / `UnmarshalJSON` 让类型能自然参与 JSON 序列化。

## 十、HTTP client：上游请求的统一出口

`service/http_client.go` 初始化全局上游 HTTP client。

`InitHttpClient()` 配置：

- `MaxIdleConns`
- `MaxIdleConnsPerHost`
- `IdleConnTimeout`
- `ForceAttemptHTTP2`
- 环境代理 `http.ProxyFromEnvironment`
- 可选 `TLSInsecureSkipVerify`
- 可选 `Timeout`
- `CheckRedirect`

`CheckRedirect` 会用 fetch setting 做 SSRF 防护，避免重定向到不允许的域名、IP、端口或内网地址。

代理 client 支持：

- http
- https
- socks5
- socks5h

`NewProxyHttpClient(proxyURL)` 会缓存同一个 proxy URL 的 client，避免每次请求都重新创建 transport。`ResetProxyClientCache()` 会关闭旧 transport 的 idle connections 并清空缓存。

### Go 学习点

- `http.Transport` 是连接池和网络行为的核心。
- `CheckRedirect` 可以拦截重定向。
- SOCKS5 代理通过自定义 `DialContext` 接入。
- map 缓存 proxy client 需要 mutex 保护。

## 十一、这些基础设施如何串到业务链路

### 11.1 relay retry 和 BodyStorage

```text
middleware.Distribute()
  -> common.GetBodyStorage()
  -> 读取 model/group 后 Seek 回开头

controller.Relay()
  -> for retry:
     -> common.GetBodyStorage()
     -> c.Request.Body = io.NopCloser(bodyStorage)
     -> relay helper 重新解析或透传 body

middleware.BodyStorageCleanup()
  -> 请求结束释放 BodyStorage
```

如果没有 BodyStorage，retry 第二次发请求时 body 已经被第一次读空。

### 11.2 配置热更新和 RWMap

```text
model.SyncOptions()
  -> loadOptionsFromDatabase()
  -> updateOptionMap()
  -> ratio_setting.UpdateModelRatioByJSONString()
  -> types.LoadFromJsonString(RWMap, json)
```

倍率 map 被热更新时，读请求仍可能并发发生，所以需要 `RWMap` 的锁。

### 11.3 订阅套餐缓存和 HybridCache

```text
getSubscriptionPlanCache()
  -> cachex.NewHybridCache[SubscriptionPlan]()
  -> Redis enabled ? Redis : memory hot cache
```

这样代码只调用 `cache.Get()` / `cache.SetWithTTL()`，不用在业务逻辑里反复判断 Redis 是否启用。

### 11.4 上游 HTTP 请求和 http client

```text
channel.DoApiRequest()
  -> doRequest()
     -> if channel proxy: service.NewProxyHttpClient()
     -> else service.GetHttpClient()
     -> client.Do(req)
```

上游请求走同一套 client 配置，统一处理连接池、超时、SSRF redirect 检查和代理。

## 十二、常见误解

| 误解 | 正确理解 |
| --- | --- |
| 请求体可以随便读很多次 | 原始 `c.Request.Body` 只能读一次，项目靠 BodyStorage 复读 |
| 磁盘缓存文件会长期存在 | 它是请求临时文件，请求结束和启动清理都会删除旧文件 |
| Redis 开启后所有缓存都走 Redis | 有些缓存仍是进程内，HybridCache 才是 Redis/内存双模式 |
| 返回内部 map 更省事 | 会破坏锁保护，`ReadAll()` 必须返回副本 |
| proxy client 每次都创建也没事 | 会浪费连接池，项目按 proxy URL 缓存 client |
| `ReaderOnly` 没必要 | 它防止 `http.NewRequest` 或后续逻辑关闭底层 BodyStorage |
| `ApiError` 一定返回 HTTP 4xx | 后台 API 常返回 HTTP 200 + `success:false` |

## 十三、练习题

1. 从 `middleware.Distribute()` 找一次 `common.GetBodyStorage()` 的使用，说明为什么读完后还能给 controller 再读。
2. 从 `controller.Relay()` 的 retry loop 找 `c.Request.Body = io.NopCloser(bodyStorage)`，说明它解决了什么问题。
3. 打开 `common/body_storage.go`，比较 `memoryStorage.Close()` 和 `diskStorage.Close()` 的副作用。
4. 打开 `common/disk_cache_config.go`，说明哪些字段用 mutex，哪些统计用 atomic。
5. 找一个使用 `types.RWMap` 的 ratio setting，说明配置热更新时如何避免并发 map 写 panic。
6. 打开 `pkg/cachex/hybrid_cache.go`，说明 Redis 关闭时缓存如何工作。
7. 打开 `service/http_client.go`，说明重定向为什么也要做 SSRF 检查。
8. 找一个新增业务 JSON 解析位置，判断它是否应该使用 `common.DecodeJson` 或 `common.Unmarshal`。
