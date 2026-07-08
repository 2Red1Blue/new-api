# new-api Provider Adaptor 学习指南

这份文档专门讲 `relay/channel/*` 这一层：new-api 如何把统一的 OpenAI/Claude/Gemini 入口转换成不同上游 provider 请求，再把响应转换回客户端期望的格式。

读这块源码时要记住一句话：controller 和 relay helper 负责“编排”，provider adaptor 负责“协议差异”。

## 一、Adaptor 在整体链路中的位置

普通文本请求的大链路是：

```text
router/relay-router.go
  -> middleware.TokenAuth()
  -> middleware.Distribute()
  -> controller.Relay()
  -> relay.TextHelper()
     -> relay.GetAdaptor(info.ApiType)
     -> adaptor.Init(info)
     -> adaptor.ConvertOpenAIRequest()
     -> channel.DoApiRequest()
     -> adaptor.DoResponse()
     -> service.PostTextConsumeQuota()
```

`relay/channel/adapter.go` 定义统一接口：

```go
type Adaptor interface {
    Init(info *relaycommon.RelayInfo)
    GetRequestURL(info *relaycommon.RelayInfo) (string, error)
    SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
    ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
    ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error)
    ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error)
    ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error)
    ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error)
    ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error)
    DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
    DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
    GetModelList() []string
    GetChannelName() string
}
```

这就是 Go interface 的经典用法：调用方只依赖“能做什么”，不关心具体是 OpenAI、Claude、Gemini、Azure 还是其他 provider。

## 二、Adaptor 工厂：APIType 到实现

`relay/relay_adaptor.go` 的 `GetAdaptor(apiType int)` 是工厂函数。

```text
constant.APITypeOpenAI     -> openai.Adaptor
constant.APITypeAnthropic  -> claude.Adaptor
constant.APITypeGemini     -> gemini.Adaptor
constant.APITypeAws        -> aws.Adaptor
constant.APITypeCohere     -> cohere.Adaptor
...
```

它返回的是接口类型 `channel.Adaptor`，但实际值是某个 provider 的 struct 指针，比如 `&openai.Adaptor{}`。

读这段时可以练习 Go 的两件事：

- interface 返回值：函数签名是接口，返回具体类型。
- switch 工厂：用常量做路由，避免 controller import 所有 provider。

## 三、TextHelper 如何使用 Adaptor

`relay/compatible_handler.go` 的 `TextHelper()` 是最适合学习 adaptor 的入口。

简化流程：

```text
TextHelper(c, info)
  -> info.InitChannelMeta(c)
  -> textReq := info.Request.(*dto.GeneralOpenAIRequest)
  -> request := common.DeepCopy(textReq)
  -> helper.ModelMappedHelper(c, info, request)
  -> 处理 StreamOptions
  -> adaptor := GetAdaptor(info.ApiType)
  -> adaptor.Init(info)

  if passThrough:
    requestBody = 原始请求 body
  else:
    convertedRequest := adaptor.ConvertOpenAIRequest(c, info, request)
    relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
    common.Marshal(convertedRequest)
    relaycommon.RemoveDisabledFields(...)
    relaycommon.ApplyParamOverrideWithRelayInfo(...)
    requestBody = relaycommon.NewOutboundJSONBody(jsonData)

  resp := adaptor.DoRequest(c, info, requestBody)
  if resp.StatusCode != 200:
    service.RelayErrorHandler(...)
  usage := adaptor.DoResponse(c, resp, info)
  service.PostTextConsumeQuota(c, info, usage, nil)
```

这里有三个关键点：

1. 请求转换发生在 `ConvertOpenAIRequest()`，不是 controller。
2. 真正发 HTTP 发生在 `channel.DoApiRequest()`，不是 provider 自己完全手写。
3. 响应解析和格式转换发生在 `DoResponse()` 以及 provider handler。

## 四、统一 HTTP 请求发送：channel.DoApiRequest

`relay/channel/api_request.go` 是所有普通 HTTP provider 共用的发送层。

核心流程：

```text
DoApiRequest(adaptor, c, info, requestBody)
  -> adaptor.GetRequestURL(info)
  -> http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
  -> applyUpstreamContentLength(req, info)
  -> adaptor.SetupRequestHeader(c, &req.Header, info)
  -> processHeaderOverride(info, c)
  -> applyHeaderOverrideToRequest(req, overrides)
  -> doRequest(c, req, info)
```

### 4.1 Content-Length 细节

`applyUpstreamContentLength()` 会把 `info.UpstreamRequestBodySize` 写到 `req.ContentLength`。

原因是：当请求 body 被包装成通用 `io.Reader` 后，`net/http.NewRequest` 不一定能自动识别长度。如果不设置 `Content-Length`，Go 可能用 chunked transfer encoding，而有些上游不接受。

这是一个很真实的 Go 网络编程细节。

### 4.2 Header 的三层来源

上游 header 不是一个地方生成的，而是三层叠加：

1. `SetupApiRequestHeader()` 复制 Content-Type、Accept 等基础 header。
2. provider 的 `SetupRequestHeader()` 写认证 header，例如 OpenAI 的 `Authorization`、Claude 的 `x-api-key`、Gemini 的 `x-goog-api-key`。
3. channel header override 最后应用，优先级最高。

header override 支持：

- 普通 key/value 覆盖。
- `{api_key}` 占位符。
- `{client_header:<name>}` 从客户端请求 header 透传。
- `*` 透传所有安全 header。
- `re:<regex>` / `regex:<regex>` 按正则透传。

源码特意跳过 hop-by-hop header、cookie、host、content-length、authorization、x-api-key 等敏感或不应透传的 header。

### 4.3 doRequest 和 HTTP trace

`doRequest()` 负责选 HTTP client、启动 SSE ping、执行请求、记录上游请求时序。

如果渠道设置了 proxy，就用 `service.NewProxyHttpClient()`；否则用全局 `service.GetHttpClient()`。

`attachUpstreamHTTPTrace()` 使用 `net/http/httptrace` 记录：

- DNS start/done
- connect start/done
- TLS handshake
- connection reuse
- wrote headers/request
- first response byte

这些时序会写入 `RelayInfo`，用于日志和性能分析。

## 五、OpenAI Adaptor

OpenAI adaptor 在 `relay/channel/openai/adaptor.go` 和 `relay/channel/openai/relay-openai.go`。

### 5.1 URL 构造

`openai.Adaptor.GetRequestURL()` 处理很多变体：

- 普通 OpenAI：`ChannelBaseUrl + RequestURLPath`
- Azure：`/openai/deployments/{deployment}/{task}?api-version=...`
- Azure Realtime：`/openai/realtime?deployment=...&api-version=...`
- Azure Responses：`/openai/v1/responses` 或 `/openai/responses`
- Custom：直接把 `{model}` 替换成上游模型名
- Claude/Gemini relayFormat 走 OpenAI channel 时，转到 `/v1/chat/completions`

这说明 `ChannelType` 比 `APIType` 更具体：同样是 OpenAI adaptor，可能实际渠道是 OpenAI、Azure、OpenRouter、Custom。

### 5.2 Header 构造

`SetupRequestHeader()` 规则：

- Azure 写 `api-key`。
- OpenAI 写 `Authorization: Bearer ...`。
- OpenAI organization 写 `OpenAI-Organization`。
- Realtime 可能写 `Sec-WebSocket-Protocol` 或 `openai-beta`。
- OpenRouter 默认补 `HTTP-Referer` 和 `X-OpenRouter-Title`。

如果 header override 已经显式设置 Authorization，默认 Authorization 会跳过。

### 5.3 OpenAI 请求转换

`ConvertOpenAIRequest()` 里有很多模型兼容逻辑：

- 非 OpenAI/Azure 渠道清空 `StreamOptions`。
- OpenRouter 自动设置 `usage.include=true`。
- OpenRouter 的 `-thinking` 模型后缀转换成 reasoning 参数。
- OpenAI o 系列/GPT-5 系列把 `max_tokens` 转成 `max_completion_tokens`。
- o 系列和 GPT-5 去掉不支持的 `temperature`、`top_p`、`logprobs` 等参数。
- reasoning effort 后缀会写回 `request.ReasoningEffort`，并更新 `info.UpstreamModelName`。
- 部分 reasoning 模型把第一条 `system` message 改成 `developer`。

读这段时要特别留意“请求 DTO 的可选字段大多是指针”。因为 optional scalar 如果不是指针，`omitempty` 会把显式 `0`、`false` 误删。

### 5.4 音频和图片 multipart

OpenAI adaptor 的 `ConvertAudioRequest()`、`ConvertImageRequest()` 会根据 relay mode 构造 JSON 或 multipart body。

音频转写/翻译：

```text
ParseMultipartFormReusable(c)
  -> 复制 form fields
  -> 打开 file
  -> multipart.Writer 写文件
  -> c.Request.Header.Set("Content-Type", writer.FormDataContentType())
```

图片编辑类似，但支持 `image`、`image[]`、`image[0]` 等多图字段，并根据扩展名设置 MIME type。

这块适合学习：

- `mime/multipart.Writer`
- `io.Copy`
- 文件打开后及时关闭
- Content-Type boundary 必须由 writer 生成

## 六、OpenAI 响应处理

非流式入口是 `OpenaiHandler()`：

```text
io.ReadAll(resp.Body)
  -> common.Unmarshal(responseBody, &simpleResponse)
  -> 检查 OpenAI error
  -> 如果 usage 缺失则本地估算
  -> 根据 RelayFormat 转换响应:
     OpenAI -> 原样或 force format
     Claude -> service.ResponseOpenAI2Claude()
     Gemini -> service.ResponseOpenAI2Gemini()
  -> service.IOCopyBytesGracefully()
  -> 返回 usage
```

流式入口是 `OaiStreamHandler()`：

```text
helper.StreamScannerHandler(resp, callback)
  -> 每次收到 data，把上一条 stream data 写给下游
  -> processTokenData() 累积文本和 tool count
  -> handleLastResponse() 解析最后一条 usage / id / model
  -> usage 缺失则 ResponseText2Usage()
  -> HandleFinalResponse()
  -> 返回 usage
```

流式中有一个容易误解的细节：它会延迟一条 data 再发送。这样最后一条 data 可以被单独处理，用来提取 usage、response id、system fingerprint 等信息。

## 七、SSE 扫描器：StreamScannerHandler

`relay/helper/stream_scanner.go` 是所有 SSE 流式响应的基础设施。

它做了几件事：

1. 设置 `text/event-stream` 等响应 header。
2. 用 `bufio.Scanner` 按行读取上游响应。
3. 只处理 `data:` 和 `[DONE]`。
4. 用 `dataChan` 把扫描和处理拆成两个 goroutine。
5. 用 `writeMutex` 保护并发写下游。
6. 可选 ping 保活。
7. 监听客户端断开：`c.Request.Context().Done()`。
8. 用 `StreamStatus` 记录结束原因。

结束原因包括：

- 正常 `[DONE]`
- EOF
- 超时
- 客户端断开
- scanner 错误
- ping 失败
- panic

Go 学习点很多：

- `bufio.Scanner.Buffer()` 调整最大 token 大小。
- `context.WithCancel()` 协调 goroutine。
- channel 做 goroutine 间通信。
- `sync.WaitGroup` 等待退出。
- `sync.Mutex` 保护写响应。
- `defer` 做资源清理和 panic 保护。

## 八、Claude Adaptor

Claude adaptor 在 `relay/channel/claude/adaptor.go`。

### 8.1 URL 和 Header

URL 默认是：

```text
{ChannelBaseUrl}/v1/messages
```

如果 `IsClaudeBetaQuery` 或渠道设置 `ClaudeBetaQuery`，会追加 `?beta=true`。

Header：

- `x-api-key: {ApiKey}`
- `anthropic-version`：优先用客户端传入值，否则默认 `2023-06-01`
- `anthropic-beta`：可从客户端 header 透传
- `model_setting.GetClaudeSettings().WriteHeaders(...)` 写模型相关 beta header

### 8.2 请求转换

Claude 原生请求走 `ConvertClaudeRequest()`，基本原样返回。

OpenAI 兼容请求走：

```text
ConvertOpenAIRequest()
  -> RequestOpenAI2ClaudeMessage(c, *request)
```

也就是把 OpenAI 的 messages/tools/stream 等转换成 Claude Messages API 结构。

### 8.3 响应处理

`DoResponse()` 会设置：

```go
info.FinalRequestRelayFormat = types.RelayFormatClaude
```

然后根据 `info.IsStream` 调用：

- `ClaudeStreamHandler()`
- `ClaudeHandler()`

如果客户端原本是 OpenAI 格式，但渠道是 Claude，最终还可能在 handler 中转回 OpenAI 兼容响应。

## 九、Gemini Adaptor

Gemini adaptor 在 `relay/channel/gemini/adaptor.go`。

### 9.1 URL 构造

`GetRequestURL()` 根据模型和模式决定 action：

| 场景 | URL action |
| --- | --- |
| 图片模型 `imagen...` | `predict` |
| embedding | `embedContent` |
| batch embedding | `batchEmbedContents` |
| 普通非流式 | `generateContent` |
| 流式 | `streamGenerateContent?alt=sse` |

Gemini 流式原生路径下会设置 `info.DisablePing = true`，因为 Gemini SSE 自身行为和通用 ping 机制不完全一样。

它还会处理 thinking 相关模型后缀，例如 `-thinking`、`-nothinking`、reasoning effort 后缀，把对外模型名转换成真实上游模型名。

### 9.2 请求转换

Gemini 原生请求走 `ConvertGeminiRequest()`，主要补默认 role、修正 YouTube file mime type。

OpenAI 请求走：

```text
ConvertOpenAIRequest()
  -> CovertOpenAI2Gemini(c, *request, info)
```

Claude 请求则先借用 OpenAI adaptor：

```text
ClaudeRequest
  -> openai.Adaptor.ConvertClaudeRequest()
  -> dto.GeneralOpenAIRequest
  -> Gemini ConvertOpenAIRequest()
```

这个“先转中间格式，再转目标格式”的思路很常见：OpenAI 兼容 DTO 在项目里经常充当中间表示。

### 9.3 Embedding 和 Imagen

Gemini embedding 会把 OpenAI embedding input 解析成多条 `requests`，并强制 `info.IsGeminiBatchEmbedding = true`，确保 URL 使用 batch endpoint。

Imagen 会把 OpenAI image request 的 `size` 转成 Gemini 的 `aspectRatio`，把 `quality` 转成 `imageSize`。

## 十、Header Override 和 Pass Through 的区别

这两个概念容易混：

| 概念 | 位置 | 作用 |
| --- | --- | --- |
| PassThroughBody | `relay.TextHelper()` | 不做请求 DTO 转换，直接把客户端原始 body 发上游 |
| HeaderOverride | `channel.DoApiRequest()` | 在 provider 默认 header 后覆盖/透传 header |
| ParamOverride | `relay.TextHelper()` | 在转换后的 JSON body 上覆盖参数 |

如果开启 body pass-through，通常不会走 `ConvertOpenAIRequest()`，也不会走 `ParamOverride`。但 header override 仍然会在 `DoApiRequest()` 中应用。

## 十一、如何新增一个 Provider Adaptor

新增 provider 时可以按这个顺序读和写：

1. 在 `constant` 中确认 APIType/ChannelType。
2. 在 `relay/relay_adaptor.go` 的 `GetAdaptor()` 加分支。
3. 在 `relay/channel/<provider>/` 下实现 `Adaptor`。
4. 实现 `GetRequestURL()`：把 `RelayInfo` 转成上游 URL。
5. 实现 `SetupRequestHeader()`：认证 header、版本 header、特殊 beta header。
6. 实现必要的 `Convert*Request()`：至少 OpenAI 文本，按需要支持 Claude/Gemini/Image/Audio/Embedding。
7. `DoRequest()` 通常直接调用 `channel.DoApiRequest()` 或 `DoFormRequest()`。
8. `DoResponse()` 区分 stream/non-stream，返回 usage。
9. 如果支持 stream usage，确认 `StreamOptions` 支持并加入相应配置。
10. 确认计费 usage：如果上游不返回 usage，需要本地估算。

## 十二、读源码练习

1. 从 `relay.TextHelper()` 开始，跟一次 OpenAI 非流式请求到 `OpenaiHandler()`。
2. 从 `relay.TextHelper()` 开始，跟一次 OpenAI 流式请求到 `OaiStreamHandler()` 和 `StreamScannerHandler()`。
3. 从 `openai.Adaptor.GetRequestURL()` 找出 Azure chat completions 的最终 URL。
4. 从 `channel.DoApiRequest()` 找出 header override 为什么能覆盖默认 Authorization。
5. 从 `gemini.Adaptor.ConvertClaudeRequest()` 解释为什么 Claude 可以先转 OpenAI 再转 Gemini。
6. 找一个 multipart 请求，说明 `Content-Type` 为什么要由 `multipart.Writer` 设置。
7. 找一个 usage 缺失的响应路径，说明项目如何用本地 token 估算补齐 usage。

## 十三、常见误解

| 误解 | 正确理解 |
| --- | --- |
| 每个 provider 都自己完整发送 HTTP | 大多数 provider 复用 `channel.DoApiRequest()` |
| `APITypeOpenAI` 只代表 OpenAI 官方 | OpenAI adaptor 也覆盖 Azure、OpenRouter、Custom 等 OpenAI 兼容渠道 |
| Header override 在默认 header 之前 | 它在 provider header 之后应用，所以优先级最高 |
| pass-through 会跳过所有处理 | 它主要跳过 body 转换，header override 仍会生效 |
| SSE 扫描器只是一行行转发 | 它还负责超时、ping、客户端断开、结束原因、并发写保护 |
| 上游不返回 usage 就不能计费 | 项目会用响应文本和估算 prompt tokens 生成 usage |
| `RelayFormat` 和 `ChannelType` 是一回事 | RelayFormat 是客户端协议格式，ChannelType 是上游渠道类型 |
