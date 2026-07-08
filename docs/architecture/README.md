# new-api 源码学习文档导航

本目录用于沉淀 new-api 源码学习和架构梳理文档。

## 推荐阅读顺序

1. `go-source-learning-guide.md`

   面向已经掌握 Go 基本语法、希望一边学习 Go 一边读 new-api 源码的读者。按 Go 语言点组织，每章都包含源码入口、读法和练习。

2. `package-study-map-for-go-learners.md`

   面向希望按 Go package/目录逐步推进的读者。把 common、model、middleware、controller、service、relay、pkg 等目录映射到学习重点、阅读任务和练习。

3. `data-model-and-state-guide-for-go-learners.md`

   面向希望先掌握核心数据对象和状态流的读者。梳理 User、Token、Channel、Ability、Option、Log、Task、SystemTask、Subscription 等 model 及它们在请求、计费、后台任务中的流转。

4. `source-walkthroughs-for-go-learners.md`

   面向希望按真实业务链路精读源码的读者。覆盖登录、API Key relay、渠道选择、OpenAI 非流式文本、上游 HTTP 请求、文本计费结算、系统任务等 walkthrough。

5. `backend-request-patterns-for-go-learners.md`

   面向希望读懂普通后台 API 的读者。梳理 `/api/...` 从 router、middleware、controller、service 到 model 的处理模式，覆盖 session/access token 鉴权、权限、分页、统一响应、审计日志、用户/令牌/渠道/配置管理。

6. `auth-token-quota-guide-for-go-learners.md`

   面向希望掌握认证、API Key 和额度扣费主链路的读者。梳理 Cookie Session、用户 access token、`TokenAuth`、`TokenAuthReadOnly`、API Key context、钱包 quota、预扣费、后结算、退款和消费日志。

7. `billing-expression-guide-for-go-learners.md`

   面向希望理解动态计费表达式的读者。梳理 `tiered_expr`、`pkg/billingexpr`、表达式变量、编译缓存、`BillingSnapshot`、预扣费、真实 usage 结算、token 归一化和日志展示。

8. `auth-extensions-security-guide-for-go-learners.md`

   面向希望理解登录扩展和安全边界的读者。梳理 OAuth、自定义 OAuth、Passkey/WebAuthn、2FA、安全二次验证、Turnstile、限流、敏感词、细粒度权限和后端 i18n。

9. `channel-management-selection-guide-for-go-learners.md`

   面向希望彻底掌握渠道系统的读者。梳理渠道 CRUD、Ability 索引、内存缓存、Distribute、优先级/权重选择、auto group、重试、模型 fallback、亲和性、RPM 限制、自动禁用和自动优先级。

10. `common-infrastructure-guide-for-go-learners.md`

   面向希望理解项目公共基础设施的读者。梳理 JSON wrapper、Gin helper、BodyStorage 请求体复读、磁盘缓存、Redis、HybridCache、RWMap、HTTP client、代理与资源清理。

11. `config-database-migration-guide-for-go-learners.md`

   面向希望理解项目启动、配置和持久化基础的读者。梳理 `InitResources`、DB/LogDB 选择、GORM 迁移、跨库兼容、`options` 表、`OptionMap`、分层配置注册、setting 热更新和多节点同步。

12. `system-tasks-observability-guide-for-go-learners.md`

   面向希望理解系统后台自运转能力的读者。梳理 system task runner、DB lease、任务生命周期、异步任务 CAS、日志、request id、RelayInfo timing、审计日志、系统状态、性能保护和缓存刷新。

13. `async-task-media-guide-for-go-learners.md`

   面向希望理解异步任务和媒体生成的读者。梳理 Midjourney、Suno、视频任务、`TaskAdaptor`、任务提交、fetch、system task 轮询、CAS 状态更新、失败退款和差额结算。

14. `logging-notification-audit-guide-for-go-learners.md`

   面向希望理解日志、审计、通知和 webhook 链路的读者。梳理 `model.Log`、request id、消费/错误/充值/登录/管理日志、审计兜底、日志查询统计、通知限流、Webhook 签名和 SSRF 防护。

15. `file-resource-body-guide-for-go-learners.md`

   面向希望理解请求体复读、multipart、多模态文件和资源清理的读者。梳理 `BodyStorage`、磁盘/内存缓存、`FileSource`、URL/base64 加载、token counter、provider adapter 文件转换和清理时机。

16. `payment-subscription-guide-for-go-learners.md`

   面向希望理解充值、支付、订阅和订阅额度如何接入计费的读者。梳理 TopUp、支付 webhook、订阅套餐、订阅订单、用户订阅、余额购买、在线支付、订阅预扣幂等、过期重置和 `BillingSession`。

17. `testing-and-debugging-guide-for-go-learners.md`

   面向希望通过测试和调试掌握项目行为的读者。梳理 Go/前端测试命令、本地 debug 环境、典型测试文件、Gin/httptest/DB fixture、relay 调试链路、request id、timing 和常见故障排查方法。

18. `frontend-default-guide-for-go-learners.md`

   面向希望把默认前端和 Go 后端连起来理解的读者。梳理 React/Rsbuild/TanStack Router/Query/Table、Zustand 登录态、Axios 请求封装、管理页数据流、DataTable 体系和 i18n。

19. `source-glossary-for-go-learners.md`

   面向读源码时需要快速查概念、类型、函数和字段的读者。把 `RelayInfo`、`BillingSession`、渠道选择、计费结算、任务系统等高频对象做成速查索引。

20. `provider-adapter-guide-for-go-learners.md`

   面向希望理解 provider 适配层的读者。梳理 `relay/channel/*` 的 `Adaptor` 接口、OpenAI/Claude/Gemini 转换、上游 HTTP 请求、header override、multipart、SSE 流式扫描和新增 provider 路线。

21. `relay-error-retry-streaming-guide-for-go-learners.md`

   面向希望按函数精读 relay 主流程的读者。梳理 `controller.Relay`、`RelayInfo`、请求解析、预扣费、重试循环、`NewAPIError`、渠道错误处理、状态码映射、SSE 扫描、stream status 和 usage 结算。

22. `source-deep-dives-for-go-learners.md`

   面向已经能跟源码跳转、希望分专题深挖的读者。覆盖启动初始化、配置/数据库/缓存/后台任务、relay/adaptor/重试、计费/tiered_expr/log、前端与后台管理 API。

23. `new-api-implementation-guide.md`

   面向希望系统理解 new-api 当前实现的读者。按项目模块和业务流程组织，覆盖启动、路由、认证、relay、渠道、计费、日志、异步任务、前端等重要逻辑。

24. `model-vendor-deployment-guide-for-go-learners.md`

   面向希望理解模型目录治理的读者。梳理模型元数据、供应商、预填组、缺失模型、官方模型目录同步、渠道上游模型检测、分组倍率、价格倍率同步、io.net 部署和前端模型管理链路。

25. `rbac-admin-permissions-guide-for-go-learners.md`

   面向希望理解后台权限系统的读者。梳理系统角色、AdminAuth/RootAuth、Casbin/GORM adapter、权限 catalog、用户级 override、渠道字段级敏感校验、前端权限矩阵和路由守卫。

26. `usage-dashboard-rankings-guide-for-go-learners.md`

   面向希望理解用量统计和数据看板的读者。梳理消费日志、`quota_data` 小时聚合、Dashboard 模型/用户/流量图、公开排行榜、`perf_metrics` 模型性能指标、Pricing 页面性能展示和 OpenAI dashboard 兼容接口。

27. `realtime-audio-image-embedding-guide-for-go-learners.md`

   面向希望理解非普通文本 relay 的读者。梳理 Realtime WebSocket、音频 speech/transcription/translation、图片 generations/edits、embeddings、moderations、Gemini 原生 embedding、请求转换、usage 获取和计费结算。

28. `uptime-system-status-monitoring-guide-for-go-learners.md`

   面向希望分清状态、健康检查、Uptime、系统资源保护和渠道监控的读者。梳理 `/api/status`、`/api/healthz`、Uptime Kuma、`SystemPerformanceCheck`、Root 性能管理、实例心跳、渠道自动测试和 pprof/Pyroscope。

29. `chat-responses-tools-protocol-guide-for-go-learners.md`

   面向希望理解文本协议转换核心的读者。梳理 Chat Completions、Responses、Claude、Gemini 之间的请求/响应/流式互转，重点解释工具调用、reasoning、usage、built-in tools、pass-through 和 `RequestConversionChain`。

30. `advanced-custom-channel-endpoint-guide-for-go-learners.md`

   面向希望理解高级自定义渠道和 endpoint type 的读者。梳理 type 58 Advanced Custom 渠道、`advanced_custom` route 配置、path-aware 渠道选择、内置 converter、auth 模板、前端编辑器、endpoint 展示和模型定价目录之间的边界。

31. `middleware-routing-request-lifecycle-guide-for-go-learners.md`

   面向希望理解请求进入系统后如何被路由和中间件层层处理的读者。梳理 Gin 全局 middleware、`/api` 与 relay 路由组、User/Admin/Root/Token 鉴权、限流、请求体治理、Turnstile、安全二次验证、系统性能保护、渠道分发和 context key 写入。

32. `channel-request-override-mapping-guide-for-go-learners.md`

   面向希望理解渠道选中后请求如何继续被改写的读者。梳理 `model_mapping`、`param_override`、`header_override`、运行时 header、`status_code_mapping`、系统提示词覆盖、渠道亲和性 override template、前端编辑器和前后端校验差异。

33. `rate-limiting-request-governance-guide-for-go-learners.md`

   面向希望理解系统如何防止滥用和过载的读者。梳理全局 Web/API/Critical/Search 限流、邮件验证码限流、Turnstile、安全二次验证、匿名 body 限制、BodyStorage、模型请求限流、渠道上游 RPM、系统资源保护和前端配置面。

34. `ssrf-file-webhook-security-guide-for-go-learners.md`

   面向希望理解外链访问安全边界的读者。梳理 `fetch_setting`、SSRF URL 校验、DNS/IP/端口过滤、HTTP redirect 再校验、`FileSource` URL/base64 加载、Worker 代理、用户 webhook/Bark/Gotify 外发、视频/Midjourney 结果代理和支付入站 webhook 的区别。

35. `cache-concurrency-consistency-guide-for-go-learners.md`

   面向希望理解缓存、并发控制和一致性边界的读者。梳理 OptionMap、channel cache、Redis user/token hash、HybridCache、渠道亲和性、BatchUpdate、Task CAS、SystemTask lease、SSE stream goroutine、atomic.Value 快照和事务/行锁。

36. `settings-configuration-guide-for-go-learners.md`

   面向希望理解系统设置和配置流的读者。梳理 `options` 表、`OptionMap`、旧式配置 switch、新式 `config.GlobalConfig` 分层配置、配置热更新、Root 配置 API、前端系统设置页面、配置实时性和 relay/计费/安全/性能消费链路。

37. `i18n-localization-guide-for-go-learners.md`

   面向希望理解国际化和语言偏好的读者。梳理后端 go-i18n、`ApiErrorI18n`、message key/YAML、语言解析优先级、用户 `setting.language`、TokenAuth/session 差异、默认前端 i18next、六语言 locale、语言切换和 i18n 同步脚本。

38. `frontend-routing-data-workflows-guide-for-go-learners.md`

   面向希望把默认前端工作流和 Go 后端接口完整串起来的读者。梳理 React 启动、TanStack Router、登录态、路由守卫、Axios、React Query、DataTable、URL search params、用户/渠道/API Key/系统设置/日志端到端链路和权限安全边界。

39. `playground-chat-relay-workflow-guide-for-go-learners.md`

   面向希望理解默认前端 Playground 如何进入统一 relay 的读者。梳理 `/playground` 页面、模型/分组加载、消息状态、本地持久化、`sse.js` 流式请求、`/pg/chat/completions`、`UserAuth`、`Distribute`、临时 playground token、`RelayInfo.IsPlayground`、上游 SSE、usage、计费和消费日志。

40. `user-account-lifecycle-guide-for-go-learners.md`

   面向希望完整理解用户账户生命周期的读者。梳理注册、密码登录、2FA、Passkey、OAuth、Session/UserAuth、自助资料、通知设置、删除账号、邀请、钱包充值、兑换码、签到、管理员用户管理、绑定清理、缓存一致性和安全边界。

41. `api-key-token-lifecycle-guide-for-go-learners.md`

   面向希望完整掌握 API Key/Token 生命周期的读者。梳理默认前端 `/keys` 管理页、`/api/token` CRUD、密钥 mask/reveal、搜索、批量操作、Redis token cache、`TokenAuth` 多协议取 key、IP/模型/分组限制、relay 预扣/结算/退款和 token quota 更新。

42. `group-ratio-access-control-guide-for-go-learners.md`

   面向希望把分组、倍率和访问控制彻底分清的读者。梳理用户组、Token 组、渠道组、Ability 组、`GroupRatio`、`UserUsableGroups`、`GroupGroupRatio`、`TopupGroupRatio`、auto group、跨组重试、分组限流、渠道选路、计费倍率、日志和默认前端配置界面。

43. `channel-credentials-upstream-profile-guide-for-go-learners.md`

   面向希望理解渠道凭据和上游账号体系的读者。梳理单 Key、多 Key、`ChannelInfo`、运行时 Key 选择、`ChannelUpstreamProfile`、`UpstreamIdentity`、上游倍率同步、自动优先级、余额不足通知、前端渠道表单和多 Key 管理界面。

44. `payment-gateway-webhook-subscription-guide-for-go-learners.md`

   面向希望深挖支付网关和订阅支付实现的读者。梳理 Epay、Stripe、Creem、Waffo、Waffo Pancake、`TopUp`、`SubscriptionOrder`、webhook 验签、订单锁、事务幂等、金额和 quota 换算、订阅额度接入 relay 计费，以及默认前端钱包、订阅和支付设置界面。

45. `ionet-model-deployment-guide-for-go-learners.md`

   面向希望深挖 io.net 模型部署管理的读者。梳理 `/api/deployments`、AdminAuth 与 Root 配置边界、`pkg/ionet` normal/enterprise client、部署 CRUD、硬件/地区/副本/价格估算、container logs，以及默认前端 Deployments table、创建抽屉、系统设置、channel/audit/model catalog 边界。

## 文档定位

- 如果你还在建立 Go 项目阅读能力，先读 `go-source-learning-guide.md`。
- 如果你想按目录稳步推进，读 `package-study-map-for-go-learners.md`。
- 如果你想先弄懂核心表、数据对象和状态机，读 `data-model-and-state-guide-for-go-learners.md`。
- 如果你想跟着真实函数调用链练习跳源码，读 `source-walkthroughs-for-go-learners.md`。
- 如果你想读懂后台管理 API 的通用写法和权限/审计模式，读 `backend-request-patterns-for-go-learners.md`。
- 如果你想掌握登录、API Key、TokenAuth、预扣费、结算和退款，读 `auth-token-quota-guide-for-go-learners.md`。
- 如果你想理解 `tiered_expr`、表达式变量、token 归一化、`BillingSnapshot` 和动态计费结算，读 `billing-expression-guide-for-go-learners.md`。
- 如果你想理解 OAuth、Passkey、2FA、安全二次验证、Turnstile、限流、敏感词和细粒度权限，读 `auth-extensions-security-guide-for-go-learners.md`。
- 如果你想彻底掌握渠道配置、ability 索引、缓存、选择算法、重试、亲和性和自动优先级，读 `channel-management-selection-guide-for-go-learners.md`。
- 如果你想理解请求体复读、缓存、HTTP client、并发 map 等公共基础设施，读 `common-infrastructure-guide-for-go-learners.md`。
- 如果你想理解启动顺序、数据库迁移、Option/setting 热更新和跨库兼容，读 `config-database-migration-guide-for-go-learners.md`。
- 如果你想理解后台任务、日志、request id、timing、性能保护和缓存刷新，读 `system-tasks-observability-guide-for-go-learners.md`。
- 如果你想理解 Midjourney、Suno、视频生成等异步任务如何提交、轮询、退款和结算，读 `async-task-media-guide-for-go-learners.md`。
- 如果你想理解消费日志、错误日志、管理审计、通知限流和 Webhook 外发安全，读 `logging-notification-audit-guide-for-go-learners.md`。
- 如果你想理解请求体为什么能被多次读取、URL/base64 文件如何复用、multipart 和多模态资源如何清理，读 `file-resource-body-guide-for-go-learners.md`。
- 如果你想理解充值、支付 webhook、订阅套餐和订阅额度如何接入计费，读 `payment-subscription-guide-for-go-learners.md`。
- 如果你想把 Epay、Stripe、Creem、Waffo、Waffo Pancake 的创建订单、webhook 验签、幂等补单、金额换算、订阅订单和前端支付设置彻底串起来，读 `payment-gateway-webhook-subscription-guide-for-go-learners.md`。
- 如果你想通过测试、断点、request id 和日志来掌握项目行为，读 `testing-and-debugging-guide-for-go-learners.md`。
- 如果你想把默认 React 前端和 Go 后端 API 对上，读 `frontend-default-guide-for-go-learners.md`。
- 如果你在读源码时遇到陌生对象、函数、字段或业务术语，查 `source-glossary-for-go-learners.md`。
- 如果你想搞懂不同上游 provider 如何接入和转换协议，读 `provider-adapter-guide-for-go-learners.md`。
- 如果你想按函数精读 `controller.Relay`、重试、错误选项、流式 SSE 扫描和 usage 结算，读 `relay-error-retry-streaming-guide-for-go-learners.md`。
- 如果你想按专题把启动、relay、计费、前后端管理链路挖透，读 `source-deep-dives-for-go-learners.md`。
- 如果你已经能读 Go 项目，想快速掌握 new-api 的整体实现，读 `new-api-implementation-guide.md`。
- 如果你想分清模型目录、渠道模型、供应商、上游模型检测、价格同步和 io.net 部署之间的边界，读 `model-vendor-deployment-guide-for-go-learners.md`。
- 如果你想把 io.net deployment 的路由、配置、client、创建/更新/日志流程和前端 Deployments 页面完整串起来，读 `ionet-model-deployment-guide-for-go-learners.md`。
- 如果你想理解 Root/Admin/普通用户、Casbin 策略、渠道细粒度权限和前端权限 UI 如何协同，读 `rbac-admin-permissions-guide-for-go-learners.md`。
- 如果你想理解请求结束后的日志、用量聚合、Dashboard 图表、排行榜和模型性能指标如何串起来，读 `usage-dashboard-rankings-guide-for-go-learners.md`。
- 如果你想理解 Realtime、音频、图片、embedding、moderation 这些非普通文本接口如何接入统一 relay 和计费框架，读 `realtime-audio-image-embedding-guide-for-go-learners.md`。
- 如果你想分清 `/api/status`、`/api/healthz`、Uptime Kuma、系统过载保护、实例心跳和渠道自动测试这些“状态/监控”概念，读 `uptime-system-status-monitoring-guide-for-go-learners.md`。
- 如果你想把 Chat Completions、Responses、Claude、Gemini、工具调用和流式协议互转彻底串起来，读 `chat-responses-tools-protocol-guide-for-go-learners.md`。
- 如果你想理解 Advanced Custom 渠道为什么要按 path 过滤、route/converter/auth 如何驱动上游请求，以及 endpoint type 为什么主要影响模型能力展示而不是直接选路，读 `advanced-custom-channel-endpoint-guide-for-go-learners.md`。
- 如果你想把一个请求从 Gin 全局 middleware、路由组、鉴权、限流、Distribute 到 controller/relay 前的 context 写入完整串起来，读 `middleware-routing-request-lifecycle-guide-for-go-learners.md`。
- 如果你想理解渠道选中后模型名、请求体、请求头、错误状态码和系统提示词如何继续被配置化改写，读 `channel-request-override-mapping-guide-for-go-learners.md`。
- 如果你想理解 IP、用户、模型请求、渠道 RPM、系统资源和请求体大小这些防滥用闸门如何分层协作，读 `rate-limiting-request-governance-guide-for-go-learners.md`。
- 如果你想理解用户外链、Webhook 外发、Worker 代理和异步媒体结果代理如何防 SSRF，读 `ssrf-file-webhook-security-guide-for-go-learners.md`。
- 如果你想把缓存、Redis、批量写回、CAS、DB lease、stream goroutine 和 atomic 快照这些并发一致性问题串起来，读 `cache-concurrency-consistency-guide-for-go-learners.md`。
- 如果你想理解一个系统设置 key 如何从默认值、`options` 表、Root API、前端表单一路进入 relay/计费/安全/性能业务逻辑，读 `settings-configuration-guide-for-go-learners.md`。
- 如果你想理解后端 API message、前端 UI 文案、用户语言偏好和翻译文件维护之间如何协作，读 `i18n-localization-guide-for-go-learners.md`。
- 如果你想把默认前端的路由守卫、登录态、Axios、React Query、DataTable 和 Go controller/model 工作流串起来，读 `frontend-routing-data-workflows-guide-for-go-learners.md`。
- 如果你想理解 Playground 发起一条聊天请求后如何复用普通 OpenAI relay、流式 SSE、渠道选择和计费日志链路，读 `playground-chat-relay-workflow-guide-for-go-learners.md`。
- 如果你想把用户从注册、登录、绑定安全能力、使用余额、被管理员管理到删除的生命周期串起来，读 `user-account-lifecycle-guide-for-go-learners.md`。
- 如果你想把 API Key 从控制台创建、mask/reveal、relay 鉴权、模型/分组/IP 限制到计费扣减和缓存失效完整串起来，读 `api-key-token-lifecycle-guide-for-go-learners.md`。
- 如果你想把用户组、Token 组、渠道组和 Ability 组的边界，以及倍率、限流、auto group、选路和计费之间的关系彻底理顺，读 `group-ratio-access-control-guide-for-go-learners.md`。
- 如果你想分清渠道 API Key、多 Key、上游账号档案、上游身份、自动优先级和上游价格同步之间的关系，读 `channel-credentials-upstream-profile-guide-for-go-learners.md`。
- 如果你要改代码，先在这些文档中找到对应模块，再跳到源码和测试。
