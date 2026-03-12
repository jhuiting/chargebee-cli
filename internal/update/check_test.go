package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "v1.0.0", "v1.0.0", 0},
		{"a newer patch", "v1.0.1", "v1.0.0", 1},
		{"b newer patch", "v1.0.0", "v1.0.1", -1},
		{"a newer minor", "v1.1.0", "v1.0.0", 1},
		{"a newer major", "v2.0.0", "v1.9.9", 1},
		{"no v prefix", "1.2.3", "1.2.3", 0},
		{"mixed prefix", "v1.2.3", "1.2.3", 0},
		{"pre-release stripped", "v1.0.0-rc1", "v1.0.0", 0},
		{"different lengths", "v1.0", "v1.0.0", 0},
		{"v1.10 vs v1.9", "v1.10.0", "v1.9.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if tt.want > 0 {
				assert.Positive(t, got)
			} else if tt.want < 0 {
				assert.Negative(t, got)
			} else {
				assert.Equal(t, 0, got)
			}
		})
	}
}

func TestCheckForUpdate_NewerAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer srv.Close()

	origURL := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = origURL }()

	t.Setenv("CB_CONFIG_DIR", t.TempDir())
	t.Setenv("CB_NO_UPDATE_CHECK", "")

	info := CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.True(t, info.UpdateAvailable)
	assert.Equal(t, "v1.0.0", info.CurrentVersion)
	assert.Equal(t, "v2.0.0", info.LatestVersion)
}

func TestCheckForUpdate_AlreadyLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()

	origURL := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = origURL }()

	t.Setenv("CB_CONFIG_DIR", t.TempDir())
	t.Setenv("CB_NO_UPDATE_CHECK", "")

	info := CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.False(t, info.UpdateAvailable)
}

func TestCheckForUpdate_SkipsDev(t *testing.T) {
	info := CheckForUpdate(context.Background(), "dev")
	assert.Nil(t, info)
}

func TestCheckForUpdate_SkipsWhenDisabled(t *testing.T) {
	t.Setenv("CB_NO_UPDATE_CHECK", "1")
	info := CheckForUpdate(context.Background(), "v1.0.0")
	assert.Nil(t, info)
}

func TestCheckForUpdate_UsesCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer srv.Close()

	origURL := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = origURL }()

	t.Setenv("CB_CONFIG_DIR", t.TempDir())
	t.Setenv("CB_NO_UPDATE_CHECK", "")

	// First call fetches from server.
	info := CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.True(t, info.UpdateAvailable)
	assert.Equal(t, 1, callCount)

	// Second call uses cache — no additional HTTP request.
	info = CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.True(t, info.UpdateAvailable)
	assert.Equal(t, 1, callCount)
}

func TestCheckForUpdate_NotifiedRecently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer srv.Close()

	origURL := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = origURL }()

	t.Setenv("CB_CONFIG_DIR", t.TempDir())
	t.Setenv("CB_NO_UPDATE_CHECK", "")

	// First check: not recently notified.
	info := CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.True(t, info.UpdateAvailable)
	assert.False(t, info.NotifiedRecently)

	// Mark as notified.
	MarkNotified()

	// Second check: recently notified.
	info = CheckForUpdate(context.Background(), "v1.0.0")
	require.NotNil(t, info)
	assert.True(t, info.UpdateAvailable)
	assert.True(t, info.NotifiedRecently)
}
