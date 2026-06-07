# 欢迎使用 laiber.cloud

laiber.cloud 是面向 AI 编程工具和 API 客户端的中转服务。你只需要注册账号、创建专属 Key，再把工具的请求地址指向 laiber.cloud，就可以开始使用 Claude Code、Codex 或 OpenAI 兼容客户端。

::: tip 提示
大多数客户端只需要两个配置：`Base URL` 和 `API Key`。如果你同时使用多个 AI 编程工具，可以用 CC-Switch 管理和切换配置。
:::

## 为什么使用中转站

### 一个稳定入口

应用侧统一请求 `https://ai.laiber.cloud/v1`，后续上游渠道切换、模型迁移、价格调整都可以在网关侧处理，减少客户端改动。

### 更清晰的用量

你可以在控制台查看余额、令牌消耗、请求日志和当前可用模型。

### 更灵活的模型使用

系统支持 OpenAI 兼容格式，也可以服务 Claude Code、Codex 等 AI 编程工具。具体可用模型以控制台和 `/v1/models` 返回结果为准。

## 第一次使用

1. 注册或登录控制台。
2. 创建一个专属 Key。
3. 根据工具选择 Claude Code、Codex 或 OpenAI 兼容配置。
4. 发送一次测试请求。

## 下一步

- 阅读 [什么是 API 中转站](/zh/intro/overview)
- [注册账号](/zh/guide/registration)
- [创建专属 Key](/zh/guide/create-key)
- [使用 CC-Switch 配置工具](/zh/tools/cc-switch)
