package openai

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	info.ObserveResponseModel(responsesResponse.Model)
	responseBody = rewriteSGLangResponsesCreatedAt(info, responseBody, "created_at", responsesResponse.CreatedAt)

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := &dto.Usage{}
	service.ApplyResponsesUsage(usage, responsesResponse.Usage)
	// Count actual tool invocations from Output (not tool declarations).
	for _, output := range responsesResponse.Output {
		switch output.Type {
		case dto.BuildInCallWebSearchCall:
			info.CountBillableToolCall(dto.BuildInCallWebSearchCall, "")
		case dto.BuildInCallFileSearchCall:
			info.CountBillableToolCall(dto.BuildInCallFileSearchCall, "")
		case dto.BuildInCallFunctionCall:
			info.CountBillableToolCall(dto.BuildInCallFunctionCall, output.Name)
		}
	}

	imageCounter := &relaycommon.ImageGenerationCallCounter{}
	if !relaycommon.IsNonBillableResponsesStatus(responsesResponse.Status) {
		for i := range responsesResponse.Output {
			idx := i
			imageCounter.Observe(&responsesResponse.Output[i], &idx)
		}
	}
	imageCounter.Commit(info)

	return usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	accumulator := service.NewResponsesUsageAccumulator(info)
	var streamErr *types.NewAPIError
	var pendingEvents []responsesStreamEvent
	forwarded := false
	completed := false

	flushPendingEvents := func() {
		for _, event := range pendingEvents {
			sendResponsesStreamData(c, event.response, event.data)
			forwarded = true
		}
		pendingEvents = nil
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if streamResponse.Response != nil {
			data = string(rewriteSGLangResponsesCreatedAt(info, []byte(data), "response.created_at", streamResponse.Response.CreatedAt))
		}
		if isResponsesStreamFailureEvent(streamResponse) {
			accumulator.Observe(&streamResponse)
			streamErr = responsesStreamFailureError(streamResponse)
			if forwarded || responseWriterStarted(c) {
				flushPendingEvents()
				sendResponsesStreamData(c, streamResponse, data)
				streamErr = types.WithOpenAIError(streamErr.ToOpenAIError(), streamErr.StatusCode, types.ErrOptionWithSkipRetry())
			}
			sr.Stop(streamErr)
			return
		}
		if !forwarded && isResponsesStreamEarlyLifecycleEvent(streamResponse.Type) {
			accumulator.Observe(&streamResponse)
			pendingEvents = append(pendingEvents, responsesStreamEvent{
				response: streamResponse,
				data:     data,
			})
			return
		}
		flushPendingEvents()
		sendResponsesStreamData(c, streamResponse, data)
		forwarded = true
		accumulator.Observe(&streamResponse)
		switch streamResponse.Type {
		case "response.completed", "response.done":
			completed = true
			sr.Done()
		case "error", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
			sr.Done()
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}
	if !completed && info != nil && info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonEOF {
		streamErr = responsesStreamClosedBeforeCompletedError()
		if forwarded || responseWriterStarted(c) {
			flushPendingEvents()
			streamResponse, data := responsesStreamClosedBeforeCompletedEvent(streamErr)
			sendResponsesStreamData(c, streamResponse, data)
			streamErr = types.WithOpenAIError(streamErr.ToOpenAIError(), streamErr.StatusCode, types.ErrOptionWithSkipRetry())
		}
		return nil, streamErr
	}
	flushPendingEvents()

	common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, info.StreamStatus)
	info.StreamStatus.RequireTerminal()
	return accumulator.Finish(), nil
}

func rewriteSGLangResponsesCreatedAt(info *relaycommon.RelayInfo, payload []byte, path string, createdAt dto.IntValue) []byte {
	if info.GetChannelType() != constant.ChannelTypeSGLang || !gjson.GetBytes(payload, path).Exists() {
		return payload
	}
	patched, err := sjson.SetBytes(payload, path, int(createdAt))
	if err != nil {
		return payload
	}
	return patched
}

type responsesStreamEvent struct {
	response dto.ResponsesStreamResponse
	data     string
}

func isResponsesStreamFailureEvent(event dto.ResponsesStreamResponse) bool {
	if event.Type == "response.error" {
		return true
	}
	// A response.failed event with the protocol status is a terminal result,
	// not a transport failure. Forward and settle it so clients receive the
	// upstream outcome and accounting retains any reported usage. Keep the
	// older retry path for malformed early failures that omit the status.
	return event.Type == "response.failed" && (event.Response == nil || gjson.ParseBytes(event.Response.Status).String() != "failed")
}

func isResponsesStreamEarlyLifecycleEvent(eventType string) bool {
	return eventType == "response.created" || eventType == "response.in_progress"
}

func responsesStreamFailureError(streamResponse dto.ResponsesStreamResponse) *types.NewAPIError {
	if streamResponse.Response != nil {
		if oaiErr := streamResponse.Response.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
			return types.WithOpenAIError(*oaiErr, http.StatusInternalServerError)
		}
	}
	return types.NewOpenAIError(fmt.Errorf("responses stream error: %s", streamResponse.Type), types.ErrorCodeBadResponse, http.StatusInternalServerError)
}

func responsesStreamClosedBeforeCompletedError() *types.NewAPIError {
	return types.NewOpenAIError(fmt.Errorf("responses stream closed before response.completed"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
}

func responsesStreamClosedBeforeCompletedEvent(streamErr *types.NewAPIError) (dto.ResponsesStreamResponse, string) {
	streamResponse := dto.ResponsesStreamResponse{
		Type: "response.failed",
		Response: &dto.OpenAIResponsesResponse{
			Error: streamErr.ToOpenAIError(),
		},
	}
	data, err := common.Marshal(streamResponse)
	if err != nil {
		return streamResponse, `{"type":"response.failed"}`
	}
	return streamResponse, string(data)
}

func responseWriterStarted(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written()
}
