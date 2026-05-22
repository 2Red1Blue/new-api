package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

const upstreamNotifySuppressSeconds = 2 * 60 * 60

type UpstreamInsufficientNotificationContext struct {
	ChannelError   types.ChannelError
	ErrorMessage   string
	ModelName      string
	UsingGroup     string
	MatchedKeyword string
}

func IsUpstreamPasswordEnabled() bool {
	return strings.TrimSpace(common.UpstreamSecretKey) != ""
}

func upstreamSecretBytes() []byte {
	sum := sha256.Sum256([]byte(common.UpstreamSecretKey))
	return sum[:]
}

func EncryptUpstreamPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}
	if !IsUpstreamPasswordEnabled() {
		return "", fmt.Errorf("UPSTREAM_SECRET_KEY is not configured")
	}
	block, err := aes.NewCipher(upstreamSecretBytes())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	encrypted := gcm.Seal(nonce, nonce, []byte(password), nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func DecryptUpstreamPassword(encrypted string) (string, error) {
	encrypted = strings.TrimSpace(encrypted)
	if encrypted == "" {
		return "", nil
	}
	if !IsUpstreamPasswordEnabled() {
		return "", fmt.Errorf("UPSTREAM_SECRET_KEY is not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(upstreamSecretBytes())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted upstream password is invalid")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func parseKeywordLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	parts := strings.Split(raw, "\n")
	keywords := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		keyword := strings.ToLower(strings.TrimSpace(part))
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return keywords
}

func matchInsufficientBalanceKeyword(message string, profile *model.ChannelUpstreamProfile) (string, bool) {
	message = strings.ToLower(message)
	rawKeywordParts := []string{model.DefaultInsufficientBalanceKeywords}
	if len(operation_setting.AutomaticDisableKeywords) > 0 {
		rawKeywordParts = append(rawKeywordParts, strings.Join(operation_setting.AutomaticDisableKeywords, "\n"))
	}
	if profile != nil && strings.TrimSpace(profile.InsufficientBalanceKeywords) != "" {
		rawKeywordParts = append(rawKeywordParts, profile.InsufficientBalanceKeywords)
	}
	for _, keyword := range parseKeywordLines(strings.Join(rawKeywordParts, "\n")) {
		if strings.Contains(message, keyword) {
			return keyword, true
		}
	}
	return "", false
}

func shouldNotifyProfile(profile *model.ChannelUpstreamProfile, now int64) bool {
	if profile == nil || !profile.NotifyEnabled {
		return false
	}
	return profile.NotifySuppressUntil <= now
}

func updateInsufficientState(profile *model.ChannelUpstreamProfile, reason string, notified bool, now int64) error {
	updates := map[string]any{
		"last_insufficient_at":     now,
		"last_insufficient_reason": truncateForDB(reason, 512),
		"updated_at":               now,
	}
	if notified {
		updates["last_notified_at"] = now
		updates["notify_suppress_until"] = now + upstreamNotifySuppressSeconds
	}
	return model.DB.Model(&model.ChannelUpstreamProfile{}).
		Where("id = ?", profile.Id).
		Updates(updates).Error
}

func truncateForDB(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func formatGroupRatios(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	ratios := make(map[string]float64)
	if err := common.UnmarshalJsonStr(raw, &ratios); err != nil {
		return raw
	}
	keys := make([]string, 0, len(ratios))
	for key := range ratios {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("- %s: %.4gx", key, ratios[key]))
	}
	return strings.Join(lines, "<br/>")
}

func buildInsufficientBalanceEmail(profile *model.ChannelUpstreamProfile, ctx UpstreamInsufficientNotificationContext) (string, string) {
	channel := ctx.ChannelError
	subject := fmt.Sprintf("上游渠道余额不足：%s (#%d)", channel.ChannelName, channel.ChannelId)
	statusText := "该渠道可能已被自动暂停，充值后请回到渠道管理手动重新启用。"
	content := fmt.Sprintf(
		`<p>检测到上游渠道可能余额不足。</p>
<p><strong>渠道：</strong>%s (#%d)<br/>
<strong>渠道类型：</strong>%d<br/>
<strong>触发模型：</strong>%s<br/>
<strong>本站使用分组：</strong>%s</p>
<p><strong>上游账号：</strong>%s<br/>
<strong>上游登录地址：</strong>%s<br/>
<strong>上游分组：</strong>%s<br/>
<strong>上游分组倍率：</strong>%.4gx<br/>
<strong>上游完整分组倍率：</strong><br/>%s</p>
<p><strong>欠费 key：</strong>%s<br/>
<strong>命中的欠费关键词：</strong>%s<br/>
<strong>错误原因：</strong>%s</p>
<p><strong>充值后操作：</strong><br/>
1. 登录上游：%s<br/>
2. 给账号 %s 充值<br/>
3. 回到渠道管理页面，手动启用该渠道</p>
<p><strong>当前渠道状态：</strong>%s<br/>
<strong>静默提示：</strong>此 key 2 小时内不再重复提醒。</p>`,
		channel.ChannelName,
		channel.ChannelId,
		channel.ChannelType,
		emptyDash(ctx.ModelName),
		emptyDash(ctx.UsingGroup),
		emptyDash(profile.UpstreamAccount),
		emptyDash(profile.UpstreamLoginUrl),
		emptyDash(profile.UpstreamGroup),
		profile.UpstreamGroupRatio,
		formatGroupRatios(profile.UpstreamGroupRatios),
		emptyDash(profile.KeyMasked),
		emptyDash(ctx.MatchedKeyword),
		emptyDash(ctx.ErrorMessage),
		emptyDash(profile.UpstreamLoginUrl),
		emptyDash(profile.UpstreamAccount),
		statusText,
	)
	return subject, content
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func NotifyChannelInsufficientBalance(ctx UpstreamInsufficientNotificationContext) {
	profile, err := model.GetChannelUpstreamProfileByKey(ctx.ChannelError.ChannelId, ctx.ChannelError.UsingKey)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			common.SysLog(fmt.Sprintf("failed to query upstream profile for channel #%d: %s", ctx.ChannelError.ChannelId, err.Error()))
		}
		return
	}
	matched, ok := matchInsufficientBalanceKeyword(ctx.ErrorMessage, profile)
	if !ok {
		return
	}
	ctx.MatchedKeyword = matched
	now := common.GetTimestamp()
	shouldNotify := shouldNotifyProfile(profile, now)
	if err := updateInsufficientState(profile, ctx.ErrorMessage, shouldNotify, now); err != nil {
		common.SysLog(fmt.Sprintf("failed to update upstream insufficient state for channel #%d: %s", ctx.ChannelError.ChannelId, err.Error()))
	}
	if !shouldNotify {
		return
	}
	subject, content := buildInsufficientBalanceEmail(profile, ctx)
	NotifyRootUser(fmt.Sprintf("%s_%d_%s", dto.NotifyTypeChannelUpdate, ctx.ChannelError.ChannelId, model.KeyFingerprint(ctx.ChannelError.UsingKey)), subject, content)
}

func IsInsufficientBalanceError(channelId int, usingKey string, message string) (string, bool) {
	profile, err := model.GetChannelUpstreamProfileByKey(channelId, usingKey)
	if err != nil {
		return matchInsufficientBalanceKeyword(message, nil)
	}
	return matchInsufficientBalanceKeyword(message, profile)
}

func HumanSuppressWindow() string {
	return (time.Duration(upstreamNotifySuppressSeconds) * time.Second).String()
}
