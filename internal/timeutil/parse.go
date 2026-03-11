package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTimestamp parses a string into a Unix timestamp. It accepts:
//   - Unix int64 ("1700000000") — pass-through
//   - RFC3339 ("2024-01-01T15:04:05Z")
//   - ISO datetime ("2024-01-01T15:04:05")
//   - ISO date ("2024-01-01") — midnight UTC
//   - Relative durations: Nd, Nh, Nm (e.g. "7d" = 7 days ago from now)
func ParseTimestamp(s string, now time.Time) (int64, error) {
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return unix, nil
	}

	if d, err := parseRelative(s, now); err == nil {
		return d, nil
	}

	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix(), nil
		}
	}

	return 0, fmt.Errorf("cannot parse %q: expected unix timestamp, ISO date, RFC3339, or relative duration (e.g. 7d, 24h, 30m)", s)
}

func parseRelative(s string, now time.Time) (int64, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}

	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid relative duration %q", s)
	}

	switch unit {
	case 'd':
		return now.Add(-time.Duration(n) * 24 * time.Hour).Unix(), nil
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour).Unix(), nil
	case 'm':
		return now.Add(-time.Duration(n) * time.Minute).Unix(), nil
	default:
		return 0, fmt.Errorf("unknown unit %q in %q", string(unit), s)
	}
}

// timestampFields is the set of Chargebee API fields that contain Unix timestamps.
var timestampFields = map[string]bool{
	"created_at":         true,
	"updated_at":         true,
	"occurred_at":        true,
	"activated_at":       true,
	"started_at":         true,
	"cancelled_at":       true,
	"current_term_start": true,
	"current_term_end":   true,
	"trial_start":        true,
	"trial_end":          true,
	"next_billing_at":    true,
	"usage_date":         true,
	"voided_at":          true,
	"paid_at":            true,
	"due_date":           true,
	"dunning_start":      true,
}

// IsTimestampKey checks if a -d key refers to a known Chargebee timestamp field.
// It extracts the field name before any "[" bracket and checks against the allowlist.
func IsTimestampKey(key string) bool {
	field, _, _ := strings.Cut(key, "[")
	return timestampFields[field]
}

// ConvertIfTimestamp converts a -d value to a Unix timestamp string if the key
// refers to a known timestamp field and the value is not already numeric.
// The [between] operator is skipped (pass-through) since it requires two
// comma-separated timestamps. Non-timestamp keys pass through unchanged.
func ConvertIfTimestamp(key, value string, now time.Time) (string, error) {
	if !IsTimestampKey(key) {
		return value, nil
	}

	if strings.Contains(key, "[between]") {
		return value, nil
	}

	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value, nil
	}

	ts, err := ParseTimestamp(value, now)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp for %q: %w", key, err)
	}
	return strconv.FormatInt(ts, 10), nil
}
