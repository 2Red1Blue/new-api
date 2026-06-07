package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newResponsesStreamTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, recorder
}

func newResponsesStreamHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func setupResponsesStreamTest(t *testing.T) {
	t.Helper()

	origStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = origStreamingTimeout
	})
}

func TestOaiResponsesStreamHandlerEarlyFailureReturnsRetryableErrorWithoutWritingPendingEvents(t *testing.T) {
	setupResponsesStreamTest(t)

	c, recorder := newResponsesStreamTestContext()
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`data: {"type":"response.failed","response":{"id":"resp_1","error":{"type":"server_error","message":"We're currently experiencing high demand","code":"server_error"}}}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesStreamHandler(c, &relaycommon.RelayInfo{}, resp)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.False(t, types.IsSkipRetryError(err))
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandlerStartedWriterFlushesPendingBeforeFailure(t *testing.T) {
	setupResponsesStreamTest(t)

	c, recorder := newResponsesStreamTestContext()
	c.Writer.WriteHeaderNow()
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`data: {"type":"response.failed","response":{"id":"resp_1","error":{"type":"server_error","message":"We're currently experiencing high demand","code":"server_error"}}}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesStreamHandler(c, &relaycommon.RelayInfo{}, resp)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.True(t, types.IsSkipRetryError(err))
	body := recorder.Body.String()
	createdIndex := strings.Index(body, "event: response.created")
	failedIndex := strings.Index(body, "event: response.failed")
	require.NotEqual(t, -1, createdIndex, body)
	require.NotEqual(t, -1, failedIndex, body)
	require.Less(t, createdIndex, failedIndex, body)
}

func TestOaiResponsesStreamHandlerCompletedMarksStreamDone(t *testing.T) {
	setupResponsesStreamTest(t)

	c, recorder := newResponsesStreamTestContext()
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	require.Contains(t, recorder.Body.String(), "event: response.completed")
}

func TestOaiResponsesStreamHandlerEOFBeforeCompletedReturnsError(t *testing.T) {
	setupResponsesStreamTest(t)

	c, recorder := newResponsesStreamTestContext()
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.True(t, types.IsSkipRetryError(err))
	require.Contains(t, err.Error(), "closed before")
	require.Contains(t, err.Error(), ".completed")
	require.Contains(t, recorder.Body.String(), "event: response.failed")
}
