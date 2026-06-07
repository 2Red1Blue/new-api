package openai

import (
	"net/http"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesToChatStreamHandlerEOFBeforeCompletedReturnsError(t *testing.T) {
	setupResponsesStreamTest(t)

	c, _ := newResponsesStreamTestContext()
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesToChatStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.False(t, types.IsSkipRetryError(err))
	require.Contains(t, err.Error(), "closed before")
	require.Contains(t, err.Error(), ".completed")
}

func TestOaiResponsesToChatStreamHandlerStartedStreamEOFBeforeCompletedReturnsSkipRetryError(t *testing.T) {
	setupResponsesStreamTest(t)

	c, _ := newResponsesStreamTestContext()
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	resp := newResponsesStreamHTTPResponse(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		"",
	}, "\n"))

	usage, err := OaiResponsesToChatStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.True(t, types.IsSkipRetryError(err))
	require.Contains(t, err.Error(), "closed before")
	require.Contains(t, err.Error(), ".completed")
}
