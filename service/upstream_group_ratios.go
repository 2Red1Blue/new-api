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

// UpstreamPricingCredential 存储上游登录凭据，用于获取分组倍率
type UpstreamPricingCredential struct {
	Account  string
	Password string
}

// UpstreamGroupSnapshotEntry sub2api 分组快照条目
type UpstreamGroupSnapshotEntry struct {
	ID                   int64    `json:"id"`
	Platform             string   `json:"platform"`
	RateMultiplier       float64  `json:"rate_multiplier"`
	RPMLimit             int      `json:"rpm_limit"`
	AllowImageGeneration bool     `json:"allow_image_generation"`
	ImageRateIndependent bool     `json:"image_rate_independent"`
	ImageRateMultiplier  float64  `json:"image_rate_multiplier"`
	ImagePrice1K         *float64 `json:"image_price_1k"`
	ImagePrice2K         *float64 `json:"image_price_2k"`
	ImagePrice4K         *float64 `json:"image_price_4k"`
	Status               string   `json:"status"`
	// HasRPMLimit 为 true 仅当上游 JSON 明确包含 rpm_limit 字段，避免将空缺字段解释为 0 覆盖手动配置
	HasRPMLimit bool `json:"-"`
}

// UpstreamGroupRatiosResult 拉取结果
type UpstreamGroupRatiosResult struct {
	Ratios  map[string]float64
	Raw     string
	Source  string
	Details map[string]UpstreamGroupSnapshotEntry
}

// UpstreamCredentialFromProfileRecord 从 ChannelUpstreamProfile 提取凭据。
// account 为空 → nil（无凭据）；password 为空是允许的（sub2api token 直连模式）。
// 解密失败 → nil（调用方应视情况返回用户友好错误）。
func UpstreamCredentialFromProfileRecord(profile *model.ChannelUpstreamProfile) *UpstreamPricingCredential {
	account := strings.TrimSpace(profile.UpstreamAccount)
	if account == "" {
		return nil
	}
	password := ""
	if strings.TrimSpace(profile.UpstreamPasswordEnc) != "" {
		var err error
		password, err = DecryptUpstreamPassword(profile.UpstreamPasswordEnc)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to decrypt upstream password for channel #%d: %s", profile.ChannelId, err.Error()))
			return nil
		}
	}
	return &UpstreamPricingCredential{Account: account, Password: password}
}

// FetchUpstreamGroupRatios 按以下顺序尝试获取上游分组倍率：
//  1. account + password → 优先登录 new-api 后获取标准端点，避免未登录接口返回 HTML 登录页
//  2. account + password → 再尝试 sub2api 邮箱+密码登录获取 token
//  3. password 为空，account 当 sub2api access token 直连
//  4. 无认证试标准 new-api 端点（/api/ratio_config, /api/pricing）
//  5. 无认证试 sub2api 端点（/api/v1/groups/available）
func FetchUpstreamGroupRatios(ctx context.Context, client *http.Client, baseURL string, credential *UpstreamPricingCredential) (*UpstreamGroupRatiosResult, error) {
	account := ""
	password := ""
	if credential != nil {
		account = strings.TrimSpace(credential.Account)
		password = strings.TrimSpace(credential.Password)
	}
	hasFullCredential := account != "" && password != ""

	var sub2apiResult *UpstreamGroupRatiosResult
	var sub2apiErr error
	var legacyResult *UpstreamGroupRatiosResult
	var legacyErr error
	var loginErr error
	var sub2apiLoginErr error

	// account + password：先尝试 new-api 登录
	if hasFullCredential {
		authClient, userID, err := loginUpstreamLegacy(ctx, baseURL, &UpstreamPricingCredential{Account: account, Password: password})
		loginErr = err
		if loginErr == nil {
			legacyResult, legacyErr = fetchUpstreamGroupRatiosLegacy(ctx, authClient, baseURL, userID)
			if legacyErr == nil {
				return legacyResult, nil
			}
		}

		// 再尝试 sub2api 邮箱+密码登录
		sub2apiClient, accessToken, err := loginSub2API(ctx, baseURL, &UpstreamPricingCredential{Account: account, Password: password})
		sub2apiLoginErr = err
		if sub2apiLoginErr == nil {
			sub2apiResult, sub2apiErr = fetchSub2APIGroupRatios(ctx, sub2apiClient, baseURL, accessToken)
			if sub2apiErr == nil {
				return sub2apiResult, nil
			}
		}
	}

	// account 非空、password 为空：sub2api token 直连模式
	if account != "" && password == "" {
		sub2apiResult, sub2apiErr = fetchSub2APIGroupRatios(ctx, client, baseURL, account)
		if sub2apiErr == nil {
			return sub2apiResult, nil
		}
	}

	// 无凭据或认证路径失败后，再尝试匿名接口作为 fallback。
	legacyResult, legacyErr = fetchUpstreamGroupRatiosLegacy(ctx, client, baseURL, "")
	if legacyErr == nil {
		return legacyResult, nil
	}

	if !hasFullCredential {
		sub2apiResult, sub2apiErr = fetchSub2APIGroupRatios(ctx, client, baseURL, "")
		if sub2apiErr == nil {
			return sub2apiResult, nil
		}
	}

	err := JoinGroupFetchErrors(legacyErr, sub2apiErr)
	if hasFullCredential {
		if loginErr != nil {
			err = fmt.Errorf("%w；newapi 登录失败: %v", err, loginErr)
		}
		if sub2apiLoginErr != nil {
			err = fmt.Errorf("%w；sub2api 登录失败: %v", err, sub2apiLoginErr)
		}
	}
	return nil, err
}

// FetchUpstreamTopupRatio 从上游 /api/status 获取充值倍率。
func FetchUpstreamTopupRatio(ctx context.Context, client *http.Client, baseURL string) (float64, bool, error) {
	body, err := fetchUpstreamJSON(ctx, client, baseURL+"/api/status")
	if err != nil {
		return 0, false, err
	}
	data := unwrapSuccessData(body)
	if ratio, ok := findFloatByKeys(data, "topup_ratio", "top_up_ratio", "recharge_ratio", "upstream_topup_ratio"); ok && ratio > 0 {
		return ratio, true, nil
	}
	price, priceOK := findFloatByKeys(data, "price")
	quotaPerUnit, quotaOK := findFloatByKeys(data, "quota_per_unit")
	if priceOK && quotaOK && price > 0 && quotaPerUnit > 0 {
		return quotaPerUnit / common.QuotaPerUnit / price, true, nil
	}
	return 0, false, nil
}

// JoinGroupFetchErrors 将两个错误合并为带上下文的错误链。
func JoinGroupFetchErrors(primary, secondary error) error {
	switch {
	case primary == nil:
		return secondary
	case secondary == nil:
		return primary
	default:
		return fmt.Errorf("%w；备用接口失败: %v", primary, secondary)
	}
}

// LoginUpstreamLegacy 使用 new-api 账号密码登录，返回带 cookie 的 client 和 userID。
// 供需要认证后做 GET 请求的调用方使用。
func LoginUpstreamLegacy(ctx context.Context, baseURL string, credential *UpstreamPricingCredential) (*http.Client, string, error) {
	return loginUpstreamLegacy(ctx, baseURL, credential)
}

// --- new-api 认证路径 ---

func fetchUpstreamGroupRatiosLegacy(ctx context.Context, client *http.Client, baseURL string, userID string) (*UpstreamGroupRatiosResult, error) {
	endpoints := []string{"/api/ratio_config", "/api/pricing"}
	var lastErr error
	for _, endpoint := range endpoints {
		source := baseURL + endpoint
		body, err := fetchUpstreamJSONWithAuth(ctx, client, source, userID, "")
		if err != nil {
			lastErr = err
			continue
		}
		data := unwrapSuccessData(body)
		ratios := extractFloatMapByKeys(body, "group_ratio", "group_ratios", "topup_group_ratio")
		if len(ratios) == 0 {
			ratios = extractFloatMapByKeys(data, "group_ratio", "group_ratios", "topup_group_ratio")
		}
		if len(ratios) == 0 {
			lastErr = errors.New("上游未返回 group_ratio")
			continue
		}
		rawBytes, err := common.MarshalIndent(ratios, "", "  ")
		if err != nil {
			return nil, err
		}
		return &UpstreamGroupRatiosResult{
			Ratios: ratios,
			Raw:    string(rawBytes),
			Source: source,
		}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("上游未返回 group_ratio")
	}
	return nil, lastErr
}

func loginUpstreamLegacy(ctx context.Context, baseURL string, credential *UpstreamPricingCredential) (*http.Client, string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
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
	userID := findUserIDString(response["data"])
	if userID == "" {
		return nil, "", errors.New("上游登录未返回用户 ID")
	}
	return client, userID, nil
}

// --- sub2api 认证路径 ---

func fetchSub2APIGroupRatios(ctx context.Context, client *http.Client, baseURL string, accessToken string) (*UpstreamGroupRatiosResult, error) {
	source := baseURL + "/api/v1/groups/available"
	body, err := fetchUpstreamJSONWithAuth(ctx, client, source, "", accessToken)
	if err != nil {
		return nil, err
	}
	ratios, details, err := parseSub2APIGroupSnapshot(body)
	if err != nil {
		return nil, err
	}
	rawBytes, err := common.MarshalIndent(details, "", "  ")
	if err != nil {
		return nil, err
	}
	return &UpstreamGroupRatiosResult{
		Ratios:  ratios,
		Raw:     string(rawBytes),
		Source:  source,
		Details: details,
	}, nil
}

func loginSub2API(ctx context.Context, baseURL string, credential *UpstreamPricingCredential) (*http.Client, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	payload := map[string]string{
		"email":    strings.TrimSpace(credential.Account),
		"password": strings.TrimSpace(credential.Password),
	}
	bodyBytes, err := common.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(bodyBytes))
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
	data := unwrapSuccessData(response)
	record, _ := data.(map[string]any)
	accessToken, _ := record["access_token"].(string)
	if strings.TrimSpace(accessToken) == "" {
		message, _ := response["message"].(string)
		if strings.TrimSpace(message) == "" {
			message = "sub2api 登录未返回 access_token"
		}
		return nil, "", errors.New(message)
	}
	return client, strings.TrimSpace(accessToken), nil
}

// parseSub2APIGroupSnapshot 解析 /api/v1/groups/available 响应。
// HasRPMLimit 仅当 record["rpm_limit"] 字段明确存在时为 true，不以零值推断（Bug1 修复）。
func parseSub2APIGroupSnapshot(payload map[string]any) (map[string]float64, map[string]UpstreamGroupSnapshotEntry, error) {
	data := unwrapSuccessData(payload)
	items, ok := data.([]any)
	if !ok {
		return nil, nil, errors.New("sub2api groups/available 返回格式错误")
	}
	ratios := make(map[string]float64, len(items))
	details := make(map[string]UpstreamGroupSnapshotEntry, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := record["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		rateMultiplier, ok := asFloat64(record["rate_multiplier"])
		if !ok {
			continue
		}
		_, hasRPMLimit := record["rpm_limit"]
		entry := UpstreamGroupSnapshotEntry{
			Platform:             strings.TrimSpace(upstreamToString(record["platform"])),
			RateMultiplier:       rateMultiplier,
			RPMLimit:             upstreamToInt(record["rpm_limit"]),
			AllowImageGeneration: upstreamToBool(record["allow_image_generation"]),
			ImageRateIndependent: upstreamToBool(record["image_rate_independent"]),
			ImageRateMultiplier:  upstreamToFloatDefault(record["image_rate_multiplier"], 1),
			ImagePrice1K:         upstreamToOptionalFloat(record["image_price_1k"]),
			ImagePrice2K:         upstreamToOptionalFloat(record["image_price_2k"]),
			ImagePrice4K:         upstreamToOptionalFloat(record["image_price_4k"]),
			Status:               strings.TrimSpace(upstreamToString(record["status"])),
			HasRPMLimit:          hasRPMLimit,
		}
		if id, ok := asFloat64(record["id"]); ok {
			entry.ID = int64(id)
		}
		ratios[name] = rateMultiplier
		details[name] = entry
	}
	if len(ratios) == 0 {
		return nil, nil, errors.New("sub2api groups/available 未返回有效分组倍率")
	}
	return ratios, details, nil
}

// --- HTTP 工具 ---

func fetchUpstreamJSON(ctx context.Context, client *http.Client, url string) (map[string]any, error) {
	return fetchUpstreamJSONWithAuth(ctx, client, url, "", "")
}

func fetchUpstreamJSONWithAuth(ctx context.Context, client *http.Client, url string, userID string, bearerToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) != "" {
		req.Header.Set("New-Api-User", strings.TrimSpace(userID))
	}
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
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

// --- 数据解析工具 ---

func unwrapSuccessData(payload map[string]any) any {
	if data, ok := payload["data"]; ok {
		return data
	}
	return payload
}

func extractFloatMapByKeys(value any, keys ...string) map[string]float64 {
	if record, ok := value.(map[string]any); ok {
		for _, key := range keys {
			if ratios := convertFloatMap(record[key]); len(ratios) > 0 {
				return ratios
			}
		}
		for _, nestedKey := range []string{"data", "ratio_config", "ratios"} {
			if ratios := extractFloatMapByKeys(record[nestedKey], keys...); len(ratios) > 0 {
				return ratios
			}
		}
	}
	return nil
}

func convertFloatMap(value any) map[string]float64 {
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

func findFloatByKeys(value any, keys ...string) (float64, bool) {
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
		if number, ok := findFloatByKeys(record[nestedKey], keys...); ok {
			return number, true
		}
	}
	return 0, false
}

func findUserIDString(value any) string {
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
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func upstreamToString(value any) string {
	str, _ := value.(string)
	return str
}

func upstreamToBool(value any) bool {
	b, _ := value.(bool)
	return b
}

func upstreamToInt(value any) int {
	if number, ok := asFloat64(value); ok {
		return int(number)
	}
	return 0
}

func upstreamToFloatDefault(value any, fallback float64) float64 {
	if number, ok := asFloat64(value); ok {
		return number
	}
	return fallback
}

func upstreamToOptionalFloat(value any) *float64 {
	if number, ok := asFloat64(value); ok {
		return &number
	}
	return nil
}
