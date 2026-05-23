package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	channelAutoPriorityScanIdleInterval = time.Minute
	channelAutoPriorityScanBatchSize    = 200
)

var (
	channelAutoPriorityScanOnce    sync.Once
	channelAutoPriorityScanRunning atomic.Bool
)

type ChannelAutoPriorityScanResult struct {
	Scanned int  `json:"scanned"`
	Applied int  `json:"applied"`
	Skipped bool `json:"skipped"`
}

func normalizeAutoPriorityScanInterval(hours float64) time.Duration {
	if hours <= 0 || math.IsNaN(hours) || math.IsInf(hours, 0) {
		hours = 6
	}
	return time.Duration(math.Round(hours*60)) * time.Minute
}

func StartChannelAutoPriorityScanTask() {
	channelAutoPriorityScanOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), "channel auto-priority scan task started")
			for {
				setting := operation_setting.GetMonitorSetting()
				if !setting.AutoPriorityScanEnabled {
					time.Sleep(channelAutoPriorityScanIdleInterval)
					continue
				}

				interval := normalizeAutoPriorityScanInterval(setting.AutoPriorityScanIntervalHours)
				if _, err := RunChannelAutoPriorityScanOnce(); err != nil {
					logger.LogWarn(context.Background(), fmt.Sprintf("channel auto-priority scan failed: %v", err))
				}
				time.Sleep(interval)
			}
		})
	})
}

func RunChannelAutoPriorityScanOnce() (ChannelAutoPriorityScanResult, error) {
	if !channelAutoPriorityScanRunning.CompareAndSwap(false, true) {
		return ChannelAutoPriorityScanResult{Skipped: true}, nil
	}
	defer channelAutoPriorityScanRunning.Store(false)

	scanned, applied, err := model.RecalculateAllChannelAutoPriorities(channelAutoPriorityScanBatchSize)
	if err != nil {
		return ChannelAutoPriorityScanResult{}, err
	}
	if common.DebugEnabled || applied > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("channel auto-priority scan finished: scanned=%d applied=%d", scanned, applied))
	}
	return ChannelAutoPriorityScanResult{
		Scanned: scanned,
		Applied: applied,
	}, nil
}
