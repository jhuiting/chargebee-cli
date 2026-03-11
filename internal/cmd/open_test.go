package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvePageURL_StaticPages(t *testing.T) {
	tests := []struct {
		page string
		want string
	}{
		{"docs", "https://www.chargebee.com/docs/2.0/"},
		{"api", "https://apidocs.chargebee.com/docs/api"},
	}

	for _, tt := range tests {
		t.Run(tt.page, func(t *testing.T) {
			got, err := resolvePageURLForTest(tt.page)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePageURL_UnknownPage(t *testing.T) {
	_, err := resolvePageURLForTest("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown page")
}
