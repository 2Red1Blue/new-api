package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// FetchUpstreamGroupRatiosFromProfile 根据 profile 关联的 UpstreamIdentity auth_type 选择认证方式获取上游分组倍率。
// 优先读 identity，缺失时 fallback 到 legacy profile 字段。
func FetchUpstreamGroupRatiosFromProfile(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	profile *model.ChannelUpstreamProfile,
) (*UpstreamGroupRatiosResult, error) {
	if profile == nil {
		return nil, errors.New("upstream profile is nil")
	}

	// 先尝试通过 identity 认证
	identity, identityErr := profile.ResolveIdentity()
	if identityErr == nil && identity != nil {
		authType := strings.TrimSpace(identity.AuthType)
		switch authType {
		case model.AuthTypeSub2APIAccessToken:
			return fetchSub2APIGroupRatiosWithToken(ctx, client, baseURL, identity.AccessTokenEnc)
		case model.AuthTypeSub2APIRefreshToken:
			return fetchSub2APIGroupRatiosWithIdentityRefresh(ctx, client, baseURL, identity)
		default:
			// identity 上的 legacy_password 模式：用 identity 的 account+password
			if identity.HasPasswordAuth() {
				password, decErr := DecryptUpstreamPassword(identity.PasswordEnc)
				if decErr == nil {
					credential := &UpstreamPricingCredential{Account: identity.Account, Password: password}
					return FetchUpstreamGroupRatios(ctx, client, baseURL, credential)
				}
			}
		}
	}

	// fallback: 读 profile legacy 字段
	authType := strings.TrimSpace(profile.UpstreamAuthType)
	switch authType {
	case model.AuthTypeSub2APIAccessToken:
		return fetchSub2APIGroupRatiosWithToken(ctx, client, baseURL, profile.UpstreamAccessTokenEnc)
	case model.AuthTypeSub2APIRefreshToken:
		return fetchSub2APIGroupRatiosWithProfileRefresh(ctx, client, baseURL, profile)
	default:
		credential := UpstreamCredentialFromProfileRecord(profile)
		return FetchUpstreamGroupRatios(ctx, client, baseURL, credential)
	}
}

// fetchSub2APIGroupRatiosWithToken 用 access token 直接拉取分组倍率
func fetchSub2APIGroupRatiosWithToken(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessTokenEnc string,
) (*UpstreamGroupRatiosResult, error) {
	accessToken, err := DecryptUpstreamPassword(accessTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token failed: %w", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("access token is empty")
	}
	return fetchSub2APIGroupRatios(ctx, client, baseURL, accessToken)
}

// fetchSub2APIGroupRatiosWithIdentityRefresh 通过 identity 的 refresh_token 模式获取分组倍率。
// 使用 EnsureUpstreamAccessToken 确保 access token 有效。
func fetchSub2APIGroupRatiosWithIdentityRefresh(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	identity *model.UpstreamIdentity,
) (*UpstreamGroupRatiosResult, error) {
	accessToken, err := EnsureUpstreamAccessToken(ctx, client, identity)
	if err != nil {
		return nil, fmt.Errorf("ensure access token failed: %w", err)
	}
	return fetchSub2APIGroupRatios(ctx, client, baseURL, accessToken)
}

// fetchSub2APIGroupRatiosWithProfileRefresh legacy fallback：通过 profile 的 refresh_token 获取分组倍率。
// 内部也尝试走 identity 路径（如果 profile 有 UpstreamIdentityId）。
func fetchSub2APIGroupRatiosWithProfileRefresh(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	profile *model.ChannelUpstreamProfile,
) (*UpstreamGroupRatiosResult, error) {
	// 尝试通过 identity 走标准路径
	if profile.UpstreamIdentityId != nil && *profile.UpstreamIdentityId > 0 {
		identity := &model.UpstreamIdentity{}
		if err := model.DB.First(identity, *profile.UpstreamIdentityId).Error; err == nil {
			return fetchSub2APIGroupRatiosWithIdentityRefresh(ctx, client, baseURL, identity)
		}
	}

	// pure legacy: 直接用 profile 上的字段
	accessToken, err := DecryptUpstreamPassword(profile.UpstreamAccessTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token failed: %w", err)
	}
	if strings.TrimSpace(accessToken) != "" {
		return fetchSub2APIGroupRatios(ctx, client, baseURL, accessToken)
	}

	return nil, errors.New("no valid access token found in legacy profile")
}
