# OpenAI 兼容接入

大多数 OpenAI SDK 和兼容客户端都可以直接接入 laiber.cloud。

::: tip 更快的方式
如果你使用的是控制台 `聊天` 菜单中已有的客户端，例如 Cherry Studio、AionUI、DeepChat、Lobe Chat 等，可以直接在 Key 右侧菜单中选择对应客户端打开，无需手动复制 Key。
:::

## 基本配置

```text
Base URL: https://ai.laiber.cloud/v1
API Key: 你的 API Key
```

如果客户端支持导入连接信息，也可以在 Key 菜单中选择 `复制连接信息`，获得包含 Key 和服务地址的 JSON。

## curl

```bash
curl https://ai.laiber.cloud/v1/chat/completions \
  -H "Authorization: Bearer $LAIBER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "当前可用模型名称",
    "messages": [
      {
        "role": "user",
        "content": "用一句话介绍 laiber.cloud"
      }
    ]
  }'
```

## Node.js

```ts
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.LAIBER_API_KEY,
  baseURL: "https://ai.laiber.cloud/v1",
});

const response = await client.chat.completions.create({
  model: "当前可用模型名称",
  messages: [{ role: "user", content: "写一个简短的接口测试用例" }],
});

console.log(response.choices[0]?.message?.content);
```

## Python

```python
from openai import OpenAI
import os

client = OpenAI(
    api_key=os.environ["LAIBER_API_KEY"],
    base_url="https://ai.laiber.cloud/v1",
)

response = client.chat.completions.create(
    model="当前可用模型名称",
    messages=[{"role": "user", "content": "解释什么是 API 中转站"}],
)

print(response.choices[0].message.content)
```
