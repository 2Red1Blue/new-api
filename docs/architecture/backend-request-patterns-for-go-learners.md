# new-api 后台请求处理模式学习指南

这份文档专门讲普通后台 API，也就是 `/api/...` 这一类管理台、用户中心、配置、渠道、令牌接口。它和 relay 文档的区别是：

- relay 文档关注 `/v1/...` 如何代理 AI provider。
- 本文关注 `/api/...` 如何完成登录、鉴权、权限、参数校验、分页、CRUD、审计日志、统一响应和前端联动。

读完这份文档，你应该能独立看懂一个后台接口从 router 到 controller，再到 model/service 的完整链路。

## 一、后台 API 总入口

后台 API 路由入口是 `router/api-router.go` 的 `SetApiRouter()`。

整体结构：

```text
main.go
  -> router.SetRouter(server, assets)
     -> SetApiRouter(server)
        -> apiRouter := router.Group("/api")
        -> apiRouter.Use(RouteTag("api"))
        -> apiRouter.Use(gzip.Gzip(...))
        -> apiRouter.Use(BodyStorageCleanup())
        -> apiRouter.Use(GlobalAPIRateLimit())
        -> 注册匿名接口、用户接口、管理员接口、root 接口
```

`/api` 下的接口大致分四层：

| 层级 | 典型 middleware | 典型接口 |
| --- | --- | --- |
| 匿名 | 无登录，但常有限流/Turnstile/body limit | `/api/status`、`/api/user/login`、OAuth、支付 webhook |
| 用户 | `middleware.UserAuth()` | `/api/user/self`、`/api/token`、充值、订阅购买 |
| 管理员 | `middleware.AdminAuth()` | 用户管理、渠道管理、模型管理、兑换码 |
| Root | `middleware.RootAuth()` | 系统设置、性能、OAuth provider、敏感配置 |

渠道相关路由拆在 `router/channel-router.go`，因为它的权限更细：除了 `AdminAuth()`，还会按 `authz.ChannelRead`、`ChannelWrite`、`ChannelOperate`、`ChannelSensitiveWrite` 做细粒度授权。

## 二、统一响应格式

普通后台 API 大多使用 `common/gin.go` 里的响应 helper：

```go
func ApiError(c *gin.Context, err error) {
    c.JSON(http.StatusOK, gin.H{
        "success": false,
        "message": err.Error(),
    })
}

func ApiSuccess(c *gin.Context, data any) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data":    data,
    })
}
```

也就是说，很多后台业务失败不是 HTTP 4xx，而是：

```json
{
  "success": false,
  "message": "错误信息"
}
```

带 i18n 的版本是：

- `common.ApiErrorI18n(c, key, args...)`
- `common.ApiSuccessI18n(c, key, data, args...)`

前端 `web/default/src/lib/api.ts` 的 axios response interceptor 会按这个结构处理响应，所以新增后台接口时要优先使用这些 helper。

例外情况也存在：

- relay 接口返回 OpenAI/Claude/Gemini 兼容错误格式。
- 某些兼容外部协议的接口会返回非 `{ success, message, data }`。
- 鉴权失败有时会返回 401/403。

## 三、后台鉴权：session、access token 和 New-Api-User

后台鉴权核心在 `middleware/auth.go` 的 `authHelper(c, minRole)`。

### 3.1 session 登录态

用户登录成功后，`controller/user.go` 的 `setupLogin()` 会写 session：

```text
session.Set("id", user.Id)
session.Set("username", user.Username)
session.Set("role", user.Role)
session.Set("status", user.Status)
session.Set("group", user.Group)
session.Save()
```

之后 `UserAuth()`、`AdminAuth()`、`RootAuth()` 都会从 session 读取这些字段。

### 3.2 access token 兜底

如果 session 里没有 username，`authHelper()` 会尝试从 `Authorization` header 读取后台 access token，并调用 `model.ValidateAccessToken()`。

注意这里的 access token 是 `User.AccessToken`，用于后台管理 API；不要和 relay 的 `Token.Key` 混淆。

### 3.3 New-Api-User 防串号

`authHelper()` 还会检查请求头 `New-Api-User`：

```text
New-Api-User header
  -> strconv.Atoi()
  -> 必须等于 session/access token 中的 user id
```

这是为了防止不同版本或不同窗口的前端状态冲突，把请求发到错误用户身份下。

### 3.4 角色门槛

三个常用 middleware 只是传入不同 `minRole`：

```go
func UserAuth() func(c *gin.Context)  { authHelper(c, common.RoleCommonUser) }
func AdminAuth() func(c *gin.Context) { authHelper(c, common.RoleAdminUser) }
func RootAuth() func(c *gin.Context)  { authHelper(c, common.RoleRootUser) }
```

鉴权成功后会写入 Gin context：

- `username`
- `role`
- `id`
- `group`
- `user_group`
- `use_access_token`

controller 后续通过 `c.GetInt("id")`、`c.GetInt("role")`、`c.GetString("group")` 读取。

## 四、权限：Role 之外的细粒度授权

角色只解决“普通用户/管理员/root”的大门槛。部分后台功能还用 `service/authz` 做细粒度权限。

中间件是 `middleware.RequirePermission(permission)`：

```text
role := c.GetInt("role")
userID := c.GetInt("id")
if authz.Can(userID, role, permission) {
  c.Next()
} else {
  403 insufficient privilege
}
```

典型使用场景是渠道管理：

- 读渠道：`authz.ChannelRead`
- 写渠道：`authz.ChannelWrite`
- 操作渠道：`authz.ChannelOperate`
- 写敏感字段：`authz.ChannelSensitiveWrite`

这也是为什么“管理员”并不天然等于“能看所有渠道 key”。真实 key、上游密码等敏感信息还会叠加 root 和安全验证。

## 五、管理操作审计

管理/root 写操作会自动留审计日志。入口仍然在 `authHelper()`：

```text
authHelper(minRole >= admin)
  -> beginAdminAudit(c)
     -> 包装 gin.ResponseWriter
  -> c.Next()
  -> finishAdminAudit(c, writer)
     -> 如果 handler 没有手动记录审计
     -> 推断 success
     -> model.RecordOperationAuditLog()
```

`middleware/audit.go` 的 `auditResponseWriter` 会复制一份有限大小的响应体，用于判断 `{ success: false }` 这种 HTTP 200 的业务失败。

审计日志内容分两层：

- `Other.op`：语言无关 action 和 params，前端可 i18n 展示。
- `Other.admin_info` / `audit_info`：管理员身份、路由、方法、状态等内部信息。

普通用户查询日志时，`model/log.go` 的 `formatUserLogs()` 会移除 `admin_info`、`audit_info`、`stream_status`。

## 六、Controller 的常见写法

普通后台 controller 大多遵循这个模板：

```text
func SomeHandler(c *gin.Context) {
  1. 从 path/query/body/session/context 取参数
  2. 参数校验和权限校验
  3. 调 model/service 执行业务
  4. 必要时清缓存或记录日志
  5. common.ApiSuccess / ApiError 返回
}
```

### 6.1 参数来源

| 来源 | 代码 | 场景 |
| --- | --- | --- |
| path | `c.Param("id")` | `/api/token/:id` |
| query | `c.Query("keyword")` | 搜索、分页、过滤 |
| body | `c.ShouldBindJSON(&req)` | 创建/更新 |
| session/context | `c.GetInt("id")` | 当前用户 |
| header | `c.GetHeader(...)` | 鉴权、特殊协议 |

项目新代码中，JSON marshal/unmarshal 应使用 `common.Marshal`、`common.Unmarshal`、`common.DecodeJson` 等 wrapper。源码里有少数历史代码直接用标准库 decoder，读到时要知道这是既有实现，不是新增代码的推荐写法。

### 6.2 分页模式

分页 helper 是 `common.GetPageQuery(c)`，典型用法见 `controller/token.go`：

```text
pageInfo := common.GetPageQuery(c)
items, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
total, _ := model.CountUserTokens(userId)
pageInfo.SetTotal(int(total))
pageInfo.SetItems(items)
common.ApiSuccess(c, pageInfo)
```

前端会收到：

```json
{
  "success": true,
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "page_size": 10
  }
}
```

### 6.3 敏感字段裁剪

后台接口经常不会直接返回完整 DB model。

Token 列表用 `buildMaskedTokenResponse()`：

```text
token copy
  -> Key = token.GetMaskedKey()
  -> 返回 masked token
```

真实 token key 只能通过 `GetTokenKey()` 单独获取。渠道列表类似，会隐藏 `Channel.Key`，真实 key 要走专门接口并叠加权限。

这个模式非常重要：新增列表接口时，先问“这个 model 里有没有 password/key/secret/private data 一类字段”。

## 七、示例一：登录链路

登录接口注册在 `router/api-router.go`：

```text
POST /api/user/login
  -> CriticalRateLimit()
  -> AnonymousRequestBodyLimit()
  -> TurnstileCheck()
  -> controller.Login()
```

`controller.Login()` 做这些事：

```text
1. 检查 PasswordLoginEnabled
2. 解析 username/password
3. model.User.ValidateAndFill()
4. 如果启用 2FA:
   -> 写 pending session
   -> 返回 require_2fa
5. 否则 setupLogin()
   -> UpdateUserLastLoginAt()
   -> 写 session
   -> recordLoginAudit()
   -> 返回用户基础信息
```

Go 学习点：

- early return：每个失败条件都立即返回。
- `errors.Is`：区分数据库错误、空凭证、用户名密码错误。
- session：`sessions.Default(c)` 读写 cookie session。
- 审计：成功登录会写 `LogTypeLogin`。

## 八、示例二：Token 管理链路

Token 路由在 `router/token-router.go` 或 `router/api-router.go` 的 token group 中注册，所有用户都可以管理自己的 token。

### 8.1 列表和搜索

`controller.GetAllTokens()`：

```text
userId := c.GetInt("id")
pageInfo := common.GetPageQuery(c)
model.GetAllUserTokens(userId, start, size)
model.CountUserTokens(userId)
buildMaskedTokenResponses(tokens)
common.ApiSuccess(c, pageInfo)
```

`controller.SearchTokens()` 读取 `keyword` 和 `token` query，调用 `model.SearchUserTokens()`。model 层会清洗 LIKE 模式并限制大用户模糊搜索，防止慢查询和滥用。

### 8.2 创建 token

`controller.AddToken()`：

```text
c.ShouldBindJSON(&token)
校验 name 长度
校验 remain_quota 范围
检查用户 token 数量是否超过 MaxUserTokens
common.GenerateKey()
构造 cleanToken
cleanToken.Insert()
返回 success
```

为什么要构造 `cleanToken`，而不是直接保存用户传来的 `token`？因为服务端必须控制这些字段：

- `UserId` 来自当前登录用户。
- `Key` 由服务端生成。
- `CreatedTime` / `AccessedTime` 由服务端生成。

这是防止客户端伪造字段的常见写法。

### 8.3 更新 token

`controller.UpdateToken()` 先读用户提交的 token，再通过 `model.GetTokenByIds(token.Id, userId)` 查出当前用户自己的 token。这样可以避免用户更新别人的 token。

更新时还有两个状态保护：

- 已过期 token 不能重新启用。
- 额度耗尽且非 unlimited 的 token 不能重新启用。

`status_only` query 存在时只更新状态，否则更新完整配置字段。

Go 学习点：

- copy-and-clean：不要信任客户端传来的 owner/key/time 等字段。
- owner check：通过 `id + userId` 查询保证资源归属。
- 状态保护：业务状态不能只靠前端按钮控制，后端必须再检查。

## 九、示例三：用户管理链路

用户管理在 `/api/user` admin route 下，挂 `middleware.AdminAuth()`。

常见接口：

- `GetAllUsers()`：分页列出用户。
- `SearchUsers()`：搜索用户。
- `CreateUser()`：管理员创建用户。
- `UpdateUser()`：更新用户字段和权限。
- `ManageUser()`：启用、禁用、删除、升降级、加减额度。

### 9.1 权限边界

用户管理里经常会看到这些规则：

- 管理员不能操作同级或更高级用户。
- 不能删除 root。
- 普通管理员不能提升别人到 root。
- 修改管理员权限时要走 `service/authz`。

读 `controller/user.go` 时，可以把它分成三类逻辑：

1. 当前操作者是谁：`c.GetInt("id")`、`c.GetInt("role")`
2. 目标用户是谁：`model.GetUserById()`
3. 这次操作是否合法：角色、状态、动作、额度校验

### 9.2 ManageUser 是动作路由

`ManageUser()` 不是单一 CRUD，而是根据 action 做分支：

```text
enable / disable
promote / demote
delete
add_quota / subtract_quota
```

这种 handler 的读法是先找到 action switch，再逐个分支看每个分支操作了哪些 model 函数、清理了哪些缓存、记录了哪些日志。

Go 学习点：

- 大 switch 的读法：先读分支名，再读每个分支的前置校验和副作用。
- 权限校验不能只看 route middleware，handler 内还有对象级权限。
- 修改用户/权限后要清用户缓存和 token 缓存。

## 十、示例四：渠道管理链路

渠道管理代码量大，主要在：

- `router/channel-router.go`
- `controller/channel.go`
- `model/channel.go`
- `model/ability.go`
- `service/channel.go`
- `service/channel_select.go`

### 10.1 列表和搜索

渠道列表不会直接返回 key。controller 查询时会 `Omit("key")` 或在响应前隐藏敏感字段。

搜索接口通常支持：

- 关键词
- 分组
- 状态
- 标签
- 类型
- 排序

`model.ChannelSortOptions` 是一个值得学习的小对象：它把前端传来的 sort 字段先白名单化，再应用到 GORM query，避免任意 order by 注入。

### 10.2 新增渠道

`AddChannel()` 支持多种模式：

- 单渠道
- 批量渠道
- 多 key 合并成单渠道

新增后的关键副作用是同步 ability：

```text
Channel.Insert()
  -> DB.Create(channel)
  -> AddAbilities()
  -> InitChannelCache()
  -> InvalidatePricingCache()
```

具体函数可能分散在 model/service 中，但你要抓住结果：渠道配置变化后，必须让选渠缓存和定价缓存知道。

### 10.3 更新渠道

`UpdateChannel()` 需要额外小心：

- 不能随便覆盖只读字段。
- 敏感字段需要更高权限。
- `ChannelInfo` 多 key 状态不能被普通更新意外清空。
- key 的追加/覆盖有不同模式。
- 更新后要同步 abilities/cache/pricing。

### 10.4 渠道状态

`UpdateChannelStatus()` 只允许启用或手动禁用。运行时的自动禁用在 `service.DisableChannel()`，比如上游余额不足、错误率过高、配置的自动禁用状态码命中等。

Go 学习点：

- 白名单排序：`ChannelSortOptions`。
- 大型 controller 读法：先看请求 DTO 和 mode，再看副作用。
- 敏感字段保护：route 权限 + handler 内字段级权限。
- 缓存一致性：改渠道后要同步 ability/channel cache/pricing cache。

## 十一、示例五：Option 配置更新

系统配置接口挂 `RootAuth()`：

```text
optionRoute := apiRouter.Group("/option")
optionRoute.Use(middleware.RootAuth())
```

`controller.UpdateOption()` 接收 key/value，做一些特殊配置保护，然后调用：

```text
model.UpdateOption(key, value)
  -> DB.FirstOrCreate()
  -> DB.Save()
  -> updateOptionMap(key, value)
```

`updateOptionMap()` 会把字符串写入运行时全局变量，例如：

- `common.*`
- `setting.*`
- `ratio_setting.*`
- `billing_setting.*`
- `performance_setting.*`

多节点同步靠 `main.go` 里的 `go model.SyncOptions(common.SyncFrequency)`。

Go 学习点：

- DB 配置和内存配置是两件事。
- 修改 DB 后要同步当前进程。
- 其他进程不是立即同步，而是周期拉取。

## 十二、Controller / Service / Model 怎么分工

实际代码并非每个接口都严格三层，但整体倾向如下：

| 层 | 应该做什么 | 例子 |
| --- | --- | --- |
| router | URL、middleware、权限门槛 | `/api/user/login` 挂限流和 Turnstile |
| controller | 参数、对象级权限、响应 | `AddToken()` 校验额度范围和 token 数量 |
| service | 跨模型业务、运行时逻辑 | `DisableChannel()`、`PreConsumeBilling()` |
| model | GORM 查询、事务、缓存读写 | `SearchUserTokens()`、`PreConsumeUserSubscription()` |
| common | 通用工具和基础设施 | response helper、JSON wrapper、Redis、env |

判断逻辑该放哪里，可以问三个问题：

1. 这个逻辑只和 HTTP 请求参数/响应有关吗？放 controller。
2. 这个逻辑跨多个 model 或和运行时策略有关吗？放 service。
3. 这个逻辑是某张表的查询、事务、缓存一致性吗？放 model。

## 十三、新增后台接口的源码路线

如果你要读懂或新增一个后台接口，可以按这个顺序：

1. 在 `router/api-router.go` 或 `router/channel-router.go` 找路由。
2. 看挂了哪些 middleware：匿名、UserAuth、AdminAuth、RootAuth、RequirePermission、CriticalRateLimit。
3. 进入 controller，标出参数来源：path/query/body/context/header。
4. 找 owner check：当前用户是否只能操作自己的资源。
5. 找对象级权限：是否不能操作同级/更高级/敏感字段。
6. 找 service/model 调用。
7. 找缓存失效：用户、token、channel、pricing、option。
8. 找审计日志：handler 是否手动埋点，或依赖 admin audit 兜底。
9. 看返回结构是否符合 `{ success, message, data }`。
10. 如果前端调用它，再到 `web/default/src/features/*/api.ts` 看类型和 transform。

## 十四、常见坑

| 坑 | 正确理解 |
| --- | --- |
| 看到 HTTP 200 就以为成功 | 后台 API 业务失败也常返回 HTTP 200 + `success:false` |
| `UserAuth()` 只看 session | session 不存在时还可能走 access token |
| 管理员一定能看敏感字段 | 敏感字段可能还需要 root、安全验证、细粒度权限 |
| 前端传什么就保存什么 | controller 常构造 clean 对象，只保留允许客户端控制的字段 |
| 列表接口可以直接返回 model | 要先检查 key/password/secret/private data |
| route middleware 足够保护对象 | handler 内还要检查目标资源归属和角色级别 |
| 更新 DB 配置后就完了 | 当前进程要 updateOptionMap，其他进程靠 SyncOptions |
| 改渠道只改 channels 表 | 还要同步 abilities、channel cache、pricing cache |
| 新增写接口不用管审计 | AdminAuth/RootAuth 会兜底，但重要操作最好手动记录更精确日志 |

## 十五、练习题

1. 从 `POST /api/user/login` 开始，画出登录成功和需要 2FA 的两条路径。
2. 从 `GET /api/token/` 开始，说明 token key 是在哪里被 mask 的。
3. 从 `PUT /api/token/` 开始，找出为什么过期 token 不能重新启用。
4. 从 `POST /api/user/manage` 开始，列出每个 action 修改了哪些字段。
5. 从 `POST /api/channel` 开始，跟到 ability 和 channel cache 的更新。
6. 从 `PUT /api/option` 开始，说明 DB option 如何变成运行时全局变量。
7. 找一个 AdminAuth 写接口，说明它如果没有手动记录日志，会怎样被 audit middleware 兜底。
8. 找一个列表接口，检查它有没有隐藏敏感字段。
