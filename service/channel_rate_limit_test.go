package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func resetChannelRPMLimiterForTest(t *testing.T, now int64) {
	t.Helper()
	channelRPMLimiterMu.Lock()
	previousNow := channelRPMNowUnix
	channelRPMLimiters = map[int]*channelRPMWindow{}
	channelRPMNowUnix = func() int64 {
		return now
	}
	channelRPMLimiterMu.Unlock()

	t.Cleanup(func() {
		channelRPMLimiterMu.Lock()
		channelRPMLimiters = map[int]*channelRPMWindow{}
		channelRPMNowUnix = previousNow
		channelRPMLimiterMu.Unlock()
	})
}

func channelWithRPM(id int, rpm int) *model.Channel {
	channel := &model.Channel{Id: id}
	channel.SetOtherSettings(dto.ChannelOtherSettings{UpstreamRPMLimit: rpm})
	return channel
}

func TestCheckAndReserveChannelRPMAllowsUnlimited(t *testing.T) {
	resetChannelRPMLimiterForTest(t, 1_000)
	channel := channelWithRPM(1, 0)

	for range 10 {
		require.True(t, CheckAndReserveChannelRPM(channel))
	}
}

func TestCheckAndReserveChannelRPMRejectsAfterLimit(t *testing.T) {
	resetChannelRPMLimiterForTest(t, 1_000)
	channel := channelWithRPM(2, 2)

	require.True(t, CheckAndReserveChannelRPM(channel))
	require.True(t, CheckAndReserveChannelRPM(channel))
	require.False(t, CheckAndReserveChannelRPM(channel))
}

func TestCheckAndReserveChannelRPMResetsOnNextMinute(t *testing.T) {
	now := int64(1_000)
	resetChannelRPMLimiterForTest(t, now)
	channel := channelWithRPM(3, 1)

	require.True(t, CheckAndReserveChannelRPM(channel))
	require.False(t, CheckAndReserveChannelRPM(channel))

	channelRPMLimiterMu.Lock()
	channelRPMNowUnix = func() int64 {
		return now + 60
	}
	channelRPMLimiterMu.Unlock()

	require.True(t, CheckAndReserveChannelRPM(channel))
}

func TestCheckAndReserveChannelRPMCleansExpiredWindows(t *testing.T) {
	now := int64(1_000)
	resetChannelRPMLimiterForTest(t, now)

	require.True(t, CheckAndReserveChannelRPM(channelWithRPM(10, 1)))
	require.True(t, CheckAndReserveChannelRPM(channelWithRPM(11, 1)))

	channelRPMLimiterMu.Lock()
	channelRPMNowUnix = func() int64 {
		return now + 60
	}
	channelRPMLimiterMu.Unlock()

	require.True(t, CheckAndReserveChannelRPM(channelWithRPM(12, 1)))

	channelRPMLimiterMu.Lock()
	defer channelRPMLimiterMu.Unlock()
	require.Len(t, channelRPMLimiters, 1)
	require.NotNil(t, channelRPMLimiters[12])
}
