package service

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertChannelAffinityFailureRecordKeepsDifferentChannels(t *testing.T) {
	records := []ChannelAffinityFailureRecord{
		{
			FailedChannelID: 1,
			Reason:          "unavailable_status",
			StatusCode:      503,
			ErrorType:       types.ErrorTypeUpstreamError,
			ErrorCode:       types.ErrorCodeBadResponseStatusCode,
		},
	}

	next := ChannelAffinityFailureRecord{
		FailedChannelID: 2,
		Reason:          "unavailable_status",
		StatusCode:      503,
		ErrorType:       types.ErrorTypeUpstreamError,
		ErrorCode:       types.ErrorCodeBadResponseStatusCode,
	}

	updated := upsertChannelAffinityFailureRecord(records, next)

	require.Len(t, updated, 2)
	assert.Equal(t, 1, updated[0].FailedChannelID)
	assert.Equal(t, 2, updated[1].FailedChannelID)
}

func TestUpsertChannelAffinityFailureRecordReplacesSameChannelSameType(t *testing.T) {
	records := []ChannelAffinityFailureRecord{
		{
			FailedChannelID:      34,
			Reason:               "unavailable_status",
			StatusCode:           503,
			ErrorType:            types.ErrorTypeUpstreamError,
			ErrorCode:            types.ErrorCodeBadResponseStatusCode,
			MessagePreview:       "first",
			CreatedAtUnixSeconds: 1,
		},
		{
			FailedChannelID:      32,
			Reason:               "unavailable_status",
			StatusCode:           503,
			ErrorType:            types.ErrorTypeUpstreamError,
			ErrorCode:            types.ErrorCodeBadResponseStatusCode,
			MessagePreview:       "other channel",
			CreatedAtUnixSeconds: 2,
		},
	}

	next := ChannelAffinityFailureRecord{
		FailedChannelID:      34,
		Reason:               "unavailable_status",
		StatusCode:           503,
		ErrorType:            types.ErrorTypeUpstreamError,
		ErrorCode:            types.ErrorCodeBadResponseStatusCode,
		MessagePreview:       "latest",
		CreatedAtUnixSeconds: 3,
	}

	updated := upsertChannelAffinityFailureRecord(records, next)

	require.Len(t, updated, 2)
	require.Equal(t, 32, updated[0].FailedChannelID)
	require.Equal(t, 34, updated[1].FailedChannelID)
	assert.Equal(t, "latest", updated[1].MessagePreview)
	assert.Equal(t, int64(3), updated[1].CreatedAtUnixSeconds)
}
