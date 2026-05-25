package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// upstreamPricingCredential 存储上游登录凭据，用于获取分组倍率
type upstreamPricingCredential struct {
	Account  string
	Password string
}

// asFloat64 将任意数值类型转为 float64，支持 json.Number
func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		// json.Number 是类型引用，不调用 encoding/json 的 marshal/unmarshal
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func upstreamCredentialFromProfileRecord(profile *model.ChannelUpstreamProfile) *upstreamPricingCredential {
	account := strings.TrimSpace(profile.UpstreamAccount)
	if account == "" || strings.TrimSpace(profile.UpstreamPasswordEnc) == "" {
		return nil
	}
	password, err := DecryptUpstreamPassword(profile.UpstreamPasswordEnc)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to decrypt upstream password for channel #%d: %s", profile.ChannelId, err.Error()))
		return nil
	}
	return &upstreamPricingCredential{
		Account:  account,
		Password: password,
	}
}

func fetchUpstreamGroupRatiosForSync(ctx context.Context, client *http.Client, baseURL string, credential *upstreamPricingCredential) (map[string]float64, string, string, error) {
	ratios, raw, source, err := fetchUpstreamGroupRatiosWithClientForSync(ctx, client, baseURL, "")
	if err == nil {
		return ratios, raw, source, nil
	}
	if credential == nil || strings.TrimSpace(credential.Account) == "" || strings.TrimSpace(credential.Password) == "" {
		return nil, "", "", err
	}
	authClient, userID, loginErr := loginUpstreamForSync(ctx, baseURL, credential)
	if loginErr != nil {
		return nil, "", "", fmt.Errorf("%w；登录上游获取价格失败: %v", err, loginErr)
	}
	ratios, raw, source, authErr := fetchUpstreamGroupRatiosWithClientForSync(ctx, authClient, baseURL, userID)
	if authErr != nil {
		return nil, "", "", fmt.Errorf("%w；登录后获取价格失败: %v", err, authErr)
	}
	return ratios, raw, source, nil
}

func fetchUpstreamGroupRatiosWithClientForSync(ctx context.Context, client *http.Client, baseURL string, userID string) (map[string]float64, string, string, error) {
	endpoints := []string{"/api/ratio_config", "/api/pricing"}
	var lastErr error
	for _, endpoint := range endpoints {
		source := baseURL + endpoint
		body, err := fetchUpstreamJSONWithUserForSync(ctx, client, source, userID)
		if err != nil {
			lastErr = err
			continue
		}
		data := unwrapSuccessDataForSync(body)
		ratios := extractFloatMapByKeysForSync(body, "group_ratio", "group_ratios", "topup_group_ratio")
		if len(ratios) == 0 {
			ratios = extractFloatMapByKeysForSync(data, "group_ratio", "group_ratios", "topup_group_ratio")
		}
		if len(ratios) == 0 {
			lastErr = errors.New("上游未返回 group_ratio")
			continue
		}
		rawBytes, err := common.MarshalIndent(ratios, "", "  ")
		if err != nil {
			return nil, "", "", err
		}
		return ratios, string(rawBytes), source, nil
	}
	if lastErr == nil {
		lastErr = errors.New("上游未返回 group_ratio")
	}
	return nil, "", "", lastErr
}

func loginUpstreamForSync(ctx context.Context, baseURL string, credential *upstreamPricingCredential) (*http.Client, string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
	}
	payload := map[string]string{
		"username": strings.TrimSpace(credential.Account),
		"password": strings.TrimSpace(credential.Password),
	}
	bodyBytes, err := common.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/user/login", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("上游登录返回 %s", resp.Status)
	}
	var response map[string]any
	if err := common.DecodeJson(io.LimitReader(resp.Body, 1<<20), &response); err != nil {
		return nil, "", err
	}
	success, _ := response["success"].(bool)
	if !success {
		message, _ := response["message"].(string)
		if strings.TrimSpace(message) == "" {
			message = "上游登录失败"
		}
		return nil, "", errors.New(message)
	}
	userID := findUserIDStringForSync(response["data"])
	if userID == "" {
		return nil, "", errors.New("上游登录未返回用户 ID")
	}
	return client, userID, nil
}

func findUserIDStringForSync(value any) string {
	if record, ok := value.(map[string]any); ok {
		for _, key := range []string{"id", "user_id", "uid"} {
			if number, ok := asFloat64(record[key]); ok {
				return strconv.FormatInt(int64(number), 10)
			}
			if str, ok := record[key].(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

func fetchUpstreamTopupRatioForSync(ctx context.Context, client *http.Client, baseURL string) (float64, bool, error) {
	body, err := fetchUpstreamJSONForSync(ctx, client, baseURL+"/api/status")
	if err != nil {
		return 0, false, err
	}
	data := unwrapSuccessDataForSync(body)
	if ratio, ok := findFloatByKeysForSync(data, "topup_ratio", "top_up_ratio", "recharge_ratio", "upstream_topup_ratio"); ok && ratio > 0 {
		return ratio, true, nil
	}
	price, priceOK := findFloatByKeysForSync(data, "price")
	quotaPerUnit, quotaOK := findFloatByKeysForSync(data, "quota_per_unit")
	if priceOK && quotaOK && price > 0 && quotaPerUnit > 0 {
		return quotaPerUnit / common.QuotaPerUnit / price, true, nil
	}
	return 0, false, nil
}

func fetchUpstreamJSONForSync(ctx context.Context, client *http.Client, url string) (map[string]any, error) {
	return fetchUpstreamJSONWithUserForSync(ctx, client, url, "")
}

func fetchUpstreamJSONWithUserForSync(ctx context.Context, client *http.Client, url string, userID string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) != "" {
		req.Header.Set("New-Api-User", strings.TrimSpace(userID))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("上游返回 %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := common.DecodeJson(bytes.NewReader(body), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func unwrapSuccessDataForSync(payload map[string]any) any {
	if data, ok := payload["data"]; ok {
		return data
	}
	return payload
}

func extractFloatMapByKeysForSync(value any, keys ...string) map[string]float64 {
	if record, ok := value.(map[string]any); ok {
		for _, key := range keys {
			if ratios := convertFloatMapForSync(record[key]); len(ratios) > 0 {
				return ratios
			}
		}
		for _, nestedKey := range []string{"data", "ratio_config", "ratios"} {
			if ratios := extractFloatMapByKeysForSync(record[nestedKey], keys...); len(ratios) > 0 {
				return ratios
			}
		}
	}
	return nil
}

func convertFloatMapForSync(value any) map[string]float64 {
	record, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]float64, len(record))
	for key, raw := range record {
		if ratio, ok := asFloat64(raw); ok {
			result[key] = ratio
		}
	}
	return result
}

func findFloatByKeysForSync(value any, keys ...string) (float64, bool) {
	record, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	for _, key := range keys {
		if number, ok := asFloat64(record[key]); ok {
			return number, true
		}
	}
	for _, nestedKey := range []string{"data", "status", "payment", "config"} {
		if number, ok := findFloatByKeysForSync(record[nestedKey], keys...); ok {
			return number, true
		}
	}
	return 0, false
}

// syncChannelUpstreamGroupRatio 拉取单个 profile 的上游分组倍率并更新数据库
func syncChannelUpstreamGroupRatio(ctx context.Context, profile *model.ChannelUpstreamProfile) error {
	baseURL := strings.TrimSpace(profile.UpstreamLoginUrl)
	if baseURL == "" {
		return nil
	}

	credential := upstreamCredentialFromProfileRecord(profile)
	client := &http.Client{Timeout: 10 * time.Second}

	// 拉取分组倍率
	groupRatios, _, _, err := fetchUpstreamGroupRatiosForSync(ctx, client, baseURL, credential)
	if err != nil {
		return fmt.Errorf("获取分组倍率失败: %w", err)
	}

	// 从分组倍率中提取当前上游分组对应的倍率
	groupName := strings.TrimSpace(profile.UpstreamGroup)
	groupRatio := float64(0)
	if groupName != "" {
		if r, ok := groupRatios[groupName]; ok {
			groupRatio = r
		}
	}

	// 拉取充值倍率（可选，失败不阻断）
	topupRatio := profile.UpstreamTopupRatio
	if tr, ok, _ := fetchUpstreamTopupRatioForSync(ctx, client, baseURL); ok && tr > 0 {
		topupRatio = tr
	}

	// 序列化完整分组倍率 JSON
	groupRatiosJSON := ""
	if len(groupRatios) > 0 {
		if b, err := common.Marshal(groupRatios); err == nil {
			groupRatiosJSON = string(b)
		}
	}

	now := common.GetTimestamp()
	updates := map[string]any{
		"upstream_group_ratio":  groupRatio,
		"upstream_topup_ratio":  topupRatio,
		"upstream_group_ratios": groupRatiosJSON,
		"updated_at":            now,
	}
	return model.DB.Model(&model.ChannelUpstreamProfile{}).
		Where("id = ?", profile.Id).
		Updates(updates).Error
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
