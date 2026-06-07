package service

import (
	"fmt"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestAppendStreamStatusCompletedAfterClientGoneIsOK(t *testing.T) {
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(relaycommon.StreamEndReasonClientGone, fmt.Errorf("context canceled"))
	status.SetEndReason(relaycommon.StreamEndReasonDone, nil)

	other := map[string]interface{}{}
	appendStreamStatus(&relaycommon.RelayInfo{
		IsStream:     true,
		StreamStatus: status,
	}, other)

	streamInfo, ok := other["stream_status"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "ok", streamInfo["status"])
	require.Equal(t, string(relaycommon.StreamEndReasonDone), streamInfo["end_reason"])
	require.NotContains(t, streamInfo, "end_error")
}
