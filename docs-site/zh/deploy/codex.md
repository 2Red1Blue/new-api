# Codex 部署指南

Codex 是 OpenAI 的命令行编程工具。通过 laiber.cloud 使用时，需要把 Codex 的模型提供商配置为 laiber.cloud，并使用 Codex 专用 Key 鉴权。

## 前置准备

1. 已注册并登录 `https://ai.laiber.cloud`。
2. 已创建 Codex 专用 Key。
3. 本机已安装 Node.js 和 npm。

::: tip 推荐
如果你安装了 CC-Switch，优先在令牌页右侧菜单选择 `CC Switch`，应用类型选 `Codex`，再选择主模型一键导入。下面的 `config.toml` 方式适合作为手动备用方案。
:::

## 安装 Codex

```bash
npm install -g @openai/codex@latest
```

验证安装：

```bash
codex --version
```

## 创建配置目录

```bash
mkdir -p ~/.codex
cd ~/.codex
```

## 编写 config.toml

下面示例使用 laiber.cloud 的 OpenAI Responses 兼容入口。请按控制台实际可用模型调整 `model`。

```toml
model_provider = "laiber"
model = "gpt-5.5"
model_reasoning_effort = "high"
disable_response_storage = true

[model_providers.laiber]
name = "laiber.cloud"
base_url = "https://ai.laiber.cloud/v1"
wire_api = "responses"
requires_openai_auth = true
```

## 编写 auth.json

将 `sk-xxx` 替换为你的 Codex 专用 Key：

```json
{
  "OPENAI_API_KEY": "sk-xxx"
}
```

## 启动 Codex

进入项目目录：

```bash
cd your-project
codex
```

首次使用可以让 Codex 解释当前目录或生成一个小文件，确认请求能正常返回。

## 常见问题

### Codex 和 Claude Code 的 Key 可以共用吗

不建议共用。两者协议、模型和分组可能不同，最好分别创建 `codex` 和 `claude-code` 两个 Key。

### 配置文件放在哪里

| 系统 | 路径 |
| --- | --- |
| macOS / Linux | `~/.codex/` |
| Windows | `%USERPROFILE%\.codex\` |

### 提示认证失败

- 检查 `auth.json` 是否是合法 JSON。
- 检查 `OPENAI_API_KEY` 是否完整。
- 检查 Key 是否属于支持 Codex / Responses 的分组。

### 模型不可用

调用模型列表接口确认当前 Key 可用模型：

```bash
curl https://ai.laiber.cloud/v1/models \
  -H "Authorization: Bearer 你的_Codex_Key"
```
