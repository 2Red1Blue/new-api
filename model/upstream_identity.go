package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// UpstreamAuthType 常量
const (
	AuthTypeLegacyPassword         = ""                // 默认，沿用 account+password 登录
	AuthTypeLegacyPasswordExplicit = "legacy_password" // 显式指定
	AuthTypeSub2APIAccessToken     = "sub2api_access_token"
	AuthTypeSub2APIRefreshToken    = "sub2api_refresh_token"
)

// UpstreamIdentity 上游身份，按 (base_url, account) 唯一。
// 存储登录凭据和会话 token，供同一身份下的多个 ChannelUpstreamProfile 共享。
type UpstreamIdentity struct {
	Id                   int64  `json:"id"`
	IdentityFingerprint  string `json:"-" gorm:"type:varchar(32);uniqueIndex:idx_identity_fp_account,priority:1"`
	Account              string `json:"account" gorm:"type:varchar(255);uniqueIndex:idx_identity_fp_account,priority:2"`
	BaseURL              string `json:"base_url" gorm:"type:varchar(512);default:''"`

	// --- 密码登录 ---
	PasswordEnc string `json:"-" gorm:"type:text"`

	// --- 会话凭据 (access_token / refresh_token) ---
	AuthType             string `json:"auth_type" gorm:"type:varchar(32);default:''"`
	AccessTokenEnc       string `json:"-" gorm:"type:text"`
	RefreshTokenEnc      string `json:"-" gorm:"type:text"`
	AccessTokenExpiresAt int64  `json:"access_token_expires_at" gorm:"bigint;default:0"`
	AuthRefreshedAt      int64  `json:"auth_refreshed_at" gorm:"bigint;default:0"`
	AuthRefreshError     string `json:"auth_refresh_error" gorm:"type:varchar(512);default:''"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// UpstreamIdentityFingerprint 是 (base_url, account) 的确定性摘要
func UpstreamIdentityFingerprint(baseURL, account string) string {
	raw := strings.TrimSpace(baseURL) + "|" + strings.TrimSpace(account)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:8])
}

// EnsureUpstreamIdentity 按 (base_url, account) 查找或创建，返回 identity 和是否为新建
func EnsureUpstreamIdentity(baseURL, account string) (*UpstreamIdentity, bool, error) {
	baseURL = strings.TrimSpace(baseURL)
	account = strings.TrimSpace(account)
	if baseURL == "" || account == "" {
		return nil, false, fmt.Errorf("base_url and account are required")
	}

	fp := UpstreamIdentityFingerprint(baseURL, account)
	now := common.GetTimestamp()

	identity := &UpstreamIdentity{}
	err := DB.Where("identity_fingerprint = ? AND account = ?", fp, account).First(identity).Error
	if err == nil {
		return identity, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	identity = &UpstreamIdentity{
		IdentityFingerprint: fp,
		Account:             account,
		BaseURL:             baseURL,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if createErr := DB.Create(identity).Error; createErr != nil {
		return nil, false, createErr
	}
	return identity, true, nil
}

// GetUpstreamIdentityByProfile 通过 profile 的 FK 查询 identity，缺失时 fallback 到 legacy 字段创建
func GetUpstreamIdentityByProfile(profile *ChannelUpstreamProfile) (*UpstreamIdentity, error) {
	if profile == nil {
		return nil, nil
	}
	// 优先走 FK
	if profile.UpstreamIdentityId != nil && *profile.UpstreamIdentityId > 0 {
		identity := &UpstreamIdentity{}
		if err := DB.First(identity, *profile.UpstreamIdentityId).Error; err != nil {
			return nil, err
		}
		return identity, nil
	}
	// legacy fallback: 用 profile 上的 account + channel 的 base_url 尝试匹配
	account := strings.TrimSpace(profile.UpstreamAccount)
	if account == "" {
		return nil, nil
	}
	channel, err := GetChannelById(profile.ChannelId, true)
	if err != nil {
		return nil, err
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		return nil, nil
	}
	identity, _, err := EnsureUpstreamIdentity(baseURL, account)
	return identity, err
}

// ResolveIdentity 懒加载 profile 关联的 identity。
// 优先使用已缓存的 identity，否则走 GetUpstreamIdentityByProfile。
// 调用方应在批量场景使用 PreloadUpstreamIdentities 预加载。
func (profile *ChannelUpstreamProfile) ResolveIdentity() (*UpstreamIdentity, error) {
	if profile == nil {
		return nil, nil
	}
	if profile.UpstreamIdentity != nil {
		return profile.UpstreamIdentity, nil
	}
	identity, err := GetUpstreamIdentityByProfile(profile)
	if err != nil {
		return nil, err
	}
	profile.UpstreamIdentity = identity
	return identity, nil
}

// PreloadUpstreamIdentities 批量预加载 profiles 的 identity，避免 N+1 查询。
func PreloadUpstreamIdentities(profiles []*ChannelUpstreamProfile) error {
	if len(profiles) == 0 {
		return nil
	}

	// 收集有 FK 的 profile
	idSet := make(map[int64]struct{})
	for _, p := range profiles {
		if p == nil || p.UpstreamIdentityId == nil || *p.UpstreamIdentityId <= 0 {
			continue
		}
		idSet[*p.UpstreamIdentityId] = struct{}{}
	}

	if len(idSet) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	var identities []UpstreamIdentity
	if err := DB.Where("id IN ?", ids).Find(&identities).Error; err != nil {
		return err
	}

	idMap := make(map[int64]*UpstreamIdentity, len(identities))
	for i := range identities {
		idMap[identities[i].Id] = &identities[i]
	}

	for _, p := range profiles {
		if p == nil || p.UpstreamIdentityId == nil {
			continue
		}
		if identity, ok := idMap[*p.UpstreamIdentityId]; ok {
			p.UpstreamIdentity = identity
		}
	}

	return nil
}

// HasSessionAuth 返回 identity 是否配置了会话凭据
func (i *UpstreamIdentity) HasSessionAuth() bool {
	if i == nil {
		return false
	}
	return i.AuthType != "" && i.AuthType != AuthTypeLegacyPasswordExplicit
}

// HasPasswordAuth 返回 identity 是否配置了密码登录
func (i *UpstreamIdentity) HasPasswordAuth() bool {
	if i == nil {
		return false
	}
	return i.PasswordEnc != ""
}
