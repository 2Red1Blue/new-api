package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled        bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes        float64 `json:"auto_test_channel_minutes"`
	AutoPriorityScanEnabled       bool    `json:"auto_priority_scan_enabled"`
	AutoPriorityScanIntervalHours float64 `json:"auto_priority_scan_interval_hours"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:        false,
	AutoTestChannelMinutes:        10,
	AutoPriorityScanEnabled:       false,
	AutoPriorityScanIntervalHours: 6,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	return &monitorSetting
}
