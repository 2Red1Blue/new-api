# 创建专属 Key

专属 Key 是你使用 laiber.cloud 的访问凭证。Claude Code、Codex、Cherry Studio 和其他客户端都需要使用它完成鉴权。

## 创建步骤

1. 登录 `https://ai.laiber.cloud`。
2. 进入 `令牌` / `API Keys` 页面。
3. 点击创建令牌。
4. 填写令牌名称，例如 `claude-code`、`codex`、`cherry-studio`。
5. 按用途选择分组、模型限制、额度和过期时间。
6. 保存后复制完整 Key。

::: warning 安全提醒
完整 Key 通常只显示一次。请妥善保存，不要发到公开聊天、截图、前端代码或 Git 仓库中。
:::

## 一键导入客户端

创建 Key 后，在 Key 列表右侧点击更多操作菜单，可以看到几个常用入口：

| 菜单项 | 说明 |
| --- | --- |
| `复制密钥` | 复制完整 Key，用于手动配置 |
| `复制连接信息` | 复制包含 Key 和服务地址的连接信息 |
| `CC Switch` | 打开导入弹窗，选择 Claude、Codex 或 Gemini 后一键导入 |
| `聊天` | 打开预设客户端，例如 Cherry Studio、AionUI、DeepChat、Lobe Chat 等 |

`CC Switch` 导入时需要选择应用类型和主模型。模型下拉来自你当前账号可用模型列表，也可以手动输入模型名称。

## 分组怎么选

不同工具可能需要不同分组或模型能力：

| 用途 | 建议 |
| --- | --- |
| Claude Code | 使用支持 Claude / Anthropic 协议的分组 |
| Codex | 使用支持 OpenAI Responses 或 Codex 的分组 |
| Cherry Studio | 使用 OpenAI 兼容分组 |
| 普通 API 调用 | 根据需要选择可用模型和价格合适的分组 |

如果你不确定该选哪个分组，先创建一个普通 OpenAI 兼容 Key 做测试，再根据工具文档切换到对应分组。

## 测试 Key

创建后可以用模型列表接口测试：

```bash
curl https://ai.laiber.cloud/v1/models \
  -H "Authorization: Bearer 你的_API_Key"
```

能返回模型列表，就说明 Key 和基础连接正常。

## 下一步

- [修改令牌设置](/zh/guide/modify-token)
- [CC-Switch 配置工具](/zh/tools/cc-switch)
- [Claude Code 部署指南](/zh/deploy/claude-code)
- [Codex 部署指南](/zh/deploy/codex)
