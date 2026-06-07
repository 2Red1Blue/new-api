# 什么是 API 中转站

API 中转站位于客户端和模型服务之间。你把 Claude Code、Codex、Cherry Studio 或自己的程序连接到 laiber.cloud，laiber.cloud 再把请求转发到可用模型服务，并返回统一响应。

## 请求流程

```text
客户端
  -> laiber.cloud 网关
  -> 鉴权与模型匹配
  -> 模型服务
  -> 返回统一格式响应
```

## 它解决什么问题

- 一个 Key 可以接入多种工具。
- 客户端只需要填写固定 Base URL。
- 可以在控制台查看余额、模型、日志和用量。
- 切换工具或模型时，不需要到处修改复杂配置。

## 适合哪些人

- 使用 Claude Code、Codex 等 AI 编程工具的开发者。
- 想用 Cherry Studio、Open WebUI、LobeChat 等客户端的人。
- 想用统一 API Key 接入不同模型的用户。
- 需要查看自己的余额、请求日志和模型消耗的人。
