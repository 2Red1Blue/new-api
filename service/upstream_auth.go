package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const tokenRefreshMarginSeconds = 120

// 按 identity ID 分片的细粒度锁，同一 identity 下的多个 profile 共享同一把锁
var identityRefreshLocks [64]sync.Mutex

func identityRefreshLock(identityID int64) *sync.Mutex {
	return &identityRefreshLocks[identityID%int64(len(identityRefreshLocks))]
}

// EnsureUpstreamAccessToken 确保 identity 有有效的 access token。
// 若 token 未过期直接返回；否则调用上游 /api/v1/auth/refresh 刷新。
// 这是纯粹的认证层能力，不访问任何业务接口。
func EnsureUpstreamAccessToken(ctx context.Context, client *http.Client, identity *model.UpstreamIdentity) (string, error) {
	if identity == nil {
		return "", errors.New("upstream identity is nil")
	}
	if identity.AuthType != model.AuthTypeSub2APIRefreshToken && identity.AuthType != model.AuthTypeSub2APIAccessToken {
		return "", fmt.Errorf("unsupported auth type for session auth: %s", identity.AuthType)
	}

	// access_token 模式：直接解密返回，不做刷新
	if identity.AuthType == model.AuthTypeSub2APIAccessToken {
		accessToken, err := DecryptUpstreamPassword(identity.AccessTokenEnc)
		if err != nil {
			return "", fmt.Errorf("decrypt access token failed: %w", err)
		}
		if strings.TrimSpace(accessToken) == "" {
			return "", errors.New("access token is empty")
		}
		return accessToken, nil
	}

	// refresh_token 模式：需要加锁防止并发刷新
	lock := identityRefreshLock(identity.Id)
	lock.Lock()
	defer lock.Unlock()

	now := common.GetTimestamp()

	// access token 仍然有效，并且能覆盖到下一次定时扫描。
	if identity.AccessTokenExpiresAt > 0 &&
		now+upstreamTokenRefreshLeadSeconds() < identity.AccessTokenExpiresAt {
		accessToken, decErr := DecryptUpstreamPassword(identity.AccessTokenEnc)
		if decErr != nil {
			return "", fmt.Errorf("decrypt access token failed: %w", decErr)
		}
		if strings.TrimSpace(accessToken) != "" {
			return accessToken, nil
		}
	}

	// 需要刷新：先 reload identity 确保拿到最新的 refresh_token
	latestIdentity := &model.UpstreamIdentity{}
	if err := model.DB.First(latestIdentity, identity.Id).Error; err != nil {
		return "", fmt.Errorf("reload identity failed: %w", err)
	}
	identity = latestIdentity

	if identity.AccessTokenExpiresAt > 0 &&
		now+upstreamTokenRefreshLeadSeconds() < identity.AccessTokenExpiresAt {
		accessToken, decErr := DecryptUpstreamPassword(identity.AccessTokenEnc)
		if decErr != nil {
			return "", fmt.Errorf("decrypt access token failed: %w", decErr)
		}
		if strings.TrimSpace(accessToken) != "" {
			return accessToken, nil
		}
	}

	refreshToken, decErr := DecryptUpstreamPassword(identity.RefreshTokenEnc)
	if decErr != nil {
		return "", fmt.Errorf("decrypt refresh token failed: %w", decErr)
	}
	if strings.TrimSpace(refreshToken) == "" {
		return "", errors.New("refresh token is empty")
	}

	// 调用上游刷新接口
	newAccessToken, newRefreshToken, expiresIn, refreshErr := sub2apiRefreshTokens(ctx, client, identity.BaseURL, refreshToken)
	if refreshErr != nil {
		_ = model.DB.Model(&model.UpstreamIdentity{}).
			Where("id = ?", identity.Id).
			Updates(map[string]any{
				"auth_refreshed_at":  now,
				"auth_refresh_error": truncateForDB(refreshErr.Error(), 512),
				"updated_at":         now,
			}).Error
		return "", fmt.Errorf("refresh token failed: %w", refreshErr)
	}

	// 加密新 token
	newAccessEnc, encErr := EncryptUpstreamPassword(newAccessToken)
	if encErr != nil {
		return "", fmt.Errorf("encrypt new access token failed: %w", encErr)
	}
	newRefreshEnc, encErr := EncryptUpstreamPassword(newRefreshToken)
	if encErr != nil {
		return "", fmt.Errorf("encrypt new refresh token failed: %w", encErr)
	}

	// 乐观更新：确保没有其他实例抢先刷新
	oldRefreshEnc := identity.RefreshTokenEnc
	expiresAt := now + expiresIn
	result := model.DB.Model(&model.UpstreamIdentity{}).
		Where("id = ? AND refresh_token_enc = ?", identity.Id, oldRefreshEnc).
		Updates(map[string]any{
			"access_token_enc":        newAccessEnc,
			"refresh_token_enc":       newRefreshEnc,
			"access_token_expires_at": expiresAt,
			"auth_refreshed_at":       now,
			"auth_refresh_error":      "",
			"updated_at":              now,
		})

	// 先检查 DB 错误，再判断 RowsAffected
	if result.Error != nil {
		return "", fmt.Errorf("save refreshed tokens failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 其他实例已抢先刷新，reload identity 后用新 token
		reloaded := &model.UpstreamIdentity{}
		if err := model.DB.First(reloaded, identity.Id).Error; err != nil {
			return "", errors.New("identity stale after concurrent refresh")
		}
		accessToken, decErr := DecryptUpstreamPassword(reloaded.AccessTokenEnc)
		if decErr != nil {
			return "", fmt.Errorf("decrypt access token after race: %w", decErr)
		}
		if strings.TrimSpace(accessToken) == "" {
			return "", errors.New("access token empty after concurrent refresh")
		}
		return accessToken, nil
	}

	return newAccessToken, nil
}

func upstreamTokenRefreshLeadSeconds() int64 {
	lead := int64(tokenRefreshMarginSeconds)
	setting := operation_setting.GetMonitorSetting()
	if !setting.AutoPriorityScanEnabled {
		return lead
	}
	interval := normalizeAutoPriorityScanInterval(setting.AutoPriorityScanIntervalHours)
	if interval <= 0 {
		return lead
	}
	intervalLead := int64(interval/time.Second) + tokenRefreshMarginSeconds
	if intervalLead > lead {
		return intervalLead
	}
	return lead
}

// sub2apiRefreshTokens 调用上游 /api/v1/auth/refresh 刷新 token。
func sub2apiRefreshTokens(ctx context.Context, client *http.Client, baseURL string, refreshToken string) (string, string, int64, error) {
	payload := map[string]string{
		"refresh_token": strings.TrimSpace(refreshToken),
	}
	bodyBytes, err := common.Marshal(payload)
	if err != nil {
		return "", "", 0, err
	}

	refreshURL := strings.TrimRight(baseURL, "/") + "/api/v1/auth/refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		bodyStr := strings.ToLower(strings.TrimSpace(string(body)))
		if strings.Contains(bodyStr, "invalid") || strings.Contains(bodyStr, "expired") ||
			strings.Contains(bodyStr, "revoked") || strings.Contains(bodyStr, "not found") {
			return "", "", 0, fmt.Errorf("refresh token is invalid or expired: %s", resp.Status)
		}
		return "", "", 0, fmt.Errorf("refresh endpoint returned %s", resp.Status)
	}

	var response map[string]any
	if err := common.DecodeJson(io.LimitReader(resp.Body, 1<<20), &response); err != nil {
		return "", "", 0, err
	}

	data, _ := response["data"].(map[string]any)
	if data == nil {
		return "", "", 0, errors.New("refresh response missing data")
	}

	accessToken, _ := data["access_token"].(string)
	newRefreshToken, _ := data["refresh_token"].(string)
	expiresIn, _ := asFloat64(data["expires_in"])
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	accessToken = strings.TrimSpace(accessToken)
	newRefreshToken = strings.TrimSpace(newRefreshToken)
	if accessToken == "" {
		return "", "", 0, errors.New("refresh response missing access_token")
	}

	return accessToken, newRefreshToken, int64(expiresIn), nil
}
