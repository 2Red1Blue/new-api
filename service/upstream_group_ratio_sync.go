package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// syncChannelUpstreamGroupRatio 拉取单个 profile 的上游分组倍率并更新数据库
func syncChannelUpstreamGroupRatio(ctx context.Context, profile *model.ChannelUpstreamProfile) error {
	baseURL := strings.TrimSpace(profile.UpstreamLoginUrl)
	if baseURL == "" {
		return nil
	}

	credential := UpstreamCredentialFromProfileRecord(profile)
	client := &http.Client{Timeout: 10 * time.Second}

	fetched, err := FetchUpstreamGroupRatios(ctx, client, baseURL, credential)
	if err != nil {
		return fmt.Errorf("获取分组倍率失败: %w", err)
	}

	groupName := strings.TrimSpace(profile.UpstreamGroup)
	groupRatio := float64(0)
	currentGroupDetail, hasCurrentGroupDetail := UpstreamGroupSnapshotEntry{}, false
	if groupName != "" {
		r, ok := fetched.Ratios[groupName]
		if ok {
			groupRatio = r
		} else {
			common.SysLog(fmt.Sprintf("upstream group %q not found in fetched ratios for channel #%d profile #%d, writing 0",
				groupName, profile.ChannelId, profile.Id))
		}
		currentGroupDetail, hasCurrentGroupDetail = fetched.Details[groupName]
	}

	now := common.GetTimestamp()
	updates := map[string]any{
		"upstream_group_ratio":  groupRatio,
		"upstream_group_ratios": fetched.Raw,
		"updated_at":            now,
	}
	if profile.UpstreamTopupRatio == 0 {
		if tr, ok, _ := FetchUpstreamTopupRatio(ctx, client, baseURL); ok && tr > 0 {
			updates["upstream_topup_ratio"] = tr
		}
	}

	if err := model.DB.Model(&model.ChannelUpstreamProfile{}).
		Where("id = ?", profile.Id).
		Updates(updates).Error; err != nil {
		return err
	}

	shouldUpdateRPM := groupName == ""
	rpmLimit := 0
	if groupName != "" && hasCurrentGroupDetail && currentGroupDetail.HasRPMLimit {
		shouldUpdateRPM = true
		rpmLimit = currentGroupDetail.RPMLimit
	}
	if !shouldUpdateRPM {
		return nil
	}

	var channel model.Channel
	if err := model.DB.First(&channel, "id = ?", profile.ChannelId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	settings := channel.GetOtherSettings()
	settings.UpstreamRPMLimit = rpmLimit
	channel.SetOtherSettings(settings)
	return model.DB.Model(&model.Channel{}).
		Where("id = ?", channel.Id).
		Update("settings", channel.OtherSettings).Error
}

// SyncAllChannelUpstreamGroupRatios 批量拉取所有配置了上游登录地址的渠道分组倍率并写入数据库
// 返回：已同步数量、失败数量、首个致命错误
func SyncAllChannelUpstreamGroupRatios(ctx context.Context, batchSize int) (synced int, failed int, err error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	var lastID int64
	for {
		var profiles []model.ChannelUpstreamProfile
		if err = model.DB.
			Where("id > ? AND upstream_login_url != ''", lastID).
			Order("id ASC").
			Limit(batchSize).
			Find(&profiles).Error; err != nil {
			return synced, failed, err
		}
		if len(profiles) == 0 {
			break
		}
		for i := range profiles {
			lastID = profiles[i].Id
			if syncErr := syncChannelUpstreamGroupRatio(ctx, &profiles[i]); syncErr != nil {
				common.SysLog(fmt.Sprintf("sync upstream group ratio failed for channel #%d profile #%d: %s",
					profiles[i].ChannelId, profiles[i].Id, syncErr.Error()))
				failed++
			} else {
				synced++
			}
		}
		if len(profiles) < batchSize {
			break
		}
	}
	return synced, failed, nil
}
