# 支付网关、Webhook 与订阅支付学习指南

这份文档专门讲 new-api 里“钱进入系统”的实现：钱包充值、第三方支付网关、支付 webhook、订阅购买、订阅额度接入 relay 计费，以及默认前端如何把这些能力串起来。

它适合在读完这些文档后继续阅读：

- `payment-subscription-guide-for-go-learners.md`
- `auth-token-quota-guide-for-go-learners.md`
- `billing-expression-guide-for-go-learners.md`
- `frontend-routing-data-workflows-guide-for-go-learners.md`

这篇会比总览文档更关注 provider 级细节：Epay、Stripe、Creem、Waffo、Waffo Pancake 的差异，订单幂等边界，金额和 quota 的换算，以及订阅订单如何复用普通支付 webhook。

## 一、先看大图

new-api 里和支付有关的链路可以分成三层：

```text
用户钱包充值
  -> 创建 TopUp pending 订单
  -> 第三方支付完成
  -> webhook/notify 验签
  -> TopUp success
  -> users.quota 增加
  -> 写充值日志

订阅购买
  -> 创建 SubscriptionOrder pending 订单
  -> 第三方支付完成
  -> webhook/notify/return 验签
  -> SubscriptionOrder success
  -> 创建 UserSubscription
  -> 可能升级用户分组
  -> relay 计费时消耗订阅额度

后台配置
  -> 支付合规确认
  -> 配置网关密钥、产品、价格、回调地址
  -> 前端展示可用支付方式
```

钱包充值增加的是 `users.quota`。

订阅购买创建的是 `user_subscriptions`，它有有效期、额度上限、重置周期和分组升级规则。

第三方支付网关本身不会直接影响 relay。relay 只看钱包额度、订阅额度和用户计费偏好。

## 二、核心源码入口

| 关注点 | 文件 |
| --- | --- |
| 路由入口 | `router/api-router.go` |
| 支付合规确认 | `controller/payment_compliance.go` |
| 支付启用条件 | `controller/payment_webhook_availability.go` |
| 钱包充值通用逻辑 / Epay | `controller/topup.go` |
| Stripe 充值和共享 webhook | `controller/topup_stripe.go` |
| Creem 充值和共享 webhook | `controller/topup_creem.go` |
| Waffo 充值和 webhook | `controller/topup_waffo.go` |
| Waffo Pancake 充值、配置和 webhook | `controller/topup_waffo_pancake.go` |
| TopUp 模型和充值事务 | `model/topup.go` |
| 订阅管理 API | `controller/subscription.go` |
| 订阅 Epay 支付 | `controller/subscription_payment_epay.go` |
| 订阅 Stripe 支付 | `controller/subscription_payment_stripe.go` |
| 订阅 Creem 支付 | `controller/subscription_payment_creem.go` |
| 订阅 Waffo Pancake 支付 | `controller/subscription_payment_waffo_pancake.go` |
| 订阅模型、订单、预扣和重置 | `model/subscription.go` |
| 订阅维护任务 | `service/subscription_reset_task.go` |
| relay 计费资金源选择 | `service/billing_session.go`, `service/funding_source.go` |
| Waffo Pancake SDK 封装 | `service/waffo_pancake.go` |
| 默认前端钱包页 | `web/default/src/features/wallet` |
| 默认前端订阅页 | `web/default/src/features/subscriptions` |
| 默认前端支付设置 | `web/default/src/features/system-settings/integrations` |

读源码时建议先从 `router/api-router.go` 开始，把匿名 webhook、用户支付、管理员配置三类路由分清。

## 三、路由分组

支付路由分布在几个不同路由组里。

匿名 webhook：

```text
POST /api/stripe/webhook
POST /api/creem/webhook
POST /api/waffo/webhook
POST /api/waffo-pancake/webhook/:env
POST /api/user/epay/notify
GET  /api/user/epay/notify
POST /api/subscription/epay/notify
GET  /api/subscription/epay/notify
GET  /api/subscription/epay/return
POST /api/subscription/epay/return
```

这些入口通常不能要求用户登录，因为调用方是支付平台。

用户钱包充值：

```text
GET  /api/user/topup/info
GET  /api/user/topup/self
POST /api/user/pay
POST /api/user/stripe/amount
POST /api/user/stripe/pay
POST /api/user/creem/pay
POST /api/user/waffo/amount
POST /api/user/waffo/pay
POST /api/user/waffo-pancake/amount
POST /api/user/waffo-pancake/pay
```

注意：

```text
POST /api/user/topup
```

这个不是在线支付，而是兑换码充值，逻辑在 `controller/user.go` 的 `TopUp`。

管理员充值管理：

```text
GET  /api/admin/topup
POST /api/admin/topup/complete
```

订阅用户路由：

```text
GET  /api/subscription/plans
GET  /api/subscription/self
PUT  /api/subscription/self/preference
POST /api/subscription/balance/pay
POST /api/subscription/epay/pay
POST /api/subscription/stripe/pay
POST /api/subscription/creem/pay
POST /api/subscription/waffo-pancake/pay
```

订阅管理路由：

```text
GET    /api/subscription/admin/plans
POST   /api/subscription/admin/plans
PUT    /api/subscription/admin/plans/:id
PATCH  /api/subscription/admin/plans/:id
POST   /api/subscription/admin/bind
GET    /api/subscription/admin/users/:id/subscriptions
POST   /api/subscription/admin/users/:id/subscriptions
POST   /api/subscription/admin/user_subscriptions/:id/invalidate
DELETE /api/subscription/admin/user_subscriptions/:id
```

## 四、支付合规开关

支付功能前面有一层合规确认。

核心配置在：

```text
setting/operation_setting/payment_setting.go
```

核心判断：

```text
operation_setting.IsPaymentComplianceConfirmed()
```

它要求：

```text
ComplianceConfirmed == true
ComplianceTermsVersion == CurrentComplianceTermsVersion
```

确认接口：

```text
POST /api/option/payment_compliance
  -> controller.ConfirmPaymentCompliance
  -> model.UpdateOption(...)
```

`ConfirmPaymentCompliance` 有一个安全细节：

```text
use_access_token == true 时拒绝
```

也就是说，确认合规必须来自 dashboard session，不能用 API access token 静默完成。

支付启用条件集中在：

```text
controller/payment_webhook_availability.go
```

不同网关的启用条件不一样：

| 网关 | 启用条件 |
| --- | --- |
| Epay | 合规已确认，`PayAddress/EpayId/EpayKey` 已配，且 `PayMethods` 非空 |
| Stripe | 合规已确认，`StripeApiSecret/StripeWebhookSecret/StripePriceId` 已配 |
| Creem | 合规已确认，`CreemApiKey` 和 `CreemProducts` 已配；webhook 还要求 `CreemWebhookSecret` |
| Waffo | 合规已确认，`WaffoEnabled=true`，当前环境 key/private/cert 已配 |
| Waffo Pancake | 合规已确认，`WaffoPancakeMerchantID/PrivateKey/ProductID` 已配 |

`GetTopUpInfo` 会根据这些启用条件决定前端能看到哪些支付方式。

但要注意一个边界：有些创建订单接口本身没有统一调用 `isXTopUpEnabled()`。如果直接打接口，可能创建或尝试创建订单；后续 webhook 又可能因为合规关闭或配置不完整被拒绝。

## 五、TopUp 模型

钱包充值订单模型在 `model/topup.go`：

```go
type TopUp struct {
    Id              int
    UserId          int
    Amount          int64
    Money           float64
    TradeNo         string
    PaymentMethod   string
    PaymentProvider string
    CreateTime      int64
    CompleteTime    int64
    Status          string
}
```

字段含义：

| 字段 | 说明 |
| --- | --- |
| `Amount` | 系统内部用于计算充值额度的数量，含展示模式归一化后的值 |
| `Money` | 用户实际支付金额或展示支付金额 |
| `TradeNo` | 本地唯一订单号 |
| `PaymentMethod` | 用户选择或最终实际支付方式 |
| `PaymentProvider` | 订单归属的支付网关 |
| `Status` | `pending/success/failed/expired` |

`PaymentMethod` 和 `PaymentProvider` 很容易混：

- `PaymentMethod` 更接近“用户选择的付款方式”，例如 Epay 下的 `alipay`、`wxpay`，或者 `stripe`。
- `PaymentProvider` 更接近“这个本地订单应该由哪个网关完成”，例如 `epay`、`stripe`、`creem`。

provider guard 是安全关键：

```text
Webhook -> trade_no -> 查 TopUp -> 检查 PaymentProvider
```

如果拿一个 Stripe 订单号去走 Epay 回调，应该被拒绝。

## 六、订单锁、事务和幂等

支付 webhook 天然会重复投递，也可能和浏览器 return、管理员补单同时发生。

new-api 用两层防线处理：

```text
进程内 LockOrder(trade_no)
  -> DB transaction
  -> FOR UPDATE 锁订单行
  -> 检查 provider
  -> 检查 status
  -> 改 success / failed / expired
  -> 增加 quota 或创建 subscription
```

进程内锁在 `controller/topup.go`：

```text
LockOrder
UnlockOrder
```

它用 `sync.Map` 保存按订单号分片的 mutex，并带引用计数，避免锁对象被过早删除。

DB 事务和行锁在 `model/topup.go`、`model/subscription.go`：

```text
tx.Set("gorm:query_option", "FOR UPDATE").Where(trade_no = ?).First(...)
```

为了兼容 PostgreSQL，代码会用：

```text
`trade_no`
"trade_no"
```

按数据库类型选择列引用方式。

幂等规则：

| 函数 | 已 success 时 |
| --- | --- |
| `RechargeWaffo` | 直接返回成功 |
| `RechargeWaffoPancake` | 直接返回成功 |
| `ManualCompleteTopUp` | 直接返回成功 |
| `CompleteSubscriptionOrder` | 直接返回成功 |
| `Recharge` / Stripe 普通充值 | 非 pending 返回错误 |
| `RechargeCreem` | 非 pending 返回错误 |

所以读 controller 时要结合 model 一起看：有的 controller 会在调用充值前先检查 pending，有的把幂等交给 model。

## 七、Epay 钱包充值

Epay 是旧式表单跳转支付。

创建订单入口：

```text
controller.RequestEpay
```

流程：

```text
RequestEpay
  -> 解析 amount/payment_method
  -> 检查最小充值
  -> 读取用户 group
  -> getPayMoney(amount, group)
  -> 检查 payment_method 是否在 PayMethods
  -> 生成 USR{id}NO... trade_no
  -> epay.Purchase(...)
  -> 插入 TopUp pending
  -> 返回 uri + params 给前端提交表单
```

回调入口：

```text
controller.EpayNotify
```

流程：

```text
EpayNotify
  -> 检查 isEpayWebhookEnabled
  -> GET/POST 参数转 map
  -> GetEpayClient().Verify(params)
  -> 先写 success/fail 给 Epay
  -> TradeStatus == success 时继续处理
  -> LockOrder(trade_no)
  -> 查 TopUp
  -> 检查 PaymentProvider == epay
  -> pending -> success
  -> users.quota += TopUp.Amount * QuotaPerUnit
  -> RecordTopupLog
```

一个重要边界：

```text
EpayNotify 验签成功后先返回 "success"，再处理本地充值。
```

如果本地数据库更新随后失败，Epay 平台可能不会重试。这是读这段时必须知道的风险边界。

## 八、Stripe 钱包充值

Stripe 使用 Checkout Session。

创建入口：

```text
controller.RequestStripePay
```

流程：

```text
RequestStripePay
  -> payment_method 必须是 stripe
  -> 校验 amount 范围
  -> 校验 success_url/cancel_url 可信
  -> 读取用户
  -> chargedMoney = req.Amount * TopupGroupRatio
  -> referenceId = ref_ + sha1(...)
  -> genStripeLink(...)
  -> 插入 TopUp pending
  -> 返回 pay_link
```

`genStripeLink` 的关键点：

```text
ClientReferenceID = referenceId
Price = StripePriceId
Quantity = amount
Mode = payment
```

Webhook 入口：

```text
controller.StripeWebhook
```

它处理这些事件：

```text
checkout.session.completed
checkout.session.expired
checkout.session.async_payment_succeeded
checkout.session.async_payment_failed
```

成功事件进入：

```text
fulfillOrder
```

这个函数有一个非常关键的复用逻辑：

```text
fulfillOrder
  -> 先 model.CompleteSubscriptionOrder(referenceId, ..., stripe)
  -> 如果不是订阅订单
  -> 再 model.Recharge(referenceId, customerId, callerIp)
```

所以 Stripe 的一个 webhook 同时服务订阅和钱包充值。

过期事件也类似：

```text
sessionExpired
  -> 先 ExpireSubscriptionOrder
  -> 如果不是订阅订单
  -> UpdatePendingTopUpStatus(... expired)
```

Stripe 的另一个边界：

```text
StripeWebhook 最后通常返回 200 OK。
```

普通充值失败时多是记录日志，而不是让 Stripe 重试。

## 九、Creem 钱包充值

Creem 的充值金额来自配置的产品列表：

```text
setting.CreemProducts
```

产品结构在 `controller/topup_creem.go`：

```go
type CreemProduct struct {
    ProductId string
    Name      string
    Price     float64
    Currency  string
    Quota     int64
}
```

创建入口：

```text
controller.RequestCreemPay
```

流程：

```text
RequestCreemPay
  -> product_id 必须存在
  -> 从 CreemProducts 找产品
  -> referenceId = ref_ + sha1(...)
  -> 插入 TopUp pending
       Amount = product.Quota
       Money  = product.Price
  -> genCreemLink(referenceId, product, user.Email, user.Username)
  -> 返回 checkout_url/order_id
```

Creem webhook：

```text
controller.CreemWebhook
```

验签逻辑：

```text
Header: creem-signature
HMAC-SHA256(body, CreemWebhookSecret)
```

入口会先检查 `isCreemWebhookEnabled()`，因此通常必须配置 webhook secret。

只有这个事件会处理：

```text
checkout.completed
```

且要求：

```text
event.Object.Order.Status == "paid"
```

订单号来自：

```text
event.Object.RequestId
```

成功处理顺序：

```text
handleCheckoutCompleted
  -> LockOrder(request_id)
  -> 先 CompleteSubscriptionOrder(request_id, ..., creem)
  -> 如果不是订阅订单
  -> 要求 Order.Type == "onetime"
  -> 查 TopUp
  -> pending 才处理
  -> RechargeCreem
```

Creem 充值不乘 `QuotaPerUnit`：

```text
TopUp.Amount = product.Quota
RechargeCreem -> users.quota += TopUp.Amount
```

这是它和 Epay/Stripe/Waffo/Pancake 最大的金额语义差异。

## 十、Waffo 钱包充值

Waffo 是独立 SDK 接入。

创建入口：

```text
controller.RequestWaffoPay
```

流程：

```text
RequestWaffoPay
  -> 检查 WaffoEnabled
  -> 解析 amount/pay_method
  -> 校验服务端配置的支付方式
  -> getWaffoPayMoney(amount, group)
  -> tradeNo = WAFFO-...
  -> 插入 TopUp pending
  -> sdk.Order().Create(...)
  -> 返回 redirect URL
```

支付方式选择有两个路径：

- 新路径：`pay_method_index`，用服务端配置列表索引选择。
- 兼容路径：`pay_method_type/pay_method_name`。

Webhook 入口：

```text
controller.WaffoWebhook
```

流程：

```text
WaffoWebhook
  -> 检查 isWaffoWebhookEnabled
  -> 读 X-SIGNATURE
  -> SDK 验签
  -> 只处理 PAYMENT 事件
  -> order_status == PAY_SUCCESS
  -> handleWaffoPayment
  -> RechargeWaffo
```

如果订单状态不是成功，代码会尝试把 pending 订单标为 failed。

Waffo 充值额度：

```text
TopUp.Amount * QuotaPerUnit
```

如果系统用 TOKENS 展示，创建订单时会把前端传入的 token 数归一化成金额单位，避免完成时再乘 `QuotaPerUnit` 导致重复放大。

## 十一、Waffo Pancake 钱包充值

Waffo Pancake 的接入比 Waffo 多一层商户店铺和产品配置。

配置相关接口在 `controller/topup_waffo_pancake.go`：

```text
POST /api/option/waffo-pancake/save
POST /api/option/waffo-pancake/pair
GET  /api/option/waffo-pancake/catalog
POST /api/option/waffo-pancake/subscription-product
GET  /api/option/waffo-pancake/subscription-product-options
```

`pair` 和 `catalog` 可以用“前端输入但尚未保存”的凭据去探测或创建。只有 `save` 会真正写入 OptionMap。

钱包充值入口：

```text
controller.RequestWaffoPancakePay
```

流程：

```text
RequestWaffoPancakePay
  -> 检查 isWaffoPancakeTopUpEnabled
  -> 检查 min topup
  -> 读取用户和 group
  -> getWaffoPancakePayMoney
  -> tradeNo = WAFFO_PANCAKE-...
  -> 插入 TopUp pending
  -> CreateWaffoPancakeCheckoutSession
       ProductID = WaffoPancakeProductID
       BuyerIdentity = newapi_user_{id}
       OrderMerchantExternalID = tradeNo
  -> 返回 checkout_url/session_id/order_id/token
```

Webhook 入口：

```text
POST /api/waffo-pancake/webhook/:env
```

`:env` 只能是：

```text
test
prod
```

处理流程：

```text
WaffoPancakeWebhook
  -> 检查 webhook enabled
  -> 校验 URL env
  -> 读 X-Waffo-Signature
  -> VerifyConfiguredWaffoPancakeWebhook
  -> event.Mode 必须等于 URL env
  -> 只处理 order.completed
  -> 根据 trade_no 前缀判断订阅/充值
```

普通充值订单号不是 Pancake 的 `ORD_*`，而是：

```text
event.Data.OrderMerchantExternalID
```

解析订单时还会校验 buyer identity，避免仅凭订单号完成别人的订单。

## 十二、金额、折扣和分组充值倍率

支付金额不只是用户输入的 amount。

常见变量：

| 变量 | 来源 | 用途 |
| --- | --- | --- |
| `amount` | 前端输入 | 用户想买多少额度或数量 |
| `QuotaPerUnit` | `common` | 金额单位到内部 quota 的换算 |
| `TopupGroupRatio` | 分组配置 | 不同用户组充值价格系数 |
| `AmountDiscount` | 支付设置 | 预设金额折扣 |
| provider unit price | provider 设置 | Stripe/Waffo/Pancake 的单位价格 |

Epay：

```text
payMoney = normalizedAmount * Price * TopupGroupRatio * Discount
充值 quota = TopUp.Amount * QuotaPerUnit
```

Stripe：

```text
预估 payMoney = amount * StripeUnitPrice * TopupGroupRatio * Discount
Checkout = StripePriceId * Quantity(amount)
落库 Money = amount * TopupGroupRatio
充值 quota = TopUp.Money * QuotaPerUnit
```

这里要注意：Stripe 预估里有 `StripeUnitPrice`，但 `GetChargedAmount` 落库时只乘分组充值倍率。实际扣款价格由 Stripe 的 PriceId 和 Quantity 决定。

Creem：

```text
TopUp.Amount = product.Quota
TopUp.Money  = product.Price
充值 quota = TopUp.Amount
```

Waffo：

```text
payMoney = normalizedAmount * WaffoUnitPrice * TopupGroupRatio * Discount
充值 quota = TopUp.Amount * QuotaPerUnit
```

Waffo Pancake：

```text
payMoney = normalizedAmount * WaffoPancakeUnitPrice * TopupGroupRatio * Discount
充值 quota = TopUp.Amount * QuotaPerUnit
```

TOKENS 展示模式下：

- Epay 和 Stripe 有最小充值换算逻辑。
- Waffo 和 Pancake 的最小充值比较仍更接近原始请求量。
- Waffo/Pancake 会在落库前把 token 数归一化，避免完成时乘 `QuotaPerUnit` 后重复放大。

## 十三、订阅模型

订阅有三张核心表。

`SubscriptionPlan`：

```text
套餐定义
```

关键字段：

| 字段 | 说明 |
| --- | --- |
| `PriceAmount` | 展示价格，后台强制 USD |
| `DurationUnit/DurationValue/CustomSeconds` | 有效期 |
| `AllowBalancePay` | 是否允许钱包余额购买 |
| `AllowWalletOverflow` | 订阅额度不足时是否允许钱包兜底 |
| `StripePriceId` | Stripe 产品价格 ID |
| `CreemProductId` | Creem 产品 ID |
| `WaffoPancakeProductId` | Pancake 产品 ID |
| `MaxPurchasePerUser` | 用户购买上限，0 表示不限 |
| `UpgradeGroup/DowngradeGroup` | 购买后升级组，过期后降级组 |
| `TotalAmount` | 订阅总额度，0 表示无限 |
| `QuotaResetPeriod` | 额度重置周期 |

`SubscriptionOrder`：

```text
订阅支付订单
```

它和 `TopUp` 很像，也有：

```text
TradeNo
PaymentMethod
PaymentProvider
Status
ProviderPayload
```

`UserSubscription`：

```text
用户订阅实例
```

它是购买完成后的快照：

| 字段 | 说明 |
| --- | --- |
| `AmountTotal/AmountUsed` | 订阅额度 |
| `StartTime/EndTime/Status` | 生命周期 |
| `Source` | `order/admin/balance` 等来源 |
| `LastResetTime/NextResetTime` | 重置状态 |
| `UpgradeGroup/PrevUserGroup/DowngradeGroup` | 分组变更和回退 |
| `AllowWalletOverflow` | 从 plan 购买时复制的兜底策略 |

`AllowWalletOverflow` 是快照。改套餐不会自动改已有订阅。

## 十四、创建 UserSubscription

统一创建函数：

```text
model.CreateUserSubscriptionFromPlanTx
```

流程：

```text
CreateUserSubscriptionFromPlanTx
  -> 检查 plan/user
  -> MaxPurchasePerUser 限购
  -> calcPlanEndTime
  -> calcNextResetTime
  -> 如果 UpgradeGroup 非空，锁定并更新用户 group
  -> 创建 UserSubscription
       AmountTotal = plan.TotalAmount
       AmountUsed = 0
       EndTime = 根据套餐周期计算
       AllowWalletOverflow = plan 快照
       PrevUserGroup = 购买前用户组
```

限购边界：

```text
MaxPurchasePerUser 统计用户历史上所有该 plan 订阅。
```

它不只统计 active，也包括 expired/cancelled。

## 十五、完成订阅订单

统一完成函数：

```text
model.CompleteSubscriptionOrder
```

流程：

```text
CompleteSubscriptionOrder(tradeNo, providerPayload, expectedProvider, actualMethod)
  -> 事务 + FOR UPDATE 锁 SubscriptionOrder
  -> 检查 expectedPaymentProvider
  -> success 直接返回
  -> 非 pending 报错
  -> 读取 plan
  -> CreateUserSubscriptionFromPlanTx
  -> upsertSubscriptionTopUpTx
  -> order.Status = success
  -> 保存 ProviderPayload
  -> 如果 actualPaymentMethod 非空则更新 PaymentMethod
  -> 事务外刷新用户组缓存、写日志
```

`upsertSubscriptionTopUpTx` 会给订阅支付也写一条 `TopUp` 风格的记录，用于充值/支付记录展示。

订阅的 `TopUp.Amount` 通常是 0，因为它不是给钱包直接加额度。

provider guard 同样重要：

```text
expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider
  -> ErrPaymentMethodMismatch
```

这样能防止跨网关 callback 完成订单。

## 十六、订阅购买方式

### 1. 余额购买

入口：

```text
controller.SubscriptionRequestBalancePay
```

核心：

```text
model.PurchaseSubscriptionWithBalance
```

流程：

```text
PurchaseSubscriptionWithBalance
  -> 事务
  -> 读取 plan
  -> 检查 enabled/price/AllowBalancePay
  -> requiredQuota = ceil(plan.PriceAmount * QuotaPerUnit)
  -> FOR UPDATE 锁用户
  -> 扣 users.quota
  -> CreateUserSubscriptionFromPlanTx
  -> 创建 success SubscriptionOrder
  -> 事务外更新 quota cache / group cache / log
```

这里花的是钱包余额，不经过外部支付 webhook。

### 2. Epay 订阅

入口：

```text
controller.SubscriptionRequestEpay
```

流程：

```text
SubscriptionRequestEpay
  -> 校验合规
  -> 读取 plan
  -> 检查 enabled / price / payment_method / 限购
  -> tradeNo = SUBUSR...
  -> 创建 SubscriptionOrder pending
  -> epay.Purchase(...)
  -> 拉起失败则 ExpireSubscriptionOrder
  -> 返回 uri + params
```

完成入口有两个：

```text
SubscriptionEpayNotify
SubscriptionEpayReturn
```

两者都会验签，并调用：

```text
CompleteSubscriptionOrder(tradeNo, payload, epay, verifyInfo.Type)
```

所以这里非常依赖 `CompleteSubscriptionOrder` 幂等。

### 3. Stripe 订阅

入口：

```text
controller.SubscriptionRequestStripePay
```

流程：

```text
SubscriptionRequestStripePay
  -> 校验合规
  -> 读取 plan
  -> 检查 plan.StripePriceId
  -> 检查 Stripe secret / webhook secret
  -> 限购
  -> referenceId = sub_ref_...
  -> genStripeSubscriptionLink
       Mode = subscription
       Price = plan.StripePriceId
       ClientReferenceID = referenceId
  -> 创建 SubscriptionOrder pending
  -> 返回 pay_link
```

完成依赖普通 Stripe webhook：

```text
StripeWebhook -> fulfillOrder -> CompleteSubscriptionOrder
```

### 4. Creem 订阅

入口：

```text
controller.SubscriptionRequestCreemPay
```

流程要点：

- plan 必须配置 `CreemProductId`。
- 创建 `SubscriptionOrder pending`。
- 构造一个轻量 `CreemProduct`，复用普通充值的 `genCreemLink`。
- 完成依赖普通 Creem webhook，先尝试 `CompleteSubscriptionOrder`。

### 5. Waffo Pancake 订阅

入口：

```text
controller.SubscriptionRequestWaffoPancakePay
```

流程要点：

- plan 必须配置 `WaffoPancakeProductId`。
- trade_no 使用：

```text
WAFFO_PANCAKE_SUB-...
```

- 创建 `SubscriptionOrder pending`。
- 调 `CreateWaffoPancakeCheckoutSession`。
- 创建会话失败会把订单标为 failed。
- 完成依赖普通 Pancake webhook，通过 `WAFFO_PANCAKE_SUB-` 前缀分流。

## 十七、订阅如何接入 relay 计费

支付完成只是创建 `UserSubscription`。真正消费发生在 relay 请求前后。

入口：

```text
service.NewBillingSession
```

用户偏好来自：

```text
user setting: billing_preference
```

归一化合法值：

```text
subscription_first
wallet_first
subscription_only
wallet_only
```

默认是：

```text
subscription_first
```

决策逻辑：

```text
subscription_only
  -> 只走订阅

wallet_only
  -> 只走钱包

wallet_first
  -> 先钱包
  -> 钱包额度不足时回退订阅

subscription_first
  -> 无活跃订阅：走钱包
  -> 有活跃订阅：先走订阅
  -> 订阅额度不足：
       若所有活跃订阅都允许 AllowWalletOverflow，回退钱包
       否则返回订阅额度不足
```

订阅资金源：

```text
SubscriptionFunding.PreConsume
  -> model.PreConsumeUserSubscription
```

钱包资金源：

```text
WalletFunding.PreConsume
  -> model.DecreaseUserQuota
```

订阅预扣至少为 1：

```text
if preConsumedQuota <= 0 {
    subConsume = 1
}
```

原因是订阅预扣需要创建预扣记录并锁定订阅，`amount <= 0` 不合法。

订阅不走 trust quota 旁路。否则预扣记录、实际扣费和日志字段会不一致。

结算：

```text
BillingSession.Settle(actualQuota)
  -> delta = actualQuota - preConsumedQuota
  -> funding.Settle(delta)
  -> token quota 补扣或退还
```

失败退款：

```text
BillingSession.Refund
  -> funding.Refund
  -> 退 token quota
  -> 如果订阅有 extraReserved，单独 PostConsumeUserSubscriptionDelta(-extraReserved)
```

## 十八、订阅预扣记录

订阅预扣模型：

```text
SubscriptionPreConsumeRecord
```

用途：

```text
request_id -> 本次订阅预扣
```

这样同一个 request 重复进入预扣时，可以识别已有预扣。

相关函数：

```text
PreConsumeUserSubscription
RefundSubscriptionPreConsume
PostConsumeUserSubscriptionDelta
CleanupSubscriptionPreConsumeRecords
```

边界：

- request id 必须稳定且唯一。
- 已退款记录再次使用会报错。
- `Reserve` 对订阅的额外预留不是通过新的 `SubscriptionPreConsumeRecord` 表示，而是靠 `BillingSession.extraReserved` 在失败退款时单独回滚。

## 十九、订阅过期和额度重置

维护任务：

```text
service.StartSubscriptionQuotaResetTask
```

启动点：

```text
main.go
```

行为：

```text
仅 master 节点启动
每 1 分钟 tick
启动后立即跑一次
atomic.Bool 防重入
每批 300 条
先 ExpireDueSubscriptions
再 ResetDueSubscriptions
每 30 分钟清理 7 天前的预扣记录
```

过期：

```text
model.ExpireDueSubscriptions
```

它会：

- 把到期 active subscription 标记为 expired。
- 如果没有其他 active upgraded subscription，按 `DowngradeGroup` 或 `PrevUserGroup` 回退用户组。
- 刷新用户组缓存。

重置：

```text
model.ResetDueSubscriptions
```

重置周期：

| 周期 | 含义 |
| --- | --- |
| `never` | 不重置 |
| `daily` | 下一个本地日期 00:00 |
| `weekly` | 下一个周一 00:00 |
| `monthly` | 下个月 1 日 00:00 |
| `custom` | 按秒数滚动 |

真正重置时：

```text
AmountUsed = 0
LastResetTime 推进
NextResetTime 推进
```

`TotalAmount=0` 表示无限额度，但仍可能累计 `AmountUsed`，只是不会做上限检查。

## 二十、前端钱包充值数据流

默认前端的旧 `/console/topup` 已重定向到：

```text
/wallet
```

路由文件：

```text
web/default/src/routes/console/topup.tsx
```

钱包功能主要在：

```text
web/default/src/features/wallet
```

典型数据流：

```text
页面加载
  -> useTopupInfo
  -> GET /api/user/topup/info
  -> 得到 pay_methods、min_topup、amount_options、discount、creem_products、waffo_pay_methods
  -> 用户选择支付方式和金额
  -> 先调 amount 预估接口
  -> 点击支付
  -> 调 provider pay 接口
  -> 打开 pay_link / checkout_url 或提交 Epay 表单
```

`usePayment` 处理 Epay/Stripe/Pancake 的基础分流：

```text
stripe
  -> /api/user/stripe/amount
  -> /api/user/stripe/pay
  -> window.open(pay_link)

waffo_pancake
  -> /api/user/waffo-pancake/amount
  -> 通常走专用 hook 发起 checkout

other epay methods
  -> /api/user/pay
  -> submitPaymentForm(url, params)
```

`submitPaymentForm` 会动态创建 HTML form。非 Safari 会用新窗口，Safari 直接当前上下文提交，避免弹窗兼容问题。

Creem、Waffo、Waffo Pancake 有各自 hook：

```text
use-creem-payment.ts
use-waffo-payment.ts
use-waffo-pancake-payment.ts
```

它们对应后端：

```text
POST /api/user/creem/pay
POST /api/user/waffo/pay
POST /api/user/waffo-pancake/pay
```

## 二十一、前端订阅 UI

订阅前端主要在：

```text
web/default/src/features/subscriptions
```

API 封装：

```text
web/default/src/features/subscriptions/api.ts
```

主要接口映射：

| 前端函数 | 后端接口 |
| --- | --- |
| `getPublicPlans` | `GET /api/subscription/plans` |
| `getSelfSubscriptionFull` | `GET /api/subscription/self` |
| `updateBillingPreference` | `PUT /api/subscription/self/preference` |
| `paySubscriptionBalance` | `POST /api/subscription/balance/pay` |
| `paySubscriptionEpay` | `POST /api/subscription/epay/pay` |
| `paySubscriptionStripe` | `POST /api/subscription/stripe/pay` |
| `paySubscriptionCreem` | `POST /api/subscription/creem/pay` |
| `paySubscriptionWaffoPancake` | `POST /api/subscription/waffo-pancake/pay` |
| `getAdminPlans` | `GET /api/subscription/admin/plans` |
| `createPlan` | `POST /api/subscription/admin/plans` |
| `updatePlan` | `PUT /api/subscription/admin/plans/:id` |
| `patchPlanStatus` | `PATCH /api/subscription/admin/plans/:id` |
| `createUserSubscription` | `POST /api/subscription/admin/users/:id/subscriptions` |
| `invalidateUserSubscription` | `POST /api/subscription/admin/user_subscriptions/:id/invalidate` |
| `deleteUserSubscription` | `DELETE /api/subscription/admin/user_subscriptions/:id` |

订阅购买弹窗会根据 plan 的 provider 字段和系统支付方式决定展示哪些按钮。

管理员订阅计划 UI 负责编辑：

- 套餐标题和价格。
- 有效期。
- 总额度。
- 重置周期。
- 是否启用。
- 是否允许余额购买。
- 是否允许钱包兜底。
- Stripe / Creem / Waffo Pancake 产品 ID。
- 购买后升级分组和过期后降级分组。

Waffo Pancake 有辅助接口为订阅套餐创建产品：

```text
POST /api/option/waffo-pancake/subscription-product
GET  /api/option/waffo-pancake/subscription-product-options
```

命名叫 subscription product，但底层实际创建的是 Pancake OnetimeProduct，用来表示 new-api 的订阅套餐付款入口。

## 二十二、前端支付设置 UI

支付设置注册在：

```text
web/default/src/features/system-settings/billing/section-registry.tsx
```

主组件：

```text
web/default/src/features/system-settings/integrations/payment-settings-section.tsx
```

它管理这些配置：

| 配置 | 对应后端字段 |
| --- | --- |
| Epay 地址、ID、Key、价格、最小充值 | `PayAddress/EpayId/EpayKey/Price/MinTopUp` |
| Epay 支付方式 | `PayMethods` |
| 预设金额 | `payment_setting.amount_options` |
| 预设金额折扣 | `payment_setting.amount_discount` |
| Stripe secret、webhook secret、price id | `StripeApiSecret/StripeWebhookSecret/StripePriceId` |
| Creem API key、webhook secret、产品列表 | `CreemApiKey/CreemWebhookSecret/CreemProducts` |
| Waffo sandbox/prod 凭据、商户号、币种、支付方式 | `Waffo*` |
| Waffo Pancake merchant/private/store/product | `WaffoPancake*` |
| 合规确认 | `payment_setting.compliance_*` |

合规确认 UI 会弹出风险确认，并调用：

```text
confirmPaymentCompliance
  -> POST /api/option/payment_compliance
```

保存系统设置通常走通用 option 更新，但合规字段不能通过普通 option 更新绕过，必须走专门接口。

多个 JSON 字段提供了可视化编辑器：

- `PayMethods`
- `AmountOptions`
- `AmountDiscount`
- `CreemProducts`
- `WaffoPayMethods`

这些字段最终还是保存为 option 字符串，由后端 setting 包解析。

## 二十三、支付网关和订阅的复用关系

Stripe、Creem、Waffo Pancake 有一个共同模式：

```text
同一个 provider webhook
  -> 先尝试 CompleteSubscriptionOrder
  -> 如果本地没有订阅订单
  -> 再按钱包充值处理 TopUp
```

这样外部平台只需要配置一个 webhook。

但它要求本地订单号足够可区分：

| provider | 钱包订单号 | 订阅订单号 |
| --- | --- | --- |
| Stripe | `ref_...` | `sub_ref_...` |
| Creem | `ref_...` | `sub_creem_ref_...` |
| Waffo Pancake | `WAFFO_PANCAKE-...` | `WAFFO_PANCAKE_SUB-...` |
| Epay | `USR...` | `SUBUSR...` |

即使前缀区分不严，provider guard 仍会保护：

```text
TopUp.PaymentProvider
SubscriptionOrder.PaymentProvider
```

## 二十四、常见误解

### 误解 1：充值 amount 永远乘 QuotaPerUnit

不对。

Epay、Waffo、Waffo Pancake 充值一般是：

```text
TopUp.Amount * QuotaPerUnit
```

Stripe 普通充值是：

```text
TopUp.Money * QuotaPerUnit
```

Creem 是：

```text
TopUp.Amount
```

因为 Creem 产品配置里的 `Quota` 已经是内部额度。

### 误解 2：订阅支付成功会增加 users.quota

不对。

订阅支付成功创建 `UserSubscription`，不会给钱包加余额。

为了支付记录展示，系统会 upsert 一条 `TopUp` 风格记录，但 `Amount` 通常是 0。

### 误解 3：订阅套餐改了，用户已有订阅马上变化

大多不对。

`UserSubscription` 保存了购买时快照：

- 总额度。
- 有效期。
- 重置时间。
- 升级/降级分组。
- 是否允许钱包兜底。

后续改 plan 不会自动重写已有订阅。

### 误解 4：所有 webhook 失败都会触发第三方重试

不一定。

Epay 验签成功后先写 `success`，本地后续失败平台可能不会重试。

Stripe handler 最后通常返回 200，普通充值失败只是记录日志。

Pancake 对永久不可解析的订单也会返回 OK，避免第三方重复推无法处理的事件。

### 误解 5：Waffo Pancake 的订单号就是 ORD_*

不对。

`ORD_*` 是 Pancake 平台内部订单号。

new-api 本地订单号放在：

```text
orderMerchantExternalId
```

并且会配合买家身份校验。

### 误解 6：subscription_first 就一定能用钱包兜底

不一定。

如果存在活跃订阅，且其中有订阅不允许 `AllowWalletOverflow`，订阅额度不足时就不会回退钱包。

### 误解 7：订阅可以走 trust quota 旁路

不可以。

订阅预扣需要创建预扣记录并锁定订阅。如果跳过预扣，后续结算、退款和日志字段会不一致。

## 二十五、建议阅读顺序

第一轮只看主线：

1. `router/api-router.go`
2. `controller/payment_webhook_availability.go`
3. `model/topup.go`
4. `controller/topup.go`
5. `controller/topup_stripe.go`
6. `model/subscription.go`
7. `controller/subscription.go`
8. `service/billing_session.go`

第二轮看 provider 差异：

1. `controller/topup_creem.go`
2. `controller/topup_waffo.go`
3. `controller/topup_waffo_pancake.go`
4. `controller/subscription_payment_*.go`
5. `service/waffo_pancake.go`

第三轮把前端串起来：

1. `web/default/src/features/wallet`
2. `web/default/src/features/subscriptions/api.ts`
3. `web/default/src/features/subscriptions/components`
4. `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`
5. `web/default/src/features/system-settings/integrations/waffo-pancake-settings-section.tsx`

## 二十六、给 Go 学习者的源码读法

这套支付代码很适合练习 Go 项目里的几个能力。

### 1. 读事务边界

看到支付完成函数时，先找：

```text
DB.Transaction
FOR UPDATE
status check
provider check
```

这些比普通 if/else 更重要，因为支付系统最怕重复加钱。

### 2. 读错误返回语义

有些错误要让第三方重试，有些错误要返回 OK 避免重复推送。

例如 Pancake 里：

- 本地完成失败可能返回 `500 retry`。
- 永久解析不了的订单返回 `200 OK`。

### 3. 区分 model 和 controller 职责

controller：

- 解析请求。
- 验签。
- 调 provider SDK。
- 选择成功/失败响应。

model：

- 锁订单。
- 检查状态。
- 更新用户 quota。
- 创建订阅。
- 维护幂等。

### 4. 注意 float 和 decimal 的边界

金额计算里有些路径用 `shopspring/decimal`，有些 Stripe 小额场景仍用 float64。

读金额代码时要追踪：

```text
前端传入 amount
显示模式是否是 TOKENS
是否乘 UnitPrice
是否乘 TopupGroupRatio
是否乘 Discount
落库 Amount/Money
完成时如何转 users.quota
```

### 5. 不要只读 happy path

支付代码的关键价值在异常分支：

- webhook 禁用。
- 验签失败。
- 订单不存在。
- provider 不匹配。
- 订单已 success。
- 订单非 pending。
- 第三方订单非 paid。
- 异步支付失败。
- 用户组升级后过期回退。

这些分支决定系统是否会重复充值、漏充值或错误完成订单。
