package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamGroupRatioSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelUpstreamProfile{}, &model.Task{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestSyncChannelUpstreamGroupRatioStoresSub2APISnapshotAndRPM(t *testing.T) {
	setupUpstreamGroupRatioSyncTestDB(t)

	origSecret := common.UpstreamSecretKey
	common.UpstreamSecretKey = "test-upstream-secret"
	t.Cleanup(func() {
		common.UpstreamSecretKey = origSecret
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing", "/api/user/login":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		case "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"sub2api-token","token_type":"Bearer"}}`))
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") != "Bearer sub2api-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"id":27,"name":"Claude混池","platform":"anthropic","rate_multiplier":0.5,"rpm_limit":12,"allow_image_generation":false,"image_rate_independent":false,"image_rate_multiplier":1,"image_price_1k":null,"image_price_2k":null,"image_price_4k":null,"status":"active"}]}`))
		case "/api/status":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	passwordEnc, err := EncryptUpstreamPassword("secret")
	require.NoError(t, err)

	channel := &model.Channel{
		Id:            501,
		Name:          "sync-channel",
		Key:           "sk-test",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"upstream_rpm_limit":99}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	profile := &model.ChannelUpstreamProfile{
		ChannelId:           channel.Id,
		KeyFingerprint:      "test-fingerprint",
		KeyMasked:           "sk-t...test",
		UpstreamAccount:     "upstream@example.com",
		UpstreamPasswordEnc: passwordEnc,
		UpstreamLoginUrl:    server.URL,
		UpstreamGroup:       "Claude混池",
		UpstreamTopupRatio:  1,
		CreatedAt:           common.GetTimestamp(),
		UpdatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(profile).Error)

	require.NoError(t, syncChannelUpstreamGroupRatio(context.Background(), profile))

	var savedProfile model.ChannelUpstreamProfile
	require.NoError(t, model.DB.First(&savedProfile, profile.Id).Error)
	require.Equal(t, 0.5, savedProfile.UpstreamGroupRatio)
	require.Contains(t, savedProfile.UpstreamGroupRatios, `"rpm_limit": 12`)
	require.Contains(t, savedProfile.UpstreamGroupRatios, `"image_rate_multiplier": 1`)

	savedChannel, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, 12, savedChannel.GetOtherSettings().UpstreamRPMLimit)
}

func TestSyncChannelUpstreamGroupRatioKeepsRPMWhenSub2APIOmitsRPM(t *testing.T) {
	setupUpstreamGroupRatioSyncTestDB(t)

	origSecret := common.UpstreamSecretKey
	common.UpstreamSecretKey = "test-upstream-secret"
	t.Cleanup(func() {
		common.UpstreamSecretKey = origSecret
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing", "/api/user/login":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		case "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"sub2api-token","token_type":"Bearer"}}`))
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") != "Bearer sub2api-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"id":27,"name":"Claude混池","platform":"anthropic","rate_multiplier":0.5,"allow_image_generation":false,"image_rate_independent":false,"image_rate_multiplier":1,"image_price_1k":null,"image_price_2k":null,"image_price_4k":null,"status":"active"}]}`))
		case "/api/status":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	passwordEnc, err := EncryptUpstreamPassword("secret")
	require.NoError(t, err)

	channel := &model.Channel{
		Id:            502,
		Name:          "sync-channel-keep-rpm",
		Key:           "sk-test",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"upstream_rpm_limit":99}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	profile := &model.ChannelUpstreamProfile{
		ChannelId:           channel.Id,
		KeyFingerprint:      "test-fingerprint-keep-rpm",
		KeyMasked:           "sk-t...test",
		UpstreamAccount:     "upstream@example.com",
		UpstreamPasswordEnc: passwordEnc,
		UpstreamLoginUrl:    server.URL,
		UpstreamGroup:       "Claude混池",
		UpstreamTopupRatio:  1,
		CreatedAt:           common.GetTimestamp(),
		UpdatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(profile).Error)

	require.NoError(t, syncChannelUpstreamGroupRatio(context.Background(), profile))

	savedChannel, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, 99, savedChannel.GetOtherSettings().UpstreamRPMLimit)
}

func TestSyncChannelUpstreamGroupRatioClearsRPMWhenGroupNameEmpty(t *testing.T) {
	setupUpstreamGroupRatioSyncTestDB(t)

	origSecret := common.UpstreamSecretKey
	common.UpstreamSecretKey = "test-upstream-secret"
	t.Cleanup(func() {
		common.UpstreamSecretKey = origSecret
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing", "/api/user/login":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		case "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"sub2api-token","token_type":"Bearer"}}`))
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") != "Bearer sub2api-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"id":27,"name":"Claude混池","platform":"anthropic","rate_multiplier":0.5,"rpm_limit":12,"allow_image_generation":false,"image_rate_independent":false,"image_rate_multiplier":1,"image_price_1k":null,"image_price_2k":null,"image_price_4k":null,"status":"active"}]}`))
		case "/api/status":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	passwordEnc, err := EncryptUpstreamPassword("secret")
	require.NoError(t, err)

	channel := &model.Channel{
		Id:            503,
		Name:          "sync-channel-clear-rpm",
		Key:           "sk-test",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"upstream_rpm_limit":99}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	profile := &model.ChannelUpstreamProfile{
		ChannelId:           channel.Id,
		KeyFingerprint:      "test-fingerprint-clear-rpm",
		KeyMasked:           "sk-t...test",
		UpstreamAccount:     "upstream@example.com",
		UpstreamPasswordEnc: passwordEnc,
		UpstreamLoginUrl:    server.URL,
		UpstreamGroup:       "",
		UpstreamTopupRatio:  1,
		CreatedAt:           common.GetTimestamp(),
		UpdatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(profile).Error)

	require.NoError(t, syncChannelUpstreamGroupRatio(context.Background(), profile))

	savedChannel, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, 0, savedChannel.GetOtherSettings().UpstreamRPMLimit)
}

func TestSyncChannelUpstreamGroupRatioKeepsExistingTopupRatio(t *testing.T) {
	setupUpstreamGroupRatioSyncTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config":
			_, _ = w.Write([]byte(`{"success":true,"data":{"group_ratio":{"default":0.5}}}`))
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"topup_ratio":9}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		}
	}))
	defer server.Close()

	profile := &model.ChannelUpstreamProfile{
		ChannelId:          504,
		KeyFingerprint:     "test-fingerprint-keep-topup",
		KeyMasked:          "sk-t...test",
		UpstreamLoginUrl:   server.URL,
		UpstreamGroup:      "default",
		UpstreamTopupRatio: 2,
		CreatedAt:          common.GetTimestamp(),
		UpdatedAt:          common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(profile).Error)

	require.NoError(t, syncChannelUpstreamGroupRatio(context.Background(), profile))

	var savedProfile model.ChannelUpstreamProfile
	require.NoError(t, model.DB.First(&savedProfile, profile.Id).Error)
	require.Equal(t, 0.5, savedProfile.UpstreamGroupRatio)
	require.Equal(t, 2.0, savedProfile.UpstreamTopupRatio)
}

func TestSyncAllChannelUpstreamGroupRatiosStoresLatestTaskPerProfile(t *testing.T) {
	setupUpstreamGroupRatioSyncTestDB(t)

	origSecret := common.UpstreamSecretKey
	common.UpstreamSecretKey = "test-upstream-secret"
	t.Cleanup(func() {
		common.UpstreamSecretKey = origSecret
	})

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config":
			_, _ = w.Write([]byte(`{"success":true,"data":{"group_ratio":{"vip":0.6}}}`))
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"topup_ratio":2}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		}
	}))
	defer successServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body>login</body></html>`))
	}))
	defer failServer.Close()

	successChannel := &model.Channel{
		Id:            601,
		Name:          "sync-success",
		Key:           "sk-success",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"upstream_rpm_limit":7}`,
	}
	require.NoError(t, model.DB.Create(successChannel).Error)
	failChannel := &model.Channel{
		Id:            602,
		Name:          "sync-fail",
		Key:           "sk-fail",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"upstream_rpm_limit":8}`,
	}
	require.NoError(t, model.DB.Create(failChannel).Error)

	successProfile := &model.ChannelUpstreamProfile{
		ChannelId:        successChannel.Id,
		KeyFingerprint:   "success-profile",
		KeyMasked:        "sk-s...cess",
		UpstreamLoginUrl: successServer.URL,
		UpstreamGroup:    "vip",
		CreatedAt:        common.GetTimestamp(),
		UpdatedAt:        common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(successProfile).Error)

	failProfile := &model.ChannelUpstreamProfile{
		ChannelId:        failChannel.Id,
		KeyFingerprint:   "fail-profile",
		KeyMasked:        "sk-f...ail",
		UpstreamLoginUrl: failServer.URL,
		UpstreamGroup:    "vip",
		CreatedAt:        common.GetTimestamp(),
		UpdatedAt:        common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(failProfile).Error)

	synced, failed, err := SyncAllChannelUpstreamGroupRatios(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, synced)
	assert.Equal(t, 1, failed)

	var tasks []model.Task
	require.NoError(t, model.DB.Order("channel_id asc").Find(&tasks).Error)
	require.Len(t, tasks, 2)

	assert.Equal(t, successChannel.Id, tasks[0].ChannelId)
	assert.Equal(t, string(model.TaskStatusSuccess), string(tasks[0].Status))
	assert.Empty(t, tasks[0].FailReason)
	var successData map[string]any
	require.NoError(t, tasks[0].GetData(&successData))
	assert.Equal(t, successServer.URL, tasks[0].Properties.Input)
	assert.Equal(t, "vip", successData["upstream_group"])
	assert.Equal(t, 0.6, successData["group_ratio"])
	assert.Equal(t, successServer.URL+"/api/ratio_config", successData["source"])

	assert.Equal(t, failChannel.Id, tasks[1].ChannelId)
	assert.Equal(t, string(model.TaskStatusFailure), string(tasks[1].Status))
	assert.Contains(t, tasks[1].FailReason, "获取分组倍率失败")
	var failData map[string]any
	require.NoError(t, tasks[1].GetData(&failData))
	assert.Equal(t, failServer.URL, tasks[1].Properties.Input)
	assert.Equal(t, "vip", failData["upstream_group"])
	assert.Contains(t, failData["error"], "invalid character '<'")
}
