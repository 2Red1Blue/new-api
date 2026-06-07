package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type copyChannelResponse struct {
	Success bool `json:"success"`
	Message string `json:"message"`
	Data struct {
		ID int `json:"id"`
	} `json:"data"`
}

func setupCopyChannelControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelUpstreamProfile{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func decodeCopyChannelResponse(t *testing.T, recorder *httptest.ResponseRecorder) copyChannelResponse {
	t.Helper()

	var payload copyChannelResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestCopyChannelClonesUpstreamProfilesAndResetsRuntimeState(t *testing.T) {
	db := setupCopyChannelControllerTestDB(t)

	priority := int64(9)
	weight := uint(5)
	baseURL := "https://upstream.example.com"
	settings := `{"upstream_rpm_limit":12,"foo":"bar"}`
	tag := "origin-tag"
	remark := "origin-remark"

	origin := &model.Channel{
		Type:         1,
		Key:          "sk-origin",
		Status:       common.ChannelStatusEnabled,
		Name:         "origin",
		Weight:       &weight,
		CreatedTime:  111,
		TestTime:     222,
		ResponseTime: 333,
		BaseURL:      &baseURL,
		Balance:      12.5,
		Models:       "gpt-4o,claude-3-5-sonnet",
		Group:        "default,vip",
		UsedQuota:    456,
		Priority:     &priority,
		Tag:          &tag,
		Remark:       &remark,
		OtherSettings: settings,
	}
	require.NoError(t, origin.Insert())

	now := int64(123456789)
	require.NoError(t, db.Create(&[]model.ChannelUpstreamProfile{
		{
			ChannelId:                   origin.Id,
			KeyFingerprint:              "fp-1",
			KeyMasked:                   "sk-...gin1",
			KeyLabel:                    "primary",
			UpstreamAccount:             "alice@example.com",
			UpstreamPasswordEnc:         "ciphertext-1",
			UpstreamLoginUrl:            "https://login.example.com",
			UpstreamGroup:               "Claude混池",
			UpstreamGroupRatio:          0.5,
			UpstreamTopupRatio:          1.2,
			UpstreamGroupRatios:         `{"Claude混池":{"rate_multiplier":0.5,"rpm_limit":12}}`,
			AutoPriorityEnabled:         true,
			AutoPriorityBase:            2,
			AutoPriorityMin:             1,
			AutoPriorityMax:             99,
			AutoPriorityValue:           4,
			AutoPriorityUpdatedAt:       now,
			AutoPriorityReason:          "copied",
			InsufficientBalanceKeywords: "余额不足",
			NotifyEnabled:               true,
			LastInsufficientAt:          now - 30,
			LastInsufficientReason:      "old insufficient",
			LastNotifiedAt:              now - 20,
			NotifySuppressUntil:         now + 60,
			CreatedAt:                   now - 100,
			UpdatedAt:                   now - 10,
		},
		{
			ChannelId:                   origin.Id,
			KeyFingerprint:              "fp-2",
			KeyMasked:                   "sk-...gin2",
			KeyLabel:                    "backup",
			UpstreamAccount:             "bob@example.com",
			UpstreamPasswordEnc:         "ciphertext-2",
			UpstreamLoginUrl:            "https://login2.example.com",
			UpstreamGroup:               "OpenAI-Pro",
			UpstreamGroupRatio:          0.2,
			UpstreamTopupRatio:          1,
			UpstreamGroupRatios:         `{"OpenAI-Pro":{"rate_multiplier":0.2,"rpm_limit":8,"image_rate_multiplier":1}}`,
			AutoPriorityEnabled:         false,
			AutoPriorityBase:            1,
			AutoPriorityMin:             0,
			AutoPriorityMax:             100,
			AutoPriorityValue:           0,
			AutoPriorityUpdatedAt:       now,
			AutoPriorityReason:          "disabled",
			InsufficientBalanceKeywords: "quota exceeded",
			NotifyEnabled:               false,
			LastInsufficientAt:          now - 50,
			LastInsufficientReason:      "legacy",
			LastNotifiedAt:              now - 40,
			NotifySuppressUntil:         now + 120,
			CreatedAt:                   now - 200,
			UpdatedAt:                   now - 5,
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", origin.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/copy/%d?suffix=_copy&reset_balance=true", origin.Id), nil)

	CopyChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeCopyChannelResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)
	require.NotZero(t, payload.Data.ID)
	require.NotEqual(t, origin.Id, payload.Data.ID)

	cloned, err := model.GetChannelById(payload.Data.ID, true)
	require.NoError(t, err)
	require.Equal(t, "origin_copy", cloned.Name)
	require.Equal(t, 0.0, cloned.Balance)
	require.EqualValues(t, 0, cloned.UsedQuota)
	require.Equal(t, settings, cloned.OtherSettings)
	require.Equal(t, origin.Models, cloned.Models)
	require.Equal(t, origin.Group, cloned.Group)
	require.Equal(t, origin.Key, cloned.Key)
	require.Equal(t, 0, cloned.ResponseTime)
	require.EqualValues(t, 0, cloned.TestTime)

	var abilities []model.Ability
	require.NoError(t, db.Where("channel_id = ?", cloned.Id).Find(&abilities).Error)
	require.Len(t, abilities, 4)

	var upstreamProfiles []model.ChannelUpstreamProfile
	require.NoError(t, db.Where("channel_id = ?", cloned.Id).Order("key_label ASC").Find(&upstreamProfiles).Error)
	require.Len(t, upstreamProfiles, 2)

	require.Equal(t, "backup", upstreamProfiles[0].KeyLabel)
	require.Equal(t, "ciphertext-2", upstreamProfiles[0].UpstreamPasswordEnc)
	require.Equal(t, "OpenAI-Pro", upstreamProfiles[0].UpstreamGroup)
	require.Equal(t, `{"OpenAI-Pro":{"rate_multiplier":0.2,"rpm_limit":8,"image_rate_multiplier":1}}`, upstreamProfiles[0].UpstreamGroupRatios)
	require.EqualValues(t, 0, upstreamProfiles[0].LastInsufficientAt)
	require.Equal(t, "", upstreamProfiles[0].LastInsufficientReason)
	require.EqualValues(t, 0, upstreamProfiles[0].LastNotifiedAt)
	require.EqualValues(t, 0, upstreamProfiles[0].NotifySuppressUntil)

	require.Equal(t, "primary", upstreamProfiles[1].KeyLabel)
	require.Equal(t, "ciphertext-1", upstreamProfiles[1].UpstreamPasswordEnc)
	require.Equal(t, "Claude混池", upstreamProfiles[1].UpstreamGroup)
	require.Equal(t, 0.5, upstreamProfiles[1].UpstreamGroupRatio)
	require.Equal(t, `{"Claude混池":{"rate_multiplier":0.5,"rpm_limit":12}}`, upstreamProfiles[1].UpstreamGroupRatios)
	require.EqualValues(t, 0, upstreamProfiles[1].LastInsufficientAt)
	require.Equal(t, "", upstreamProfiles[1].LastInsufficientReason)
	require.EqualValues(t, 0, upstreamProfiles[1].LastNotifiedAt)
	require.EqualValues(t, 0, upstreamProfiles[1].NotifySuppressUntil)
}

func TestCopyChannelCanKeepBalanceWhenRequested(t *testing.T) {
	setupCopyChannelControllerTestDB(t)

	origin := &model.Channel{
		Type:         1,
		Key:          "sk-origin",
		Status:       common.ChannelStatusEnabled,
		Name:         "origin-balance",
		CreatedTime:  111,
		Balance:      66.6,
		Models:       "gpt-4o",
		Group:        "default",
		UsedQuota:    789,
		OtherSettings: `{"upstream_rpm_limit":18}`,
	}
	require.NoError(t, origin.Insert())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", origin.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/copy/%d?suffix=_keep&reset_balance=false", origin.Id), nil)

	CopyChannel(ctx)

	payload := decodeCopyChannelResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)

	cloned, err := model.GetChannelById(payload.Data.ID, true)
	require.NoError(t, err)
	require.Equal(t, 66.6, cloned.Balance)
	require.EqualValues(t, 789, cloned.UsedQuota)
	require.Equal(t, `{"upstream_rpm_limit":18}`, cloned.OtherSettings)
}
