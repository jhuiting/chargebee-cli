package timeutil_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jhuiting/chargebee-cli/internal/timeutil"
)

var fixedNow = time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"unix timestamp", "1700000000", 1700000000, false},
		{"rfc3339", "2024-01-01T15:04:05Z", 1704121445, false},
		{"iso datetime", "2024-01-01T15:04:05", 1704121445, false},
		{"iso date", "2024-01-01", 1704067200, false},
		{"relative 7d", "7d", fixedNow.Add(-7 * 24 * time.Hour).Unix(), false},
		{"relative 24h", "24h", fixedNow.Add(-24 * time.Hour).Unix(), false},
		{"relative 30m", "30m", fixedNow.Add(-30 * time.Minute).Unix(), false},
		{"relative 1d", "1d", fixedNow.Add(-24 * time.Hour).Unix(), false},
		{"invalid string", "not-a-date", 0, true},
		{"empty string", "", 0, true},
		{"negative relative", "-7d", 0, true},
		{"zero relative", "0d", 0, true},
		{"unknown unit", "7x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeutil.ParseTimestamp(tt.input, fixedNow)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTimestampKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"created_at[after]", true},
		{"created_at[before]", true},
		{"created_at[between]", true},
		{"updated_at[after]", true},
		{"occurred_at[after]", true},
		{"trial_start[after]", true},
		{"next_billing_at[before]", true},
		{"created_at", true},
		{"status[is]", false},
		{"email[starts_with]", false},
		{"name", false},
		{"limit", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, timeutil.IsTimestampKey(tt.key))
		})
	}
}

func TestConvertIfTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:  "timestamp key with date",
			key:   "created_at[after]",
			value: "2024-01-01",
			want:  "1704067200",
		},
		{
			name:  "timestamp key with unix",
			key:   "created_at[after]",
			value: "1700000000",
			want:  "1700000000",
		},
		{
			name:  "timestamp key with relative",
			key:   "updated_at[after]",
			value: "7d",
			want:  fmt.Sprintf("%d", fixedNow.Add(-7*24*time.Hour).Unix()),
		},
		{
			name:  "between operator passes through",
			key:   "created_at[between]",
			value: "1700000000,1710000000",
			want:  "1700000000,1710000000",
		},
		{
			name:  "non-timestamp key passes through",
			key:   "status[is]",
			value: "active",
			want:  "active",
		},
		{
			name:    "timestamp key with invalid value",
			key:     "created_at[after]",
			value:   "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeutil.ConvertIfTimestamp(tt.key, tt.value, fixedNow)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
