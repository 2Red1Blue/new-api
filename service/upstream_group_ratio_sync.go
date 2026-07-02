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
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

func DisableChannelWhenUpstreamGroupMissing(profile *model.ChannelUpstreamProfile, reasonPrefix string) error {
	if profile == nil {
		return nil
	}
	if profile.UpstreamGroup == "" {
		return nil
	}
	if strings.TrimSpace(profile.UpstreamGroupRatios) == "" {
		return nil
	}
	if !model.IsUpstreamGroupMissing(profile.UpstreamGroupRatios, profile.UpstreamGroup) {
		return nil
	}
	channel, err := model.GetChannelById(profile.ChannelId, true)
	if err != nil {
		return err
	}
	if channel.Status != common.ChannelStatusEnabled {
		return nil
	}
	reason := fmt.Sprintf("%s: upstream group %q not found in fetched ratios", reasonPrefix, profile.UpstreamGroup)
	common.SysLog(fmt.Sprintf("disabling channel #%d because upstream group %q is missing from ratios", channel.Id, profile.UpstreamGroup))
	serviceErr := types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan())
	DisableChannel(*serviceErr, reason)
	return nil
}

// syncChannelUpstreamGroupRatio 拉取单个 profile 的上游分组倍率并更新数据库
func syncChannelUpstreamGroupRatio(ctx context.Context, profile *model.ChannelUpstreamProfile) error {
	// 优先从 UpstreamIdentity 获取 baseURL，其次从 profile 的 login_url 或 channel 的 base_url
	baseURL := strings.TrimSpace(profile.UpstreamLoginUrl)
	if baseURL == "" {
		// 尝试从 identity 获取
		if identity, err := profile.ResolveIdentity(); err == nil && identity != nil {
			baseURL = strings.TrimSpace(identity.BaseURL)
		}
	}
	if baseURL == "" {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// 优先使用 profile-aware 入口（支持 session auth）
	fetched, err := FetchUpstreamGroupRatiosFromProfile(ctx, client, baseURL, profile)
	if err != nil {
		// 回退到纯 credential 方式
		credential := UpstreamCredentialFromProfileRecord(profile)
		fetched, err = FetchUpstreamGroupRatios(ctx, client, baseURL, credential)
		if err != nil {
			return fmt.Errorf("获取分组倍率失败: %w", err)
		}
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
	profile.UpstreamGroupRatios = fetched.Raw
	profile.UpstreamGroupRatio = groupRatio

	if err := model.RecordUpstreamGroupRatioSyncSuccess(
		profile.ChannelId,
		profile.Id,
		profile.UpstreamGroup,
		baseURL,
		groupRatio,
		fetched.Source,
	); err != nil {
		return err
	}

	if err := DisableChannelWhenUpstreamGroupMissing(profile, "upstream ratio sync"); err != nil {
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
				if recordErr := model.RecordUpstreamGroupRatioSyncFailure(
					profiles[i].ChannelId,
					profiles[i].Id,
					profiles[i].UpstreamGroup,
					profiles[i].UpstreamLoginUrl,
					syncErr,
				); recordErr != nil {
					common.SysLog(fmt.Sprintf("record upstream group ratio sync failure task failed for channel #%d profile #%d: %s",
						profiles[i].ChannelId, profiles[i].Id, recordErr.Error()))
				}
				failed++
			} else {
				synced++
			}
		}
		if len(profiles) < batchSize {
			break
		}
	}
	if _, cleanupErr := model.CleanupExpiredUpstreamGroupRatioSyncFailureTasks(); cleanupErr != nil {
		common.SysLog("cleanup upstream group ratio sync failure tasks failed: " + cleanupErr.Error())
	}
	return synced, failed, nil
}
