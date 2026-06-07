# Cherry Studio

Cherry Studio 支持 OpenAI 兼容服务商，可以直接连接 laiber.cloud。

## 一键打开

如果控制台的 `聊天` 菜单里配置了 Cherry Studio，你可以直接：

1. 打开 `令牌` / `API Keys` 页面。
2. 找到要使用的 Key。
3. 点击右侧更多操作菜单。
4. 展开 `聊天`。
5. 选择 `Cherry Studio`。

系统会自动把 Key 和服务地址带入预设链接。

## 手动配置

1. 打开 Cherry Studio 的 `设置`。
2. 新增一个 OpenAI 兼容服务商。
3. `API 地址` 填写 `https://ai.laiber.cloud/v1`。
4. `API Key` 填写控制台创建的 Key。
5. 点击获取模型，或手动填写模型名称。
6. 发送一条测试消息。

## 常见检查

- 如果模型列表为空，先确认 API Key 是否有效。
- 如果请求 401，检查 Key 是否复制完整。
- 如果模型不存在，进入控制台确认该 Key 是否允许访问该模型。
- 如果请求超时，可能是上游渠道不可用，需要查看控制台日志。
