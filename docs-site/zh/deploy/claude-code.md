# Claude Code 部署指南

Claude Code 是 Anthropic 的命令行编程工具。通过 laiber.cloud 使用时，你需要准备一个支持 Claude / Anthropic 协议的专属 Key，并把工具的请求地址指向 laiber.cloud。

## 前置准备

1. 已注册并登录 `https://ai.laiber.cloud`。
2. 已创建 Claude Code 专用 Key。
3. 本机已安装 Node.js，或已使用官方安装脚本安装 Claude Code。

::: tip 推荐
如果你安装了 CC-Switch，优先在令牌页右侧菜单选择 `CC Switch`，应用类型选 `Claude`，再选择主模型一键导入。下面的环境变量方式适合作为手动备用方案。
:::

## 安装 Claude Code

推荐使用官方安装方式：

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

如果你的环境必须使用 npm，也可以安装：

```bash
npm install -g @anthropic-ai/claude-code
```

安装后验证：

```bash
claude -v
```

## 配置环境变量

将下面的 `sk-xxx` 替换为你的 Claude Code 专用 Key。

### macOS / Linux

如果你使用 zsh：

```bash
echo 'export ANTHROPIC_AUTH_TOKEN="sk-xxx"' >> ~/.zshrc
echo 'export ANTHROPIC_BASE_URL="https://ai.laiber.cloud"' >> ~/.zshrc
source ~/.zshrc
```

如果你使用 bash：

```bash
echo 'export ANTHROPIC_AUTH_TOKEN="sk-xxx"' >> ~/.bash_profile
echo 'export ANTHROPIC_BASE_URL="https://ai.laiber.cloud"' >> ~/.bash_profile
source ~/.bash_profile
```

### Windows PowerShell

```powershell
[Environment]::SetEnvironmentVariable("ANTHROPIC_AUTH_TOKEN", "sk-xxx", "User")
[Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", "https://ai.laiber.cloud", "User")
```

设置后重新打开终端。

## 启动 Claude Code

进入你的项目目录：

```bash
cd your-project
claude
```

如果能正常进入交互界面并返回响应，就说明配置成功。

## 常见问题

### 连接失败

- 确认 `ANTHROPIC_BASE_URL` 是 `https://ai.laiber.cloud`。
- 确认 Key 没有复制错。
- 确认 Key 的分组支持 Claude Code。
- 检查账号余额和额度。

### 提示没有权限

通常是 Key 分组、模型权限或额度问题。回到控制台检查令牌设置。

### 修改 Key 后不生效

重启终端和 Claude Code。环境变量通常在新终端会话中才会重新读取。
