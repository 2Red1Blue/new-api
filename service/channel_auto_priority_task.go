package service

import (
	"context"
	"fmt"
	"math"
	"os"
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
	Scanned     int  `json:"scanned"`
	Applied     int  `json:"applied"`
	RatioSynced int  `json:"ratio_synced"`
	RatioFailed int  `json:"ratio_failed"`
	Skipped     bool `json:"skipped"`
}

type ChannelAutoPriorityScanStatus struct {
	Enabled       bool    `json:"enabled"`
	Scheduled     bool    `json:"scheduled"`
	Running       bool    `json:"running"`
	IsMasterNode  bool    `json:"is_master_node"`
	NodeType      string  `json:"node_type"`
	IntervalHours float64 `json:"interval_hours"`
	Reason        string  `json:"reason"`
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
			logger.LogInfo(context.Background(), "channel auto-priority scan task skipped: current node is not master")
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

func GetChannelAutoPriorityScanStatus() ChannelAutoPriorityScanStatus {
	setting := operation_setting.GetMonitorSetting()
	nodeType := os.Getenv("NODE_TYPE")
	if nodeType == "" {
		nodeType = "master"
	}

	status := ChannelAutoPriorityScanStatus{
		Enabled:       setting.AutoPriorityScanEnabled,
		Scheduled:     common.IsMasterNode && setting.AutoPriorityScanEnabled,
		Running:       channelAutoPriorityScanRunning.Load(),
		IsMasterNode:  common.IsMasterNode,
		NodeType:      nodeType,
		IntervalHours: setting.AutoPriorityScanIntervalHours,
	}
	switch {
	case !common.IsMasterNode:
		status.Reason = "Current node is not master, so scheduled scans do not run here."
	case !setting.AutoPriorityScanEnabled:
		status.Reason = "Background auto priority scan is disabled."
	default:
		status.Reason = "Scheduled scans are active on this node."
	}
	return status
}

func RunChannelAutoPriorityScanOnce() (ChannelAutoPriorityScanResult, error) {
	if !channelAutoPriorityScanRunning.CompareAndSwap(false, true) {
		return ChannelAutoPriorityScanResult{Skipped: true}, nil
	}
	defer channelAutoPriorityScanRunning.Store(false)

	ctx := context.Background()

	// 第一步：拉取所有渠道的上游分组倍率并写入数据库
	ratioSynced, ratioFailed, syncErr := SyncAllChannelUpstreamGroupRatios(ctx, channelAutoPriorityScanBatchSize)
	if syncErr != nil {
		return ChannelAutoPriorityScanResult{}, syncErr
	}
	if common.DebugEnabled || ratioSynced > 0 || ratioFailed > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("upstream group ratio sync finished: synced=%d failed=%d", ratioSynced, ratioFailed))
	}

	// 第二步：重新计算所有渠道的自动优先级
	scanned, applied, err := model.RecalculateAllChannelAutoPriorities(channelAutoPriorityScanBatchSize)
	if err != nil {
		return ChannelAutoPriorityScanResult{}, err
	}
	if common.DebugEnabled || applied > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("channel auto-priority scan finished: scanned=%d applied=%d", scanned, applied))
	}
	return ChannelAutoPriorityScanResult{
		Scanned:     scanned,
		Applied:     applied,
		RatioSynced: ratioSynced,
		RatioFailed: ratioFailed,
	}, nil
}
