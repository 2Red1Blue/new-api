# 登录扩展与安全边界学习指南

这份文档梳理 new-api 中主登录链路之外的安全能力：OAuth、自定义 OAuth、Passkey/WebAuthn、2FA、安全二次验证、Turnstile、限流、模型请求限流、敏感词检查、细粒度权限和后端 i18n。它适合和 `auth-token-quota-guide-for-go-learners.md` 一起读。

## 一、总览

new-api 的安全体系可以分成几层：

```text
入口层
  -> Turnstile
  -> 全局/关键接口限流
  -> body size limit

身份层
  -> Cookie Session
  -> 用户 access token
  -> API Key TokenAuth
  -> OAuth / Passkey / 2FA

权限层
  -> UserAuth / AdminAuth / RootAuth
  -> authz.RequirePermission
  -> 安全二次验证

请求保护层
  -> ModelRequestRateLimit
  -> sensitive words check
  -> request id / audit log
```

学习时不要只盯住 `controller.Login`。真实项目的安全边界往往分散在 router、中间件、controller、service、model 和 session 里。

## 二、相关路由入口

主要路由在 `router/api-router.go`。

OAuth：

```text
GET  /api/oauth/state
GET  /api/oauth/:provider
POST /api/oauth/email/bind
GET  /api/oauth/wechat
POST /api/oauth/wechat/bind
GET  /api/oauth/telegram/login
GET  /api/oauth/telegram/bind
```

Passkey：

```text
POST /api/user/passkey/login/begin
POST /api/user/passkey/login/finish
GET    /api/user/self/passkey
POST   /api/user/self/passkey/register/begin
POST   /api/user/self/passkey/register/finish
POST   /api/user/self/passkey/verify/begin
POST   /api/user/self/passkey/verify/finish
DELETE /api/user/self/passkey
DELETE /api/admin/user/:id/reset_passkey
```

2FA：

```text
POST /api/user/login/2fa
GET  /api/user/self/2fa/status
POST /api/user/self/2fa/setup
POST /api/user/self/2fa/enable
POST /api/user/self/2fa/disable
POST /api/user/self/2fa/backup_codes
GET    /api/admin/user/2fa/stats
DELETE /api/admin/user/:id/2fa
```

通用安全验证：

```text
POST /api/verify
```

这些接口通常会叠加：

- `CriticalRateLimit()`
- `TurnstileCheck()`
- `UserAuth()`
- `AdminAuth()`
- body limit

## 三、OAuth provider 注册

OAuth provider 接口在 `oauth/provider.go`：

```go
type Provider interface {
    GetName() string
    IsEnabled() bool
    ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error)
    GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error)
    IsUserIDTaken(providerUserID string) bool
    FillUserByProviderID(user *model.User, providerUserID string) error
    SetProviderUserID(user *model.User, providerUserID string)
    GetProviderPrefix() string
}
```

注册表在 `oauth/registry.go`：

```text
Register(name, provider)
RegisterCustom(name, provider)
Unregister(name)
GetProvider(name)
LoadCustomProviders()
ReloadCustomProviders()
```

它用：

```text
map[string]Provider + sync.RWMutex
```

保护 provider registry。

内置 provider 通过包初始化注册。`router/api-router.go` 里有 blank import：

```go
_ "github.com/QuantumNous/new-api/oauth"
```

这保证 OAuth 包的 `init()` 会执行，内置 provider 会进入 registry。

自定义 OAuth provider 从 DB 加载：

```text
oauth.LoadCustomProviders()
  -> model.GetAllCustomOAuthProviders()
  -> oauth.NewGenericOAuthProvider(config)
  -> RegisterCustom(slug, provider)
```

## 四、OAuth 登录流程

标准 OAuth 回调入口：

```text
controller.HandleOAuth
```

流程：

```text
GET /api/oauth/state
  -> GenerateOAuthCode
  -> 生成 random state
  -> 写 session oauth_state

OAuth provider callback
  -> HandleOAuth
  -> 读取 provider
  -> 校验 state
  -> 如果当前 session 已登录：绑定 OAuth
  -> 检查 provider enabled
  -> ExchangeToken
  -> GetUserInfo
  -> findOrCreateOAuthUser
  -> 检查 user status
  -> setupLogin
```

`state` 是 OAuth CSRF 防护的关键。没有它，攻击者可能把自己的第三方账号绑定或登录到受害者 session。

## 五、OAuth 绑定和创建用户

如果用户已经登录，再访问 OAuth callback，会进入绑定流程：

```text
handleOAuthBind
```

它会：

1. 交换 token。
2. 获取 provider user info。
3. 检查 provider user id 是否已被绑定。
4. 如果是自定义 provider，写 `user_oauth_bindings`。
5. 如果是内置 provider，更新 users 表对应字段。

未登录时，`findOrCreateOAuthUser` 会：

- 先用 provider user id 查已有用户。
- 兼容 GitHub legacy id 迁移。
- 如果用户不存在且注册关闭，返回注册禁用错误。
- 如果允许注册，则创建新用户。
- 设置用户名、显示名、邮箱、邀请关系、初始额度等。

这里能学到一个真实项目常见模式：第三方账号绑定既要支持新用户注册，又要支持老用户迁移，还要防止重复绑定。

## 六、Passkey / WebAuthn

Passkey 控制器在：

```text
controller/passkey.go
```

WebAuthn 服务在：

```text
service/passkey/service.go
service/passkey/session.go
service/passkey/user.go
```

凭证模型在：

```text
model/passkey.go
```

Passkey 有三类流程：

```text
注册 register
登录 login
安全验证 verify
```

### 1. BuildWebAuthn

`service/passkey.BuildWebAuthn(r)` 根据系统设置和请求构造 WebAuthn 实例：

- RP display name。
- RP origins。
- RP ID。
- authenticator selection。
- user verification。
- attachment preference。
- timeout。
- debug。

Origin 推导会读取：

- 显式配置的 origins。
- `X-Forwarded-Proto`。
- TLS 状态。
- request host。
- `system_setting.ServerAddress`。

默认不允许非 HTTPS origin，除非是 localhost/127.0.0.1 或管理员开启不安全 origin。这是 WebAuthn 的核心安全约束。

### 2. Passkey 注册

注册流程：

```text
PasskeyRegisterBegin
  -> 用户必须已登录
  -> 检查管理员是否启用 Passkey
  -> 如果用户启用 2FA，要求先通过安全验证
  -> BuildWebAuthn
  -> BeginRegistration
  -> SaveSessionData(passkey_registration_session)
  -> 返回 options

PasskeyRegisterFinish
  -> PopSessionData
  -> FinishRegistration
  -> NewPasskeyCredentialFromWebAuthn
  -> UpsertPasskeyCredential
  -> 记录安全审计
```

begin/finish 两阶段是 WebAuthn 的标准形态。begin 生成 challenge 并保存 session，finish 必须拿同一个 session data 验证浏览器返回。

### 3. Passkey 登录

登录流程：

```text
PasskeyLoginBegin
  -> BeginDiscoverableLogin
  -> SaveSessionData(passkey_login_session)
  -> 返回 assertion options

PasskeyLoginFinish
  -> PopSessionData
  -> 通过 credential id 找 PasskeyCredential
  -> 加载用户并检查状态
  -> 校验 userHandle 与用户 id
  -> FinishPasskeyLogin
  -> 更新 LastUsedAt
  -> setupLogin
```

这里的 discoverable login 允许不先输入用户名，浏览器凭证本身能找到用户。

### 4. Passkey 安全验证

安全验证流程：

```text
PasskeyVerifyBegin
  -> 用户已登录
  -> 查用户 passkey
  -> BeginLogin
  -> SaveSessionData(passkey_verify_session)

PasskeyVerifyFinish
  -> PopSessionData
  -> FinishLogin
  -> 更新 LastUsedAt
  -> session.Set("secure_passkey_ready_at")
```

注意：`PasskeyVerifyFinish` 只写入一个短期 ready 标记，不直接写完整安全验证状态。最终需要 `/api/verify` 消费这个 ready 标记。

## 七、2FA / TOTP

2FA 控制器在：

```text
controller/twofa.go
```

模型包括：

- `TwoFA`
- backup codes

### 1. 设置 2FA

```text
Setup2FA
  -> 检查是否已启用
  -> 删除旧的 disabled 记录
  -> GenerateTOTPSecret
  -> GenerateBackupCodes
  -> GenerateQRCodeData
  -> 创建 TwoFA(IsEnabled=false)
  -> 保存备用码
```

这一步只是初始化，不代表已经启用。

### 2. 启用 2FA

```text
Enable2FA
  -> 读取用户输入 code
  -> ValidateNumericCode
  -> ValidateTOTPCode
  -> twoFA.Enable()
```

只有用户能正确输入 authenticator 里的 TOTP，才真正启用。

### 3. 登录 2FA 分支

`controller.Login` 在用户密码正确后，如果发现用户启用 2FA：

```text
session.Set("pending_username")
session.Set("pending_user_id")
返回 requires_2fa
```

然后前端调用：

```text
POST /api/user/login/2fa
  -> Verify2FALogin
  -> 校验 TOTP 或备用码
  -> setupLogin
```

### 4. 禁用和备用码

禁用 2FA 要求用户提供 TOTP 或备用码。重新生成备用码也需要已登录并通过当前 2FA 保护。

备用码通常是一次性使用，验证成功后要更新使用状态，避免重复使用。

## 八、通用安全验证

有些敏感操作不应该只靠“已登录”，还需要 step-up verification。入口：

```text
POST /api/verify
  -> controller.UniversalVerify
```

支持两种方式：

```text
2fa
passkey
```

2FA 路径：

```text
用户已登录
  -> 读取 code
  -> validateTwoFactorAuth
  -> setSecureVerificationSession
```

Passkey 路径：

```text
PasskeyVerifyFinish 写 secure_passkey_ready_at
  -> UniversalVerify(method=passkey)
  -> consumePasskeyReady
  -> setSecureVerificationSession
```

安全验证 session key：

```text
secure_verified_at
secure_verified_method
```

有效期：

```text
SecureVerificationTimeout = 300 秒
```

Passkey ready 标记有效期：

```text
PasskeyReadyTimeout = 60 秒
```

中间件：

```text
middleware.SecureVerificationRequired()
middleware.OptionalSecureVerification()
```

它们用于要求或识别用户是否刚刚通过了安全验证。

## 九、Turnstile

中间件：

```text
middleware.TurnstileCheck()
```

它保护注册、登录、发送验证邮件、签到等入口。

流程：

```text
如果 TurnstileCheckEnabled=false
  -> 直接放行

如果 session 中已有 turnstile=true
  -> 放行

否则读取 query: turnstile
  -> POST 到 Cloudflare siteverify
  -> secret = TurnstileSecretKey
  -> response = token
  -> remoteip = c.ClientIP()
  -> 成功后 session.Set("turnstile", true)
```

这里的设计让用户通过一次 Turnstile 后，在同一 session 中不用每个相关接口都重复验证。

## 十、普通限流

中间件：

```text
middleware.GlobalWebRateLimit()
middleware.GlobalAPIRateLimit()
middleware.CriticalRateLimit()
middleware.DownloadRateLimit()
middleware.UploadRateLimit()
middleware.SearchRateLimit()
middleware.EmailVerificationRateLimit()
```

底层有两种实现：

```text
Redis enabled
  -> Redis list + expire

Redis disabled
  -> common.InMemoryRateLimiter
```

全局/关键接口限流默认按客户端 IP。`SearchRateLimit` 这类接口按认证后的 user id 限流，减少代理轮换绕过。

`CriticalRateLimit` 用在登录、注册、OAuth、支付、Passkey begin/finish 等高风险接口。

## 十一、模型请求限流

relay 模型请求限流在：

```text
middleware.ModelRequestRateLimit()
```

挂载位置：

```text
router/relay-router.go
  -> /v1
  -> Gemini relay
```

配置：

```text
ModelRequestRateLimitEnabled
ModelRequestRateLimitCount
ModelRequestRateLimitDurationMinutes
ModelRequestRateLimitSuccessCount
ModelRequestRateLimitGroup
```

它区分两种限制：

- total request limit：包括失败请求。
- success request limit：请求成功后才记录。

分组可以覆盖默认限制：

```text
group := token group 或 user group
setting.GetGroupRateLimit(group)
```

Redis 模式下，总请求限制使用 token bucket limiter；成功请求限制使用 Redis list。内存模式下使用 `InMemoryRateLimiter`。

## 十二、敏感词检查

配置在：

```text
setting/sensitive.go
```

核心开关：

```text
CheckSensitiveEnabled
CheckSensitiveOnPromptEnabled
StopOnSensitiveEnabled
SensitiveWords
```

服务在：

```text
service/sensitive.go
```

主要函数：

```text
CheckSensitiveText
CheckSensitiveMessages
SensitiveWordContains
SensitiveWordReplace
```

relay 主流程在 `controller/relay.go` 中：

```text
判断 setting.ShouldCheckPromptSensitive()
  -> 解析请求 meta / CombineText
  -> service.CheckSensitiveText
  -> 命中后返回 SensitiveWordsDetected
  -> MarkTiming("sensitive_check_done")
```

敏感词匹配使用 AC 自动机相关实现，适合大量关键词匹配。

## 十三、细粒度权限 authz

角色中间件只处理大级别：

```text
UserAuth
AdminAuth
RootAuth
```

更细粒度权限由 `service/authz` 管理，底层是 Casbin。

核心：

```text
authz.Init(model.DB)
authz.StartPolicySync(common.SyncFrequency)
authz.Can(userID, role, permission)
middleware.RequirePermission(permission)
```

权限 subject：

```text
user:<id>
role:<roleKey>
```

渠道相关权限是最典型的使用场景：

```text
ChannelRead
ChannelWrite
ChannelOperate
ChannelSensitiveWrite
```

`ChannelSensitiveWrite` 保护 key、base_url、header_override 等敏感字段。`controller/channel_authz.go` 还会比较 patch 中哪些字段变化，防止普通管理员修改敏感字段。

## 十四、后端 i18n

后端 i18n 初始化在 `InitResources()`：

```text
i18n.Init()
i18n.SetUserLangLoader(model.GetUserLanguage)
```

中间件：

```text
middleware.I18n()
```

controller / middleware 中常见写法：

```text
common.ApiErrorI18n(c, i18n.Msg...)
i18n.T(c, i18n.Msg...)
common.TranslateMessage(c, i18n.Msg...)
```

登录、OAuth、TokenAuth 等用户可见错误经常走 i18n。读代码时看到 `i18n.MsgOAuthStateInvalid` 这类常量，要跳到 `i18n/` 和 locale 文件看文案。

## 十五、Go 学习点

这一块源码适合学习：

1. OAuth `state` 如何防 CSRF。
2. provider registry 如何用 interface + map + RWMutex 扩展。
3. WebAuthn 为什么需要 begin/finish 两阶段。
4. session 如何保存 challenge、pending login、安全验证状态。
5. TOTP 和 backup code 如何配合。
6. step-up verification 如何保护敏感操作。
7. Redis 和内存限流如何抽象成同样的中间件。
8. token bucket 和滑动窗口计数的差异。
9. AC 自动机适合敏感词多模式匹配。
10. Casbin 如何做细粒度权限。
11. Go 包 `init()` 和 blank import 如何注册能力。

## 十六、阅读练习

### 练习 1：追 OAuth 登录

```text
GenerateOAuthCode
  -> session oauth_state
  -> HandleOAuth
  -> provider.ExchangeToken
  -> provider.GetUserInfo
  -> findOrCreateOAuthUser
  -> setupLogin
```

回答：如果没有 state 校验，会出现什么风险？

### 练习 2：追 Passkey 登录

```text
PasskeyLoginBegin
  -> SaveSessionData
  -> PasskeyLoginFinish
  -> GetPasskeyByCredentialID
  -> FinishPasskeyLogin
  -> setupLogin
```

回答：为什么 finish 时还要检查 userHandle？

### 练习 3：追一次安全二次验证

```text
PasskeyVerifyBegin/Finish
  -> secure_passkey_ready_at
  -> POST /api/verify method=passkey
  -> secure_verified_at
  -> SecureVerificationRequired
```

回答：为什么 PasskeyVerifyFinish 不直接设置 `secure_verified_at`？

### 练习 4：追模型限流

```text
TokenAuth
  -> context token/user group
  -> ModelRequestRateLimit
  -> Redis or memory limiter
  -> c.Next()
  -> 成功后记录 success count
```

回答：为什么要同时有 total count 和 success count？

