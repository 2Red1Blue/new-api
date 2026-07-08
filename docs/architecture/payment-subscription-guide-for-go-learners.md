# 支付、充值与订阅学习指南

这份文档梳理 new-api 的充值、支付网关、订阅套餐、订阅额度、支付回调和 `BillingSession` 接入。它适合和 `auth-token-quota-guide-for-go-learners.md` 一起阅读。

先建立一个大图：

```text
充值 topup
  -> 用户支付
  -> webhook/notify 完成订单
  -> 增加 users.quota
  -> 写 topup/log

订阅 subscription
  -> 用户购买 plan
  -> webhook/notify 完成 subscription order
  -> 创建 user subscription
  -> relay 计费时优先消耗订阅额度或回退钱包
```

充值增加的是钱包余额。订阅增加的是一个有有效期、额度、重置周期、分组升级规则的订阅实例。

## 一、路由入口

支付和订阅路由集中在 `router/api-router.go`。

匿名 webhook：

```text
POST /api/stripe/webhook
POST /api/creem/webhook
POST /api/waffo/webhook
POST /api/waffo-pancake/webhook/:env
POST /api/user/epay/notify
GET  /api/user/epay/notify
```

用户充值：

```text
GET  /api/user/topup/info
GET  /api/user/topup/self
POST /api/user/topup
POST /api/user/pay
POST /api/user/stripe/pay
POST /api/user/stripe/amount
POST /api/user/creem/pay
POST /api/user/waffo/amount
POST /api/user/waffo/pay
POST /api/user/waffo-pancake/amount
POST /api/user/waffo-pancake/pay
```

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

订阅支付回调：

```text
POST /api/subscription/epay/notify
GET  /api/subscription/epay/notify
GET  /api/subscription/epay/return
POST /api/subscription/epay/return
```

## 二、支付合规开关

支付相关功能前面有一层合规确认。

配置在：

```text
setting/operation_setting/payment_setting.go
```

核心字段：

```go
type PaymentSetting struct {
    AmountOptions []int
    AmountDiscount map[int]float64
    ComplianceConfirmed bool
    ComplianceTermsVersion string
    ComplianceConfirmedAt int64
    ComplianceConfirmedBy int
    ComplianceConfirmedIP string
}
```

是否已确认：

```go
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
  -> model.UpdateOptionsBulk
  -> payment_setting.compliance_*
```

一些支付开关和邀请奖励等正向额度配置，在未确认合规前会被拒绝。通用 option 更新还禁止直接修改 `payment_setting.compliance_*`，必须走专门确认接口。

这说明支付功能不是单纯技术开关，还要求管理员明确确认运营合规责任。

## 三、充值模型 TopUp

充值模型在 `model/topup.go`：

```go
type TopUp struct {
    Id int
    UserId int
    Amount int64
    Money float64
    TradeNo string
    PaymentMethod string
    PaymentProvider string
    CreateTime int64
    CompleteTime int64
    Status string
}
```

常见状态：

```text
pending
success
failed
expired
```

支付 method/provider：

```text
epay
stripe
creem
waffo
waffo_pancake
balance
```

`PaymentMethod` 偏向用户选择或实际支付方式，`PaymentProvider` 偏向订单应由哪个网关完成。很多完成函数都会检查 `expectedPaymentProvider`，防止跨网关 callback 攻击。

## 四、充值流程

以 Stripe / Waffo / Epay 这类在线充值为例：

```text
用户发起充值
  -> controller 创建 TopUp(status=pending)
  -> 调支付网关生成 pay link/form
  -> 用户完成支付
  -> 网关 webhook/notify
  -> 验签
  -> 根据 trade_no 查 TopUp
  -> 检查 PaymentProvider 和 status
  -> 标记 success
  -> users.quota += topUp.Money * QuotaPerUnit
  -> RecordTopupLog
```

本地余额增加的核心在 `model.Recharge` 及各 provider 对应函数，例如 Waffo、Waffo Pancake 有各自的处理函数。

重要防线：

- `trade_no` 唯一。
- webhook 必须验签。
- 完成订单必须在事务内。
- 使用 `FOR UPDATE` 锁订单。
- pending 之外的订单不能重复充值。
- `expectedPaymentProvider` 必须匹配。

## 五、Epay 充值

Epay 充值入口：

```text
controller.RequestEpay
controller.EpayNotify
```

大致流程：

```text
RequestEpay
  -> 检查支付方式
  -> 创建 TopUp pending
  -> epay.Purchase
  -> 返回跳转参数

EpayNotify
  -> ParseForm / Query
  -> GetEpayClient().Verify
  -> TradeStatus == success
  -> LockOrder(trade_no)
  -> model.RechargeEpay / Recharge
  -> 返回 success/fail
```

`LockOrder` 是进程内订单锁，配合 DB 事务和状态检查减少重复处理风险。

## 六、Stripe / Creem / Waffo

这些网关的控制器分别在：

```text
controller/topup_stripe.go
controller/topup_creem.go
controller/topup_waffo.go
controller/topup_waffo_pancake.go
```

共同模式：

```text
RequestXPay
  -> 校验网关配置
  -> 校验金额和折扣
  -> 创建 pending TopUp
  -> 调 provider SDK/API 创建订单
  -> 返回 payment_url

XWebhook
  -> 检查 webhook 是否可用
  -> 读取 body
  -> 验签
  -> 解析事件
  -> 只处理支付成功事件
  -> 完成充值
  -> 返回 provider 要求的响应
```

Waffo 还有支付方式列表、sandbox 配置、notify/return URL 覆盖等逻辑。

Waffo Pancake 还会校验路由中的 `:env` 和事件 `mode` 是否匹配，并校验 webhook buyer identity 与本地用户是否一致。这类额外校验可以防止测试/生产环境串单，或把别人的支付事件误绑定到当前用户。

## 七、订阅模型

订阅相关模型集中在 `model/subscription.go`。

### 1. SubscriptionPlan

订阅套餐：

```go
type SubscriptionPlan struct {
    Title string
    Subtitle string
    PriceAmount float64
    Currency string
    DurationUnit string
    DurationValue int
    CustomSeconds int64
    Enabled bool
    AllowBalancePay *bool
    AllowWalletOverflow *bool
    StripePriceId string
    CreemProductId string
    WaffoPancakeProductId string
    MaxPurchasePerUser int
    UpgradeGroup string
    DowngradeGroup string
    TotalAmount int64
    QuotaResetPeriod string
    QuotaResetCustomSeconds int64
}
```

关键配置：

- `TotalAmount = 0` 表示不限量。
- `AllowBalancePay` 控制是否允许用钱包余额购买。
- `AllowWalletOverflow` 控制订阅额度不足时是否回退钱包。
- `UpgradeGroup` 购买后升级用户分组。
- `DowngradeGroup` 过期后降级到指定分组。
- `QuotaResetPeriod` 控制每日/每周/每月/自定义重置。

### 2. SubscriptionOrder

订阅订单：

```go
type SubscriptionOrder struct {
    UserId int
    PlanId int
    Money float64
    TradeNo string
    PaymentMethod string
    PaymentProvider string
    Status string
    ProviderPayload string
}
```

它和充值 `TopUp` 类似，也是 pending -> success/expired。

### 3. UserSubscription

用户实际拥有的订阅实例：

```text
user_id
plan_id
amount_total
amount_used
start_time
end_time
status
source
last_reset_time
next_reset_time
upgrade_group
prev_user_group
downgrade_group
allow_wallet_overflow
```

购买成功后创建的是 `UserSubscription`，而不是直接改 `SubscriptionPlan`。

### 4. SubscriptionPreConsumeRecord

订阅额度预扣幂等记录：

```go
type SubscriptionPreConsumeRecord struct {
    RequestId string
    UserId int
    UserSubscriptionId int
    PreConsumed int64
    Status string
}
```

它保证同一个 request id 的订阅预扣不会被重复执行。

## 八、创建订阅实例

核心函数：

```text
CreateUserSubscriptionFromPlanTx
```

它在事务中：

1. 检查购买上限。
2. 根据套餐持续时间计算 `EndTime`。
3. 根据 reset period 计算 `NextResetTime`。
4. 如果有 `UpgradeGroup`，保存用户原分组并更新用户分组。
5. 设置 `AllowWalletOverflow`。
6. 创建 `UserSubscription`。

为什么带 `Tx`？

因为创建订阅往往和订单完成、余额扣除、用户分组升级在同一个事务里，不能拆开。

## 九、订阅订单完成

核心函数：

```text
CompleteSubscriptionOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod)
```

流程：

```text
DB.Transaction
  -> FOR UPDATE 查 SubscriptionOrder
  -> 校验 expectedPaymentProvider
  -> success 则幂等返回
  -> 非 pending 则报错
  -> 查 plan
  -> CreateUserSubscriptionFromPlanTx
  -> upsertSubscriptionTopUpTx
  -> order.Status = success
  -> order.ProviderPayload = payload
  -> 保存实际 payment method
事务后：
  -> 更新用户分组缓存
  -> RecordLog
```

`upsertSubscriptionTopUpTx` 会把订阅购买同步到 `TopUp` 表，便于账单/充值记录统一展示。

## 十、用余额购买订阅

入口：

```text
POST /api/subscription/balance/pay
  -> controller.SubscriptionRequestBalancePay
  -> model.PurchaseSubscriptionWithBalance
```

流程：

```text
检查 plan 是否启用
检查 AllowBalancePay
priceAmount * QuotaPerUnit 得到 requiredQuota
FOR UPDATE 锁用户
检查 user.quota
扣 user.quota
CreateUserSubscriptionFromPlanTx
记录日志
```

这条路径没有外部 webhook，但仍然必须在事务中完成。

## 十一、在线购买订阅

不同 provider 的入口：

```text
SubscriptionRequestEpay
SubscriptionRequestStripePay
SubscriptionRequestCreemPay
SubscriptionRequestWaffoPancakePay
```

共同流程：

```text
requirePaymentCompliance
  -> 校验 plan
  -> 校验 provider 配置
  -> 检查购买上限
  -> 创建 SubscriptionOrder pending
  -> 创建 provider 订单/checkout
  -> 返回支付链接或参数
```

回调完成时最终都会落到：

```text
model.CompleteSubscriptionOrder
```

失败、过期、取消则可能调用：

```text
model.ExpireSubscriptionOrder
```

## 十二、订阅参与 relay 计费

订阅与普通钱包计费在 `service/billing_session.go` 里统一。

入口：

```text
service.NewBillingSession(c, relayInfo, preConsumedQuota)
```

用户偏好来自：

```text
user setting BillingPreference
```

支持：

```text
subscription_only
wallet_only
wallet_first
subscription_first
```

默认是 `subscription_first`。

选择逻辑：

```text
subscription_only
  -> 只能使用订阅

wallet_only
  -> 只能使用钱包

wallet_first
  -> 先钱包，不足时尝试订阅

subscription_first
  -> 有活跃订阅则先订阅
  -> 订阅额度不足时，如果 AllowWalletOverflow=true，则回退钱包
  -> 没有活跃订阅则钱包
```

钱包资金源：

```text
WalletFunding
  -> model.DecreaseUserQuota
  -> model.IncreaseUserQuota
```

订阅资金源：

```text
SubscriptionFunding
  -> model.PreConsumeUserSubscription
  -> model.PostConsumeUserSubscriptionDelta
  -> model.RefundSubscriptionPreConsume
```

这层抽象来自 `FundingSource`。它把“钱包怎么扣”和“订阅怎么扣”的差异压在资金来源实现里，让 relay 主流程只关心 `BillingSession` 的 `preConsume`、`Settle`、`Refund`。

## 十三、订阅预扣与幂等

订阅预扣核心函数：

```text
PreConsumeUserSubscription(requestId, userId, modelName, quotaType, amount)
```

它要求：

- `requestId` 非空。
- `amount > 0`。
- 用户有 active subscription。

事务内流程：

```text
查 SubscriptionPreConsumeRecord by request_id
  -> 已存在且未 refunded：返回已有结果
  -> 不存在：FOR UPDATE 查活跃订阅
  -> 根据 reset 规则可能先重置
  -> 找到额度足够的订阅
  -> 创建 pre-consume record
  -> sub.amount_used += amount
```

退款：

```text
RefundSubscriptionPreConsume(requestId)
  -> FOR UPDATE 查 record
  -> 如果已 refunded：幂等返回
  -> PostConsumeUserSubscriptionDelta(-preConsumed)
  -> record.Status = refunded
```

后结算：

```text
BillingSession.Settle(actualQuota)
  -> delta = actual - preConsumed
  -> funding.Settle(delta)
  -> token 额度补扣/返还
```

如果实际用量比预扣更高，订阅的 `AmountUsed` 会增加；如果更低，会减少。

## 十四、订阅重置和过期

后台任务入口：

```text
service.StartSubscriptionQuotaResetTask()
```

它只在 master 节点运行，每分钟 tick：

```text
runSubscriptionQuotaResetOnce
  -> model.ExpireDueSubscriptions
  -> model.ResetDueSubscriptions
  -> model.CleanupSubscriptionPreConsumeRecords
```

`ExpireDueSubscriptions`：

- 找到 `end_time <= now` 的 active 订阅。
- 标记 expired。
- 如果没有其他 active upgraded subscription，则按 `DowngradeGroup` 或 `PrevUserGroup` 回退用户分组。
- 更新用户缓存。

`ResetDueSubscriptions`：

- 找到 `next_reset_time <= now` 的 active 订阅。
- 根据 plan 的 reset period 把 `amount_used` 归零。
- 推进 `last_reset_time` 和 `next_reset_time`。

`CleanupSubscriptionPreConsumeRecords`：

- 删除旧的幂等记录，默认保留 7 天。

## 十五、缓存

订阅计划有缓存：

```text
subscriptionPlanCache
subscriptionPlanInfoCache
```

底层用 `cachex.HybridCache`，可同时使用内存和 Redis。

相关函数：

```text
InvalidateSubscriptionPlanCache(planId)
GetSubscriptionPlanInfoByUserSubscriptionId
```

管理员修改套餐后要清缓存，否则用户页面和日志展示可能看到旧标题或旧 plan 信息。

## 十六、Go 学习点

支付和订阅源码适合学习这些 Go 实战主题：

1. 外部 webhook 必须验签。
2. 订单完成必须幂等。
3. 资金相关更新要放进 DB 事务。
4. 用 `FOR UPDATE` 锁住订单或用户余额。
5. 用 `expectedPaymentProvider` 防止跨网关回调完成订单。
6. 用唯一 `trade_no` 做订单定位。
7. 用 `request_id` 做订阅预扣幂等。
8. 用 `decimal` 或整数 quota 避免金额精度误差。
9. 用后台任务处理周期性过期和重置。
10. 用缓存失效保证套餐修改后读路径更新。
11. 区分入站支付 webhook 和出站用户通知 webhook：`service/webhook.go` 是给用户配置 URL 发通知，带 HMAC 签名和 SSRF 校验，不是支付网关回调。

## 十七、阅读练习

### 练习 1：追一次充值

从 `controller/topup_stripe.go` 或 `controller/topup_waffo.go` 开始：

```text
RequestXPay
  -> TopUp pending
  -> XWebhook
  -> model.RechargeX
  -> users.quota 增加
  -> RecordTopupLog
```

回答：重复 webhook 为什么不会重复加余额？

### 练习 2：追一次订阅购买

选择 Epay：

```text
SubscriptionRequestEpay
  -> SubscriptionOrder pending
  -> SubscriptionEpayNotify
  -> CompleteSubscriptionOrder
  -> CreateUserSubscriptionFromPlanTx
```

回答：为什么完成订单时还要检查 `expectedPaymentProvider`？

### 练习 3：追一次订阅扣费

从 `service.NewBillingSession` 开始：

```text
subscription_first
  -> model.HasActiveUserSubscription
  -> SubscriptionFunding.PreConsume
  -> PreConsumeUserSubscription
  -> BillingSession.Settle
```

回答：为什么订阅不能使用钱包那套 trust quota 旁路？

### 练习 4：追一次订阅过期

从 `service/subscription_reset_task.go` 开始：

```text
StartSubscriptionQuotaResetTask
  -> ExpireDueSubscriptions
  -> 用户分组回退
```

回答：如果用户还有另一个 active upgraded subscription，为什么不能立刻降级？
