package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	UpstreamGroupRatios         string  `json:"upstream_group_ratios" gorm:"type:text"`
	InsufficientBalanceKeywords string  `json:"insufficient_balance_keywords" gorm:"type:varchar(1024);default:''"`
	NotifyEnabled               bool    `json:"notify_enabled" gorm:"default:true"`
	LastInsufficientAt          int64   `json:"last_insufficient_at" gorm:"bigint"`
	LastInsufficientReason      string  `json:"last_insufficient_reason" gorm:"type:varchar(512);default:''"`
	LastNotifiedAt              int64   `json:"last_notified_at" gorm:"bigint"`
	NotifySuppressUntil         int64   `json:"notify_suppress_until" gorm:"bigint"`
	CreatedAt                   int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                   int64   `json:"updated_at" gorm:"bigint"`
}

type ChannelUpstreamProfileSummary struct {
	Id                          int64   `json:"id"`
	ChannelId                   int     `json:"channel_id"`
	KeyFingerprint              string  `json:"key_fingerprint"`
	KeyMasked                   string  `json:"key_masked"`
	KeyLabel                    string  `json:"key_label"`
	UpstreamAccount             string  `json:"upstream_account"`
	UpstreamLoginUrl            string  `json:"upstream_login_url"`
	UpstreamGroup               string  `json:"upstream_group"`
	UpstreamGroupRatio          float64 `json:"upstream_group_ratio"`
	UpstreamGroupRatios         string  `json:"upstream_group_ratios"`
	InsufficientBalanceKeywords string  `json:"insufficient_balance_keywords"`
	NotifyEnabled               bool    `json:"notify_enabled"`
	PasswordConfigured          bool    `json:"password_configured"`
	LastInsufficientAt          int64   `json:"last_insufficient_at"`
	LastInsufficientReason      string  `json:"last_insufficient_reason"`
	LastNotifiedAt              int64   `json:"last_notified_at"`
	NotifySuppressUntil         int64   `json:"notify_suppress_until"`
	CreatedAt                   int64   `json:"created_at"`
	UpdatedAt                   int64   `json:"updated_at"`
}

type ChannelUpstreamProfileInput struct {
	KeyLabel                    string  `json:"key_label"`
	UpstreamAccount             string  `json:"upstream_account"`
	UpstreamPassword            string  `json:"upstream_password"`
	UpstreamLoginUrl            string  `json:"upstream_login_url"`
	UpstreamGroup               string  `json:"upstream_group"`
	UpstreamGroupRatio          float64 `json:"upstream_group_ratio"`
	UpstreamGroupRatios         string  `json:"upstream_group_ratios"`
	InsufficientBalanceKeywords string  `json:"insufficient_balance_keywords"`
	NotifyEnabled               *bool   `json:"notify_enabled"`
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
	return ChannelUpstreamProfileSummary{
		Id:                          profile.Id,
		ChannelId:                   profile.ChannelId,
		KeyFingerprint:              profile.KeyFingerprint,
		KeyMasked:                   profile.KeyMasked,
		KeyLabel:                    profile.KeyLabel,
		UpstreamAccount:             profile.UpstreamAccount,
		UpstreamLoginUrl:            profile.UpstreamLoginUrl,
		UpstreamGroup:               profile.UpstreamGroup,
		UpstreamGroupRatio:          profile.UpstreamGroupRatio,
		UpstreamGroupRatios:         profile.UpstreamGroupRatios,
		InsufficientBalanceKeywords: profile.InsufficientBalanceKeywords,
		NotifyEnabled:               profile.NotifyEnabled,
		PasswordConfigured:          profile.UpstreamPasswordEnc != "",
		LastInsufficientAt:          profile.LastInsufficientAt,
		LastInsufficientReason:      profile.LastInsufficientReason,
		LastNotifiedAt:              profile.LastNotifiedAt,
		NotifySuppressUntil:         profile.NotifySuppressUntil,
		CreatedAt:                   profile.CreatedAt,
		UpdatedAt:                   profile.UpdatedAt,
	}
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

func UpsertChannelUpstreamProfile(channel *Channel, input ChannelUpstreamProfileInput, passwordEnc string) (*ChannelUpstreamProfile, error) {
	key := primaryChannelKey(channel.Key)
	profile := &ChannelUpstreamProfile{}
	fingerprint := KeyFingerprint(key)
	now := common.GetTimestamp()
	err := DB.Where("channel_id = ? AND key_fingerprint = ?", channel.Id, fingerprint).First(profile).Error
	if err != nil {
		profile = &ChannelUpstreamProfile{
			ChannelId:      channel.Id,
			KeyFingerprint: fingerprint,
			KeyMasked:      MaskKey(key),
			NotifyEnabled:  true,
			CreatedAt:      now,
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
	profile.UpstreamGroupRatios = strings.TrimSpace(input.UpstreamGroupRatios)
	profile.InsufficientBalanceKeywords = strings.TrimSpace(input.InsufficientBalanceKeywords)
	if input.NotifyEnabled != nil {
		profile.NotifyEnabled = *input.NotifyEnabled
	}
	profile.UpdatedAt = now
	if profile.Id == 0 {
		return profile, DB.Create(profile).Error
	}
	return profile, DB.Save(profile).Error
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
	profile.UpstreamGroupRatios = strings.TrimSpace(input.UpstreamGroupRatios)
	profile.InsufficientBalanceKeywords = strings.TrimSpace(input.InsufficientBalanceKeywords)
	if input.NotifyEnabled != nil {
		profile.NotifyEnabled = *input.NotifyEnabled
	}
	profile.UpdatedAt = common.GetTimestamp()
	return profile, DB.Save(profile).Error
}

func DeleteChannelUpstreamProfile(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelUpstreamProfile{}).Error
}
