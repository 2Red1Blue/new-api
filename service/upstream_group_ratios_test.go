package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseSub2APIGroupSnapshotHasRPMLimitPresence 验证 Bug1 修复：
// HasRPMLimit 仅在 JSON 明确包含 rpm_limit 字段时为 true，缺字段时为 false，字段值为 0 时为 true。
func TestParseSub2APIGroupSnapshotHasRPMLimitPresence(t *testing.T) {
	t.Run("rpm_limit 字段存在且非零", func(t *testing.T) {
		payload := map[string]any{
			"data": []any{
				map[string]any{
					"name":            "vip",
					"rate_multiplier": 1.0,
					"rpm_limit":       float64(60),
					"platform":        "openai",
					"status":          "active",
				},
			},
		}
		_, details, err := parseSub2APIGroupSnapshot(payload)
		require.NoError(t, err)
		entry := details["vip"]
		require.True(t, entry.HasRPMLimit, "rpm_limit 字段存在时 HasRPMLimit 应为 true")
		require.Equal(t, 60, entry.RPMLimit)
	})

	t.Run("rpm_limit 字段存在且为 0", func(t *testing.T) {
		payload := map[string]any{
			"data": []any{
				map[string]any{
					"name":            "free",
					"rate_multiplier": 0.5,
					"rpm_limit":       float64(0),
					"platform":        "openai",
					"status":          "active",
				},
			},
		}
		_, details, err := parseSub2APIGroupSnapshot(payload)
		require.NoError(t, err)
		entry := details["free"]
		require.True(t, entry.HasRPMLimit, "rpm_limit=0 时字段仍存在，HasRPMLimit 应为 true")
		require.Equal(t, 0, entry.RPMLimit)
	})

	t.Run("rpm_limit 字段缺失", func(t *testing.T) {
		payload := map[string]any{
			"data": []any{
				map[string]any{
					"name":            "default",
					"rate_multiplier": 1.0,
					"platform":        "openai",
					"status":          "active",
					// 不包含 rpm_limit
				},
			},
		}
		_, details, err := parseSub2APIGroupSnapshot(payload)
		require.NoError(t, err)
		entry := details["default"]
		require.False(t, entry.HasRPMLimit, "rpm_limit 字段缺失时 HasRPMLimit 应为 false，不应覆盖手动配置")
		require.Equal(t, 0, entry.RPMLimit)
	})
}

// TestFetchUpstreamGroupRatiosAccountOnlyToken 验证 Bug3 修复：account-only token 直连模式
func TestFetchUpstreamGroupRatiosAccountOnlyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratio_config", "/api/pricing":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"success":false,"message":"forbidden"}`))
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") != "Bearer sk-mytoken" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":[
				{"name":"pro","rate_multiplier":2.0,"rpm_limit":100,"platform":"openai","status":"active"},
				{"name":"default","rate_multiplier":1.0,"platform":"openai","status":"active"}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&UpstreamPricingCredential{Account: "sk-mytoken", Password: ""},
	)

	require.NoError(t, err)
	require.Equal(t, 2.0, result.Ratios["pro"])
	require.Equal(t, 1.0, result.Ratios["default"])

	pro := result.Details["pro"]
	require.True(t, pro.HasRPMLimit)
	require.Equal(t, 100, pro.RPMLimit)

	def := result.Details["default"]
	require.False(t, def.HasRPMLimit, "rpm_limit 字段缺失时 HasRPMLimit 应为 false")
}

func TestFetchUpstreamGroupRatiosFullCredentialPrioritizesLogin(t *testing.T) {
	var anonymousRatioConfigHit int
	var loginHit int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			loginHit++
			if _, ok := r.URL.Query()["turnstile"]; !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42}}`))
		case "/api/ratio_config":
			if r.Header.Get("New-Api-User") != "42" {
				anonymousRatioConfigHit++
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(`<!doctype html><html><body>login</body></html>`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"group_ratio":{"vip":0.5}}}`))
		case "/api/pricing":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!doctype html><html><body>login</body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&UpstreamPricingCredential{Account: "user@example.com", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 0.5, result.Ratios["vip"])
	require.Equal(t, 1, loginHit)
	require.Equal(t, 0, anonymousRatioConfigHit)
}

func TestFetchUpstreamGroupRatiosUsesAuthenticatedSelfGroupsEndpoint(t *testing.T) {
	var legacyEndpointHit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			if _, ok := r.URL.Query()["turnstile"]; !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":2298,"username":"lailai"}}`))
		case "/api/user/self/groups":
			if r.Header.Get("New-Api-User") != "2298" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "ok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"codex":{"desc":"plus号池","ratio":0.12},"codex-稳":{"desc":"pro号池","ratio":0.25},"default":{"desc":"默认分组","ratio":1}}}`))
		case "/api/ratio_config", "/api/pricing":
			legacyEndpointHit = true
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&UpstreamPricingCredential{Account: "user@example.com", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 0.12, result.Ratios["codex"])
	require.Equal(t, 0.25, result.Ratios["codex-稳"])
	require.Equal(t, 1.0, result.Ratios["default"])
	require.Equal(t, server.URL+"/api/user/self/groups", result.Source)
	require.False(t, legacyEndpointHit)
}

func TestFetchUpstreamGroupRatiosFallsBackToLegacyLoginURL(t *testing.T) {
	var newLoginHit int
	var legacyLoginHit int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			if _, ok := r.URL.Query()["turnstile"]; ok {
				newLoginHit++
				w.WriteHeader(http.StatusNotFound)
				return
			}
			legacyLoginHit++
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "legacy", Path: "/"})
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":7}}`))
		case "/api/user/self/groups":
			w.WriteHeader(http.StatusNotFound)
		case "/api/ratio_config":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "legacy" || r.Header.Get("New-Api-User") != "7" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"group_ratio":{"default":1}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&UpstreamPricingCredential{Account: "user@example.com", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 1.0, result.Ratios["default"])
	require.Equal(t, 1, newLoginHit)
	require.Equal(t, 1, legacyLoginHit)
	require.Equal(t, server.URL+"/api/ratio_config", result.Source)
}

// TestFetchUpstreamGroupRatiosNoCredentialReturnsError 验证无凭据且两端均失败时返回错误
func TestFetchUpstreamGroupRatiosNoCredentialReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer server.Close()

	_, err := FetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		nil,
	)

	require.Error(t, err)
}
