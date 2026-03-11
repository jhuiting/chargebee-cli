package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCmd_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath, gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		user, _, _ := r.BasicAuth()
		assert.Equal(t, "test-key", user)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "cust_123"})
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"get", "/customers/cust_123"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/v2/customers/cust_123", gotPath)
	assert.Empty(t, gotQuery)
}

func TestGetCmd_SendsQueryParams(t *testing.T) {
	var gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("limit")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"get", "/customers", "-d", "limit=10"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "10", gotQuery)
}

func TestGetCmd_RequiresPath(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"get"})
	err := root.Execute()
	assert.Error(t, err)
}
