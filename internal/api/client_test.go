package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jhuiting/chargebee-cli/internal/api"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		user, _, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test_key", user)
		assert.Equal(t, "5", r.URL.Query().Get("limit"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"list":[]}`))
	}))
	defer srv.Close()

	client := api.NewClient("test-site", "test_key")
	client.BaseURL = srv.URL

	params := url.Values{}
	params.Set("limit", "5")

	result, err := client.Get(context.Background(), "/customers", params)
	require.NoError(t, err)

	var resp struct {
		List []json.RawMessage `json:"list"`
	}
	require.NoError(t, result.Decode(&resp))
	assert.Empty(t, resp.List)
}

func TestAPIErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error_code":"api_authentication_failed","error_msg":"API key is invalid"}`))
	}))
	defer srv.Close()

	client := api.NewClient("test-site", "bad_key")
	client.BaseURL = srv.URL

	_, err := client.Get(context.Background(), "/customers", nil)
	require.Error(t, err)
	assert.NotEmpty(t, err.Error())
}

func TestRateLimitRetry(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error_code":"too_many_requests","error_msg":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"list":[]}`))
	}))
	defer srv.Close()

	client := api.NewClient("test-site", "test_key")
	client.BaseURL = srv.URL

	result, err := client.Get(context.Background(), "/events", nil)
	require.NoError(t, err)

	var resp struct {
		List []json.RawMessage `json:"list"`
	}
	require.NoError(t, result.Decode(&resp))
	assert.Equal(t, int32(3), attempts.Load())
}
