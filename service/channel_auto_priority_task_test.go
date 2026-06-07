package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestGetChannelAutoPriorityScanStatus(t *testing.T) {
	setting := operation_setting.GetMonitorSetting()
	originalEnabled := setting.AutoPriorityScanEnabled
	originalInterval := setting.AutoPriorityScanIntervalHours
	originalIsMasterNode := common.IsMasterNode
	t.Cleanup(func() {
		setting.AutoPriorityScanEnabled = originalEnabled
		setting.AutoPriorityScanIntervalHours = originalInterval
		common.IsMasterNode = originalIsMasterNode
	})

	setting.AutoPriorityScanEnabled = true
	setting.AutoPriorityScanIntervalHours = 3
	common.IsMasterNode = false
	t.Setenv("NODE_TYPE", "slave")

	status := GetChannelAutoPriorityScanStatus()
	if status.Scheduled {
		t.Fatalf("slave node should not schedule scans")
	}
	if status.NodeType != "slave" {
		t.Fatalf("NodeType = %q, want slave", status.NodeType)
	}

	common.IsMasterNode = true
	t.Setenv("NODE_TYPE", "")
	status = GetChannelAutoPriorityScanStatus()
	if !status.Scheduled {
		t.Fatalf("master node with enabled setting should schedule scans")
	}
	if status.NodeType != "master" {
		t.Fatalf("NodeType = %q, want master", status.NodeType)
	}
	if status.IntervalHours != 3 {
		t.Fatalf("IntervalHours = %v, want 3", status.IntervalHours)
	}
}

func TestNormalizeAutoPriorityScanInterval(t *testing.T) {
	tests := []struct {
		name  string
		hours float64
		want  time.Duration
	}{
		{name: "default on zero", hours: 0, want: 6 * time.Hour},
		{name: "default on negative", hours: -1, want: 6 * time.Hour},
		{name: "half hour", hours: 0.5, want: 30 * time.Minute},
		{name: "rounds to minutes", hours: 1.25, want: 75 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAutoPriorityScanInterval(tt.hours); got != tt.want {
				t.Fatalf("normalizeAutoPriorityScanInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}
