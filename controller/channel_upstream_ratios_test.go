package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

	ratios, raw, source, err := fetchUpstreamGroupRatios(
		context.Background(),
		server.Client(),
		server.URL,
		&upstreamPricingCredential{Account: "upstream", Password: "secret"},
	)

	require.NoError(t, err)
	require.Equal(t, 0.2, ratios["default"])
	require.Equal(t, 0.1, ratios["vip"])
	require.Contains(t, raw, "default")
	require.Equal(t, server.URL+"/api/pricing", source)
	require.Equal(t, 2, pricingRequests)
}
