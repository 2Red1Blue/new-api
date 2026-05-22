package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type channelRPMWindow struct {
	windowStart int64
	count       int
}

var (
	channelRPMLimiterMu sync.Mutex
	channelRPMLimiters  = map[int]*channelRPMWindow{}
	channelRPMNowUnix   = func() int64 {
		return time.Now().Unix()
	}
)

func CheckAndReserveChannelRPM(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	limit := channel.GetOtherSettings().UpstreamRPMLimit
	if limit <= 0 {
		return true
	}

	nowWindow := channelRPMNowUnix() / 60
	channelRPMLimiterMu.Lock()
	defer channelRPMLimiterMu.Unlock()

	window := channelRPMLimiters[channel.Id]
	if window == nil || window.windowStart != nowWindow {
		for channelID, limiterWindow := range channelRPMLimiters {
			if limiterWindow == nil || limiterWindow.windowStart != nowWindow {
				delete(channelRPMLimiters, channelID)
			}
		}
		channelRPMLimiters[channel.Id] = &channelRPMWindow{
			windowStart: nowWindow,
			count:       1,
		}
		return true
	}
	if window.count >= limit {
		return false
	}
	window.count++
	return true
}
