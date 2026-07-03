package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFetchUpstreamGroupRatiosFallsBackToPasswordLogin(t *testing.T) {
	var pricingRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"success":false,"message":"disabled"}`))
		case "/api/pricing":
			pricingRequests++
			if r.Header.Get("New-Api-User") != "7" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"message":"unauthorized"}`))
				return
			}
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "ok" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"message":"no session"}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"group_ratio":{"default":0.2,"vip":0.1}}}`))
		case "/api/user/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"username":"upstream"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := service.FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&service.UpstreamPricingCredential{Account: "upstream", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 0.2, result.Ratios["default"])
	require.Equal(t, 0.1, result.Ratios["vip"])
	require.Contains(t, result.Raw, "default")
	require.Equal(t, server.URL+"/api/pricing", result.Source)
	require.Equal(t, 1, pricingRequests)
}

func TestFetchUpstreamGroupRatiosFallsBackToSub2APILogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
		case "/api/user/login":
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
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := service.FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&service.UpstreamPricingCredential{Account: "upstream@example.com", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 0.5, result.Ratios["Claude混池"])
	require.Contains(t, result.Raw, `"rpm_limit": 12`)
	require.Contains(t, result.Raw, `"image_rate_multiplier": 1`)
	require.Equal(t, server.URL+"/api/v1/groups/available", result.Source)
}

// TestFetchUpstreamGroupRatiosSub2APITokenDirectConnect 验证 account-only token 模式（Bug3 修复）
func TestFetchUpstreamGroupRatiosSub2APITokenDirectConnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false}`))
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") != "Bearer my-api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":[{"name":"default","rate_multiplier":1.0,"platform":"openai","status":"active"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// password 为空，account 作为 Bearer token 直连
	result, err := service.FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&service.UpstreamPricingCredential{Account: "my-api-key", Password: ""},
	)

	require.NoError(t, err)
	require.Equal(t, 1.0, result.Ratios["default"])
}

func TestFetchChannelUpstreamGroupRatiosUsesIdentitySessionAuth(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelUpstreamProfile{}, &model.UpstreamIdentity{}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "refresh-token", payload["refresh_token"])
			_, _ = w.Write([]byte(`{"data":{"access_token":"fresh-access-token","refresh_token":"refresh-token","expires_in":3600}}`))
		case "/api/v1/groups/available":
			require.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"data":[{"name":"default","rate_multiplier":0.75,"platform":"openai","status":"active"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	now := common.GetTimestamp()
	channel := &model.Channel{
		Name:        "xingyun",
		Type:        1,
		Status:      common.ChannelStatusEnabled,
		BaseURL:     &server.URL,
		CreatedTime: now,
	}
	require.NoError(t, db.Create(channel).Error)

	refreshEnc, err := service.EncryptUpstreamPassword("refresh-token")
	require.NoError(t, err)

	identity := &model.UpstreamIdentity{
		IdentityFingerprint: model.UpstreamIdentityFingerprint(server.URL, "user@example.com"),
		Account:             "user@example.com",
		BaseURL:             server.URL,
		AuthType:            model.AuthTypeSub2APIRefreshToken,
		RefreshTokenEnc:     refreshEnc,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require.NoError(t, db.Create(identity).Error)

	profile := &model.ChannelUpstreamProfile{
		ChannelId:          channel.Id,
		UpstreamAccount:    "user@example.com",
		UpstreamLoginUrl:   server.URL,
		UpstreamIdentityId: &identity.Id,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	require.NoError(t, db.Create(profile).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channel.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/"+strconv.Itoa(channel.Id)+"/upstream_group_ratios", nil)

	FetchChannelUpstreamGroupRatios(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			GroupRatios map[string]float64 `json:"group_ratios"`
			Source      string             `json:"source"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 0.75, response.Data.GroupRatios["default"])
	require.Equal(t, server.URL+"/api/v1/groups/available", response.Data.Source)
}
