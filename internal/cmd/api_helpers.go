package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/timeutil"
)

// parseDataFlags parses -d key=value flags into url.Values.
// Timestamp fields (created_at, updated_at, etc.) are automatically converted
// from human-readable dates (ISO 8601, relative durations) to Unix timestamps.
func parseDataFlags(data []string) (url.Values, error) {
	vals := url.Values{}
	now := time.Now()
	for _, d := range data {
		key, value, ok := strings.Cut(d, "=")
		if !ok {
			return nil, fmt.Errorf("invalid data flag %q: expected key=value format", d)
		}
		converted, err := timeutil.ConvertIfTimestamp(key, value, now)
		if err != nil {
			return nil, err
		}
		vals.Add(key, converted)
	}
	return vals, nil
}

// printJSON writes JSON to the writer, pretty-printed unless raw is true.
func printJSON(w io.Writer, data []byte, raw bool) error {
	if raw {
		_, err := fmt.Fprintln(w, string(data))
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		_, err2 := fmt.Fprintln(w, string(data))
		return err2
	}
	_, err := fmt.Fprintln(w, buf.String())
	return err
}

// printResponseHeaders writes HTTP response headers to the writer.
func printResponseHeaders(w io.Writer, result *api.Result) {
	_, _ = fmt.Fprintf(w, "%s\n", result.Status)
	keys := make([]string, 0, len(result.Headers))
	for k := range result.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range result.Headers[k] {
			_, _ = fmt.Fprintf(w, "%s: %s\n", k, v)
		}
	}
	_, _ = fmt.Fprintln(w)
}

// resolveCredentials resolves site and API key from flags, env vars, or config.
// The --profile flag selects a specific profile; otherwise the active profile is used.
func resolveCredentials(cmd *cobra.Command) (site, apiKey string, err error) {
	site, _ = cmd.Flags().GetString("site")
	apiKey, _ = cmd.Flags().GetString("api-key")
	if site != "" && apiKey != "" {
		return site, apiKey, nil
	}

	// Check env vars first.
	if envSite := os.Getenv("CB_SITE"); envSite != "" && site == "" {
		site = envSite
	}
	if envKey := os.Getenv("CB_API_KEY"); envKey != "" && apiKey == "" {
		apiKey = envKey
	}
	if site != "" && apiKey != "" {
		return site, apiKey, nil
	}

	// Fall back to config profile.
	cfg, err := config.Load()
	if err != nil {
		return "", "", err
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = os.Getenv("CB_PROFILE")
	}

	var profile *config.Profile
	if profileName != "" {
		p, ok := cfg.Profiles[profileName]
		if !ok {
			return "", "", fmt.Errorf("profile %q not found, run 'cb login' first", profileName)
		}
		profile = p
	} else {
		p, err := cfg.ActiveProfile()
		if err != nil {
			return "", "", fmt.Errorf("not logged in, run 'cb login' first")
		}
		profile = p
	}

	if site == "" {
		site = profile.Site
	}
	if apiKey == "" {
		apiKey = profile.APIKey
	}
	return site, apiKey, nil
}

// resolveAPIClient creates an API client from flags, env vars, or config.
func resolveAPIClient(cmd *cobra.Command) (*api.Client, error) {
	site, apiKey, err := resolveCredentials(cmd)
	if err != nil {
		return nil, err
	}
	return api.NewClient(site, apiKey), nil
}
