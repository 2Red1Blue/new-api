# CC-Switch 配置工具

CC-Switch 是一个用于切换 AI 编程工具配置的桌面工具。laiber.cloud 的令牌页已经内置 `CC Switch` 导入入口，可以把 Key、服务地址和模型配置直接写入 CC-Switch。

## 适合谁使用

- 经常在 Claude Code 和 Codex 之间切换。
- 不想手动改环境变量或配置文件。
- 需要维护多个 API Key、多个服务地址。
- 希望通过托盘菜单快速切换当前工具配置。

## 安装

前往 CC-Switch 发布页下载对应系统安装包：

```text
https://github.com/farion1231/cc-switch/releases
```

按你的系统选择：

| 系统 | 建议 |
| --- | --- |
| Windows | 下载 `.exe` 或安装包 |
| macOS | 下载 `.dmg` |
| Linux | 下载 `.deb` 或 AppImage |

## 一键导入 laiber.cloud 配置

推荐使用控制台的一键导入：

1. 打开 `https://ai.laiber.cloud` 并登录。
2. 进入 `令牌` / `API Keys` 页面。
3. 找到要使用的 Key，点击右侧更多操作菜单。
4. 选择 `CC Switch`。
5. 在弹窗中选择应用类型：`Claude`、`Codex` 或 `Gemini`。
6. 选择或输入主模型。
7. 点击 `打开 CC Switch` 完成导入。

::: tip 提示
Codex 的导入地址会自动使用 `/v1`，Claude 和 Gemini 会使用站点根地址。通常不需要你手动填写 Base URL。
:::

## 手动配置备用

如果你的浏览器没有成功唤起 CC-Switch，可以手动新增提供商配置。

### Claude Code 配置

```text
名称: laiber Claude
Base URL: https://ai.laiber.cloud
API Key: 你的 Claude Code 专用 Key
```

### Codex 配置

```text
名称: laiber Codex
Base URL: https://ai.laiber.cloud/v1
API Key: 你的 Codex 专用 Key
```

::: tip 提示
Claude Code 和 Codex 使用的协议与配置不同，建议分别创建 Key，也分别保存配置。
:::

## 启用配置

1. 在 CC-Switch 提供商列表中选择 laiber.cloud 配置。
2. 点击启用。
3. 关闭并重新打开对应 AI 工具。
4. 发送一个简单问题测试是否可用。

## 常见问题

### 切换后仍然走旧配置

请重启 Claude Code 或 Codex。某些工具只在启动时读取环境变量或配置文件。

### Key 明明可用但工具报错

确认这个 Key 的分组是否匹配当前工具。Claude Code、Codex、普通 OpenAI 兼容客户端可能需要不同分组。

### 浏览器没有打开 CC-Switch

确认 CC-Switch 已安装，并允许浏览器打开 `ccswitch://` 协议链接。也可以改用上面的手动配置方式。

## 下一步

- [Claude Code 部署指南](/zh/deploy/claude-code)
- [Codex 部署指南](/zh/deploy/codex)
