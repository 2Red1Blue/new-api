package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const DefaultInsufficientBalanceKeywords = "insufficient_quota\ninsufficient balance\nquota exceeded\nbilling hard limit\n余额不足\n额度不足\n剩余额度\n欠费\n账户余额\ncredit balance\naccount balance"

type ChannelUpstreamProfile struct {
	Id                          int64   `json:"id"`
	ChannelId                   int     `json:"channel_id" gorm:"index:idx_channel_upstream_profile_channel_key,unique"`
	KeyFingerprint              string  `json:"key_fingerprint" gorm:"type:varchar(32);index:idx_channel_upstream_profile_channel_key,unique"`
	KeyMasked                   string  `json:"key_masked" gorm:"type:varchar(32);default:''"`
	KeyLabel                    string  `json:"key_label" gorm:"type:varchar(64);default:''"`
	UpstreamAccount             string  `json:"upstream_account" gorm:"type:varchar(255);default:''"`
	UpstreamPasswordEnc         string  `json:"-" gorm:"type:text"`
	UpstreamLoginUrl            string  `json:"upstream_login_url" gorm:"type:varchar(512);default:''"`
	UpstreamGroup               string  `json:"upstream_group" gorm:"type:varchar(128);default:''"`
	UpstreamGroupRatio          float64 `json:"upstream_group_ratio" gorm:"type:decimal(10,4);default:0"`
	UpstreamTopupRatio          float64 `json:"upstream_topup_ratio" gorm:"type:decimal(10,4);default:1"`
	UpstreamGroupRatios         string  `json:"upstream_group_ratios" gorm:"type:text"`
	AutoPriorityEnabled         bool    `json:"auto_priority_enabled" gorm:"default:true"`
	AutoPriorityBase            int64   `json:"auto_priority_base" gorm:"bigint;default:1"`
	AutoPriorityMin             int64   `json:"auto_priority_min" gorm:"bigint;default:0"`
	AutoPriorityMax             int64   `json:"auto_priority_max" gorm:"bigint;default:100"`
	AutoPriorityValue           int64   `json:"auto_priority_value" gorm:"bigint;default:0"`
	AutoPriorityUpdatedAt       int64   `json:"auto_priority_updated_at" gorm:"bigint;default:0"`
	AutoPriorityReason          string  `json:"auto_priority_reason" gorm:"type:varchar(255);default:''"`
	InsufficientBalanceKeywords string  `json:"insufficient_balance_keywords" gorm:"type:varchar(1024);default:''"`
	NotifyEnabled               bool    `json:"notify_enabled" gorm:"default:true"`
	LastInsufficientAt          int64   `json:"last_insufficient_at" gorm:"bigint"`
	LastInsufficientReason      string  `json:"last_insufficient_reason" gorm:"type:varchar(512);default:''"`
	LastNotifiedAt              int64   `json:"last_notified_at" gorm:"bigint"`
	NotifySuppressUntil         int64   `json:"notify_suppress_until" gorm:"bigint"`
	CreatedAt                   int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                   int64   `json:"updated_at" gorm:"bigint"`
	// 上游会话凭据（refresh_token → access_token），用于避开 Turnstile 等浏览器验证
	UpstreamAuthType             string `json:"upstream_auth_type" gorm:"type:varchar(32);default:''"`
	UpstreamAccessTokenEnc       string `json:"-" gorm:"type:text"`
	UpstreamRefreshTokenEnc      string `json:"-" gorm:"type:text"`
	UpstreamAccessTokenExpiresAt int64  `json:"upstream_access_token_expires_at" gorm:"bigint;default:0"`
	UpstreamAuthRefreshedAt      int64  `json:"upstream_auth_refreshed_at" gorm:"bigint;default:0"`
	UpstreamAuthRefreshError     string `json:"upstream_auth_refresh_error" gorm:"type:varchar(512);default:''"`

	// 关联上游身份，跨 channel 共享登录凭据和会话 token
	UpstreamIdentityId *int64            `json:"upstream_identity_id" gorm:"index"`
	UpstreamIdentity   *UpstreamIdentity `json:"-" gorm:"-"`
}

type ChannelUpstreamProfileSummary struct {
	Id                           int64   `json:"id"`
	ChannelId                    int     `json:"channel_id"`
	KeyFingerprint               string  `json:"key_fingerprint"`
	KeyMasked                    string  `json:"key_masked"`
	KeyLabel                     string  `json:"key_label"`
	UpstreamAccount              string  `json:"upstream_account"`
	UpstreamLoginUrl             string  `json:"upstream_login_url"`
	UpstreamGroup                string  `json:"upstream_group"`
	UpstreamGroupRatio           float64 `json:"upstream_group_ratio"`
	UpstreamTopupRatio           float64 `json:"upstream_topup_ratio"`
	UpstreamEffectiveRatio       float64 `json:"upstream_effective_ratio"`
	UpstreamGroupRatios          string  `json:"upstream_group_ratios"`
	AutoPriorityEnabled          bool    `json:"auto_priority_enabled"`
	AutoPriorityBase             int64   `json:"auto_priority_base"`
	AutoPriorityMin              int64   `json:"auto_priority_min"`
	AutoPriorityMax              int64   `json:"auto_priority_max"`
	AutoPriorityValue            int64   `json:"auto_priority_value"`
	AutoPriorityUpdatedAt        int64   `json:"auto_priority_updated_at"`
	AutoPriorityReason           string  `json:"auto_priority_reason"`
	InsufficientBalanceKeywords  string  `json:"insufficient_balance_keywords"`
	NotifyEnabled                bool    `json:"notify_enabled"`
	PasswordConfigured           bool    `json:"password_configured"`
	LastInsufficientAt           int64   `json:"last_insufficient_at"`
	LastInsufficientReason       string  `json:"last_insufficient_reason"`
	LastNotifiedAt               int64   `json:"last_notified_at"`
	NotifySuppressUntil          int64   `json:"notify_suppress_until"`
	CreatedAt                    int64   `json:"created_at"`
	UpdatedAt                    int64   `json:"updated_at"`
	UpstreamAuthType             string  `json:"upstream_auth_type"`
	UpstreamAccessTokenExpiresAt int64   `json:"upstream_access_token_expires_at"`
	UpstreamAuthRefreshedAt      int64   `json:"upstream_auth_refreshed_at"`
	UpstreamAuthRefreshError     string  `json:"upstream_auth_refresh_error"`
	SessionConfigured            bool    `json:"session_configured"`
}

type ChannelUpstreamProfileInput struct {
	KeyLabel                     string  `json:"key_label"`
	UpstreamAccount              string  `json:"upstream_account"`
	UpstreamPassword             string  `json:"upstream_password"`
	UpstreamLoginUrl             string  `json:"upstream_login_url"`
	UpstreamGroup                string  `json:"upstream_group"`
	UpstreamGroupRatio           float64 `json:"upstream_group_ratio"`
	UpstreamTopupRatio           float64 `json:"upstream_topup_ratio"`
	UpstreamGroupRatios          string  `json:"upstream_group_ratios"`
	AutoPriorityEnabled          *bool   `json:"auto_priority_enabled"`
	AutoPriorityBase             int64   `json:"auto_priority_base"`
	AutoPriorityMin              int64   `json:"auto_priority_min"`
	AutoPriorityMax              int64   `json:"auto_priority_max"`
	InsufficientBalanceKeywords  string  `json:"insufficient_balance_keywords"`
	NotifyEnabled                *bool   `json:"notify_enabled"`
	UpstreamAuthType             string  `json:"upstream_auth_type"`
	UpstreamAccessToken          string  `json:"upstream_access_token"`
	UpstreamRefreshToken         string  `json:"upstream_refresh_token"`
	UpstreamAccessTokenExpiresIn int64   `json:"upstream_access_token_expires_in"`
}

type ChannelUpstreamProfilePatch struct {
	ChannelUpstreamProfileInput
	ClearPassword bool `json:"clear_password"`
}

func KeyFingerprint(key string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(hash[:8])
}

func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func primaryChannelKey(raw string) string {
	parts := strings.SplitN(strings.TrimSpace(raw), "\n", 2)
	return strings.TrimSpace(parts[0])
}

func (profile *ChannelUpstreamProfile) Summary() ChannelUpstreamProfileSummary {
	topupRatio := normalizeUpstreamTopupRatio(profile.UpstreamTopupRatio)
	return ChannelUpstreamProfileSummary{
		Id:                           profile.Id,
		ChannelId:                    profile.ChannelId,
		KeyFingerprint:               profile.KeyFingerprint,
		KeyMasked:                    profile.KeyMasked,
		KeyLabel:                     profile.KeyLabel,
		UpstreamAccount:              profile.UpstreamAccount,
		UpstreamLoginUrl:             profile.UpstreamLoginUrl,
		UpstreamGroup:                profile.UpstreamGroup,
		UpstreamGroupRatio:           profile.UpstreamGroupRatio,
		UpstreamTopupRatio:           topupRatio,
		UpstreamEffectiveRatio:       CalculateUpstreamEffectiveRatio(profile.UpstreamGroupRatio, topupRatio),
		UpstreamGroupRatios:          profile.UpstreamGroupRatios,
		AutoPriorityEnabled:          profile.AutoPriorityEnabled,
		AutoPriorityBase:             normalizeAutoPriorityBase(profile.AutoPriorityBase),
		AutoPriorityMin:              normalizeAutoPriorityMin(profile.AutoPriorityMin),
		AutoPriorityMax:              normalizeAutoPriorityMax(profile.AutoPriorityMin, profile.AutoPriorityMax),
		AutoPriorityValue:            profile.AutoPriorityValue,
		AutoPriorityUpdatedAt:        profile.AutoPriorityUpdatedAt,
		AutoPriorityReason:           profile.AutoPriorityReason,
		InsufficientBalanceKeywords:  profile.InsufficientBalanceKeywords,
		NotifyEnabled:                profile.NotifyEnabled,
		PasswordConfigured:           profile.UpstreamPasswordEnc != "",
		LastInsufficientAt:           profile.LastInsufficientAt,
		LastInsufficientReason:       profile.LastInsufficientReason,
		LastNotifiedAt:               profile.LastNotifiedAt,
		NotifySuppressUntil:          profile.NotifySuppressUntil,
		CreatedAt:                    profile.CreatedAt,
		UpdatedAt:                    profile.UpdatedAt,
		UpstreamAuthType:             profile.UpstreamAuthType,
		UpstreamAccessTokenExpiresAt: profile.UpstreamAccessTokenExpiresAt,
		UpstreamAuthRefreshedAt:      profile.UpstreamAuthRefreshedAt,
		UpstreamAuthRefreshError:     profile.UpstreamAuthRefreshError,
		SessionConfigured:            profile.UpstreamAccessTokenEnc != "" || profile.UpstreamRefreshTokenEnc != "",
	}
}

func normalizeUpstreamTopupRatio(ratio float64) float64 {
	if ratio <= 0 {
		return 1
	}
	return ratio
}

func CalculateUpstreamEffectiveRatio(groupRatio float64, topupRatio float64) float64 {
	topupRatio = normalizeUpstreamTopupRatio(topupRatio)
	if groupRatio <= 0 {
		return 0
	}
	return groupRatio / topupRatio
}

func normalizeAutoPriorityBase(value int64) int64 {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeAutoPriorityMin(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeAutoPriorityMax(minValue int64, maxValue int64) int64 {
	minValue = normalizeAutoPriorityMin(minValue)
	if maxValue <= 0 {
		return 100
	}
	if maxValue < minValue {
		return minValue
	}
	return maxValue
}

// upstreamGroupSnapshotRaw 与 service 包中的 UpstreamGroupSnapshotEntry 保持同步
type upstreamGroupSnapshotRaw struct {
	RateMultiplier float64 `json:"rate_multiplier"`
}

func ParseUpstreamGroupRatios(raw string) map[string]float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// 格式 1: 简单倍率 map，如 {"group": 1.5}
	ratios := make(map[string]float64)
	if err := common.UnmarshalJsonStr(raw, &ratios); err == nil {
		return ratios
	}
	// 格式 2: sub2api 快照格式，如 {"group": {"rate_multiplier": 0.95, ...}}
	snapshot := make(map[string]upstreamGroupSnapshotRaw)
	if err := common.UnmarshalJsonStr(raw, &snapshot); err == nil && len(snapshot) > 0 {
		ratios = make(map[string]float64, len(snapshot))
		for name, entry := range snapshot {
			ratios[name] = entry.RateMultiplier
		}
		return ratios
	}
	return nil
}

func HasUpstreamGroupRatio(raw string, group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return true
	}
	ratios := ParseUpstreamGroupRatios(raw)
	if len(ratios) == 0 {
		return false
	}
	_, ok := ratios[group]
	return ok
}

func IsUpstreamGroupMissing(raw string, group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return false
	}
	ratios := ParseUpstreamGroupRatios(raw)
	if len(ratios) == 0 {
		return true
	}
	_, ok := ratios[group]
	return !ok
}

func clampInt64(value int64, minValue int64, maxValue int64) int64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func CalculateAutoPriorityValue(groupRatio float64, topupRatio float64, base int64, minValue int64, maxValue int64) (int64, string) {
	effectiveRatio := CalculateUpstreamEffectiveRatio(groupRatio, topupRatio)
	base = normalizeAutoPriorityBase(base)
	minValue = normalizeAutoPriorityMin(minValue)
	maxValue = normalizeAutoPriorityMax(minValue, maxValue)
	if effectiveRatio <= 0 {
		return 0, fmt.Sprintf("skip: effective_ratio=0, base=%d, min=%d, max=%d", base, minValue, maxValue)
	}
	raw := int64(math.Round(float64(base) / effectiveRatio))
	value := clampInt64(raw, minValue, maxValue)
	return value, fmt.Sprintf("cost: er=%.4f, base=%d, raw=%d, priority=%d", effectiveRatio, base, raw, value)
}

func applyAutoPriority(profile *ChannelUpstreamProfile, now int64) {
	profile.AutoPriorityBase = normalizeAutoPriorityBase(profile.AutoPriorityBase)
	profile.AutoPriorityMin = normalizeAutoPriorityMin(profile.AutoPriorityMin)
	profile.AutoPriorityMax = normalizeAutoPriorityMax(profile.AutoPriorityMin, profile.AutoPriorityMax)
	if !profile.AutoPriorityEnabled {
		profile.AutoPriorityValue = 0
		profile.AutoPriorityUpdatedAt = now
		profile.AutoPriorityReason = "disabled"
		return
	}
	value, reason := CalculateAutoPriorityValue(
		profile.UpstreamGroupRatio,
		profile.UpstreamTopupRatio,
		profile.AutoPriorityBase,
		profile.AutoPriorityMin,
		profile.AutoPriorityMax,
	)
	profile.AutoPriorityValue = value
	profile.AutoPriorityUpdatedAt = now
	profile.AutoPriorityReason = reason
}

func ApplyChannelAutoPriority(channelId int, enabled bool, value int64) error {
	if !enabled || value < 0 {
		return nil
	}
	priority := value
	if err := DB.Model(&Channel{}).Where("id = ?", channelId).Update("priority", priority).Error; err != nil {
		return err
	}
	if err := DB.Model(&Ability{}).Where("channel_id = ?", channelId).Update("priority", priority).Error; err != nil {
		return err
	}
	if common.MemoryCacheEnabled {
		channelCache, err := CacheGetChannel(channelId)
		if err == nil && channelCache != nil {
			channelCache.Priority = &priority
			CacheUpdateChannel(channelCache)
		}
	}
	return nil
}

func shouldApplyAutoPriority(profile *ChannelUpstreamProfile) bool {
	return profile.AutoPriorityEnabled && !strings.HasPrefix(profile.AutoPriorityReason, "skip:")
}

func RecalculateChannelUpstreamProfileAutoPriority(profile *ChannelUpstreamProfile, now int64) (bool, error) {
	if profile == nil {
		return false, nil
	}
	applyAutoPriority(profile, now)
	if err := DB.Model(&ChannelUpstreamProfile{}).
		Where("id = ?", profile.Id).
		Updates(map[string]any{
			"auto_priority_base":       profile.AutoPriorityBase,
			"auto_priority_min":        profile.AutoPriorityMin,
			"auto_priority_max":        profile.AutoPriorityMax,
			"auto_priority_value":      profile.AutoPriorityValue,
			"auto_priority_updated_at": profile.AutoPriorityUpdatedAt,
			"auto_priority_reason":     profile.AutoPriorityReason,
			"updated_at":               now,
		}).Error; err != nil {
		return false, err
	}
	if err := ApplyChannelAutoPriority(profile.ChannelId, shouldApplyAutoPriority(profile), profile.AutoPriorityValue); err != nil {
		return false, err
	}
	return shouldApplyAutoPriority(profile), nil
}

func RecalculateAllChannelAutoPriorities(batchSize int) (int, int, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	now := common.GetTimestamp()
	var scanned int
	var applied int
	var lastID int64
	for {
		var profiles []ChannelUpstreamProfile
		err := DB.
			Where("id > ? AND auto_priority_enabled = ?", lastID, true).
			Order("id ASC").
			Limit(batchSize).
			Find(&profiles).Error
		if err != nil {
			return scanned, applied, err
		}
		if len(profiles) == 0 {
			break
		}
		for i := range profiles {
			lastID = profiles[i].Id
			scanned++
			ok, err := RecalculateChannelUpstreamProfileAutoPriority(&profiles[i], now)
			if err != nil {
				return scanned, applied, err
			}
			if ok {
				applied++
			}
		}
		if len(profiles) < batchSize {
			break
		}
	}
	return scanned, applied, nil
}

func GetChannelUpstreamProfileByChannelId(channelId int) (*ChannelUpstreamProfile, error) {
	profile := &ChannelUpstreamProfile{}
	err := DB.Where("channel_id = ?", channelId).Order("id ASC").First(profile).Error
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func GetChannelUpstreamProfileByKey(channelId int, key string) (*ChannelUpstreamProfile, error) {
	return GetChannelUpstreamProfileByFingerprint(channelId, KeyFingerprint(key))
}

func GetChannelUpstreamProfileByFingerprint(channelId int, fingerprint string) (*ChannelUpstreamProfile, error) {
	profile := &ChannelUpstreamProfile{}
	err := DB.Where("channel_id = ? AND key_fingerprint = ?", channelId, fingerprint).First(profile).Error
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func GetChannelUpstreamProfilesByChannelIds(channelIds []int) (map[int]ChannelUpstreamProfileSummary, error) {
	result := make(map[int]ChannelUpstreamProfileSummary)
	if len(channelIds) == 0 {
		return result, nil
	}
	var profiles []ChannelUpstreamProfile
	if err := DB.Where("channel_id IN ?", channelIds).Find(&profiles).Error; err != nil {
		return nil, err
	}
	for i := range profiles {
		if existing, exists := result[profiles[i].ChannelId]; exists && existing.UpdatedAt >= profiles[i].UpdatedAt {
			continue
		}
		result[profiles[i].ChannelId] = profiles[i].Summary()
	}
	return result, nil
}

func CloneChannelUpstreamProfiles(tx *gorm.DB, sourceChannelId int, targetChannelId int, targetBaseURL string, now int64) error {
	if tx == nil {
		tx = DB
	}

	var profiles []ChannelUpstreamProfile
	if err := tx.Where("channel_id = ?", sourceChannelId).Order("id ASC").Find(&profiles).Error; err != nil {
		return err
	}
	if len(profiles) == 0 {
		return nil
	}

	clones := make([]ChannelUpstreamProfile, 0, len(profiles))
	for _, profile := range profiles {
		profile.Id = 0
		profile.ChannelId = targetChannelId
		profile.LastInsufficientAt = 0
		profile.LastInsufficientReason = ""
		profile.LastNotifiedAt = 0
		profile.NotifySuppressUntil = 0
		profile.CreatedAt = now
		profile.UpdatedAt = now
		clones = append(clones, profile)
	}

	if err := tx.Create(&clones).Error; err != nil {
		return err
	}

	// 为目标渠道重新绑定 UpstreamIdentity（base_url 可能不同）
	if strings.TrimSpace(targetBaseURL) != "" {
		for i := range clones {
			clone := &clones[i]
			if strings.TrimSpace(clone.UpstreamAccount) == "" {
				continue
			}
			identity, _, identityErr := EnsureUpstreamIdentity(targetBaseURL, clone.UpstreamAccount)
			if identityErr == nil {
				clone.UpstreamIdentityId = &identity.Id
				_ = tx.Model(clone).Update("upstream_identity_id", identity.Id).Error
			} else {
				common.SysLog(fmt.Sprintf("CloneChannelUpstreamProfiles: skip identity rebind for target channel #%d: %s", targetChannelId, identityErr.Error()))
			}
		}
	}

	return nil
}

func UpsertChannelUpstreamProfile(channel *Channel, input ChannelUpstreamProfileInput, passwordEnc string) (*ChannelUpstreamProfile, error) {
	key := primaryChannelKey(channel.Key)
	profile := &ChannelUpstreamProfile{}
	fingerprint := KeyFingerprint(key)
	now := common.GetTimestamp()
	err := DB.Where("channel_id = ? AND key_fingerprint = ?", channel.Id, fingerprint).First(profile).Error
	if err != nil {
		profile = &ChannelUpstreamProfile{
			ChannelId:           channel.Id,
			KeyFingerprint:      fingerprint,
			KeyMasked:           MaskKey(key),
			NotifyEnabled:       true,
			AutoPriorityEnabled: true,
			AutoPriorityBase:    1,
			AutoPriorityMin:     0,
			AutoPriorityMax:     100,
			CreatedAt:           now,
		}
	}
	profile.KeyLabel = strings.TrimSpace(input.KeyLabel)
	profile.KeyMasked = MaskKey(key)
	profile.UpstreamAccount = strings.TrimSpace(input.UpstreamAccount)
	if passwordEnc != "" {
		profile.UpstreamPasswordEnc = passwordEnc
	}
	profile.UpstreamLoginUrl = strings.TrimSpace(input.UpstreamLoginUrl)
	profile.UpstreamGroup = strings.TrimSpace(input.UpstreamGroup)
	profile.UpstreamGroupRatio = input.UpstreamGroupRatio
	profile.UpstreamTopupRatio = normalizeUpstreamTopupRatio(input.UpstreamTopupRatio)
	profile.UpstreamGroupRatios = strings.TrimSpace(input.UpstreamGroupRatios)
	if input.AutoPriorityEnabled != nil {
		profile.AutoPriorityEnabled = *input.AutoPriorityEnabled
	}
	if input.AutoPriorityBase > 0 {
		profile.AutoPriorityBase = input.AutoPriorityBase
	}
	if input.AutoPriorityMin >= 0 {
		profile.AutoPriorityMin = input.AutoPriorityMin
	}
	if input.AutoPriorityMax > 0 {
		profile.AutoPriorityMax = input.AutoPriorityMax
	}
	applyAutoPriority(profile, now)
	profile.InsufficientBalanceKeywords = strings.TrimSpace(input.InsufficientBalanceKeywords)
	if input.NotifyEnabled != nil {
		profile.NotifyEnabled = *input.NotifyEnabled
	}
	profile.UpdatedAt = now
	if profile.Id == 0 {
		err = DB.Create(profile).Error
	} else {
		err = DB.Save(profile).Error
	}
	if err != nil {
		return profile, err
	}

	// 确保 profile 关联到 UpstreamIdentity（account 或 baseURL 变更时重绑定）
	if strings.TrimSpace(profile.UpstreamAccount) != "" && strings.TrimSpace(channel.GetBaseURL()) != "" {
		identity, _, identityErr := EnsureUpstreamIdentity(channel.GetBaseURL(), profile.UpstreamAccount)
		if identityErr == nil && (profile.UpstreamIdentityId == nil || *profile.UpstreamIdentityId != identity.Id) {
			profile.UpstreamIdentityId = &identity.Id
			_ = DB.Model(profile).Update("upstream_identity_id", identity.Id).Error
		}
		// 同时将 profile 上的凭据字段同步到 identity（补充迁移后新增的数据）
		if identityErr == nil && identity != nil {
			identityNeedsUpdate := false
			if identity.PasswordEnc == "" && profile.UpstreamPasswordEnc != "" {
				identity.PasswordEnc = profile.UpstreamPasswordEnc
				identityNeedsUpdate = true
			}
			if identity.AuthType == "" && strings.TrimSpace(profile.UpstreamAuthType) != "" {
				identity.AuthType = profile.UpstreamAuthType
				identityNeedsUpdate = true
			}
			if identityNeedsUpdate {
				identity.UpdatedAt = common.GetTimestamp()
				_ = DB.Save(identity).Error
			}
		}
	}

	return profile, ApplyChannelAutoPriority(channel.Id, shouldApplyAutoPriority(profile), profile.AutoPriorityValue)
}

func UpdateChannelUpstreamProfile(channelId int, input ChannelUpstreamProfilePatch, passwordEnc string) (*ChannelUpstreamProfile, error) {
	profile, err := GetChannelUpstreamProfileByChannelId(channelId)
	if err != nil {
		return nil, err
	}
	channel, err := GetChannelById(channelId, true)
	if err == nil && strings.TrimSpace(channel.Key) != "" {
		key := primaryChannelKey(channel.Key)
		profile.KeyFingerprint = KeyFingerprint(key)
		profile.KeyMasked = MaskKey(key)
	}
	profile.KeyLabel = strings.TrimSpace(input.KeyLabel)
	profile.UpstreamAccount = strings.TrimSpace(input.UpstreamAccount)
	if input.ClearPassword {
		profile.UpstreamPasswordEnc = ""
	} else if passwordEnc != "" {
		profile.UpstreamPasswordEnc = passwordEnc
	}
	profile.UpstreamLoginUrl = strings.TrimSpace(input.UpstreamLoginUrl)
	profile.UpstreamGroup = strings.TrimSpace(input.UpstreamGroup)
	profile.UpstreamGroupRatio = input.UpstreamGroupRatio
	profile.UpstreamTopupRatio = normalizeUpstreamTopupRatio(input.UpstreamTopupRatio)
	profile.UpstreamGroupRatios = strings.TrimSpace(input.UpstreamGroupRatios)
	if input.AutoPriorityEnabled != nil {
		profile.AutoPriorityEnabled = *input.AutoPriorityEnabled
	}
	if input.AutoPriorityBase > 0 {
		profile.AutoPriorityBase = input.AutoPriorityBase
	}
	if input.AutoPriorityMin >= 0 {
		profile.AutoPriorityMin = input.AutoPriorityMin
	}
	if input.AutoPriorityMax > 0 {
		profile.AutoPriorityMax = input.AutoPriorityMax
	}
	profile.InsufficientBalanceKeywords = strings.TrimSpace(input.InsufficientBalanceKeywords)
	if input.NotifyEnabled != nil {
		profile.NotifyEnabled = *input.NotifyEnabled
	}
	profile.UpdatedAt = common.GetTimestamp()
	applyAutoPriority(profile, profile.UpdatedAt)
	if err := DB.Save(profile).Error; err != nil {
		return profile, err
	}

	// 确保 profile 关联到 UpstreamIdentity（account 或 baseURL 变更时重绑定）
	if channel != nil && strings.TrimSpace(profile.UpstreamAccount) != "" && strings.TrimSpace(channel.GetBaseURL()) != "" {
		identity, _, identityErr := EnsureUpstreamIdentity(channel.GetBaseURL(), profile.UpstreamAccount)
		if identityErr == nil && (profile.UpstreamIdentityId == nil || *profile.UpstreamIdentityId != identity.Id) {
			profile.UpstreamIdentityId = &identity.Id
			_ = DB.Model(profile).Update("upstream_identity_id", identity.Id).Error
		}
		if identityErr == nil && identity != nil {
			identityNeedsUpdate := false
			if identity.PasswordEnc == "" && profile.UpstreamPasswordEnc != "" {
				identity.PasswordEnc = profile.UpstreamPasswordEnc
				identityNeedsUpdate = true
			}
			if identity.AuthType == "" && strings.TrimSpace(profile.UpstreamAuthType) != "" {
				identity.AuthType = profile.UpstreamAuthType
				identityNeedsUpdate = true
			}
			if identityNeedsUpdate {
				identity.UpdatedAt = common.GetTimestamp()
				_ = DB.Save(identity).Error
			}
		}
	}

	return profile, ApplyChannelAutoPriority(channelId, shouldApplyAutoPriority(profile), profile.AutoPriorityValue)
}

func DeleteChannelUpstreamProfile(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelUpstreamProfile{}).Error
}
