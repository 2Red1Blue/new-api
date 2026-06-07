# 常见问题

## Base URL 应该填什么

OpenAI 兼容客户端填写：

```text
https://ai.laiber.cloud/v1
```

如果客户端要求填写完整接口地址，再使用对应路径，例如 `/v1/chat/completions`。

## API Key 可以给别人用吗

不建议。每个人、每个应用或每个服务都应该创建独立 API Key，方便限制额度、追踪消耗和快速回收。

## 为什么某个模型不可用

可能原因包括：当前 Key 限制了模型、用户分组不可用、渠道关闭、上游服务异常或模型名称填写不一致。

## 为什么请求会变慢

常见原因包括：上游响应慢、网络链路波动、模型本身较慢、请求上下文过长、渠道正在重试。

## 部署文档站需要什么

这个文档站是 VitePress 静态站。构建后产物在 `docs-site/.vitepress/dist`，可以部署到 Nginx、Cloudflare Pages、Vercel、GitHub Pages 或任意静态文件服务。
