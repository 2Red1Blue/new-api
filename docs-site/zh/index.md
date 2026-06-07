---
layout: home

hero:
  name: laiber.cloud
  text: AI API 中转站
  tagline: 一个账号，一个稳定入口，接入当前可用的 AI 编程与 OpenAI 兼容模型能力。
  actions:
    - theme: brand
      text: 开始使用
      link: /zh/intro/welcome
    - theme: alt
      text: 创建专属 Key
      link: /zh/guide/create-key
    - theme: alt
      text: 客户端配置
      link: /zh/clients/openai-compatible

features:
  - title: 统一入口
    details: 客户端只需配置一个 Base URL，具体可用模型以控制台模型列表为准。
  - title: 用量清晰
    details: 控制台可查看余额、令牌消耗、请求日志和可用模型。
  - title: 面向开发者
    details: 支持 OpenAI 兼容接入，也可配置 Claude Code、Codex 等 AI 编程工具。
  - title: 配置顺手
    details: 令牌菜单支持复制密钥、复制连接信息、导入 CC Switch，并可一键打开预设聊天客户端。
---

## 快速路径

第一次使用时，按这条路径最顺：

1. 打开 `https://ai.laiber.cloud` 并登录。
2. 创建专属 Key。
3. 在 Key 右侧菜单中选择 `CC Switch` 或 `聊天`，一键导入到目标工具。
4. 选择当前可用模型并发送测试请求。

## 常用地址

| 用途 | 地址 |
| --- | --- |
| 控制台 | `https://ai.laiber.cloud` |
| OpenAI 兼容 Base URL | `https://ai.laiber.cloud/v1` |
| 聊天补全 | `https://ai.laiber.cloud/v1/chat/completions` |
| 模型列表 | `https://ai.laiber.cloud/v1/models` |

## 快速导入

在控制台的 `令牌` / `API Keys` 页面，每个 Key 右侧都有更多操作菜单：

| 菜单项 | 用途 |
| --- | --- |
| `复制密钥` | 复制完整 API Key，适合手动配置客户端 |
| `复制连接信息` | 复制包含 Key 和服务地址的连接 JSON，适合支持连接信息导入的客户端 |
| `CC Switch` | 选择 Claude、Codex 或 Gemini，并生成一键导入配置 |
| `聊天` | 打开已配置的聊天客户端，例如 Cherry Studio、AionUI、DeepChat、Lobe Chat 等 |

如果你不想手动填写 Base URL 和 Key，优先使用这些导入入口。
