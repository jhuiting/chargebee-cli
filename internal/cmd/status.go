package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/output"
	"github.com/jhuiting/chargebee-cli/internal/version"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current configuration and API connectivity",
		Long: `Display the current profile, site, and verify API connectivity.

Examples:
  cb status
  cb status --profile staging`,
		RunE: runStatus,
	}
}

func runStatus(cmd *cobra.Command, _ []string) error {
	out := output.Default

	out.KeyValue("Version", fmt.Sprintf("%s (%s)", version.Version, version.Commit))

	site, apiKey, err := resolveCredentials(cmd)
	if err != nil {
		out.Warning("Not logged in — run 'cb login' to get started")
		return nil
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		cfg, loadErr := config.Load()
		if loadErr == nil {
			profileName = cfg.DefaultProfile
		}
	}
	if profileName == "" {
		profileName = "(unknown)"
	}
	out.KeyValue("Profile", profileName)
	out.KeyValue("Site", site)
	out.KeyValue("API Key", maskKey(apiKey))

	out.Status("Checking API connectivity...")
	client := api.NewClient(site, apiKey)
	if err := checkConnectivity(client); err != nil {
		out.Error("API unreachable: %s", err)
	} else {
		out.Success("API connected to %s.chargebee.com", site)
	}

	if pcVersion, desc, err := detectProductCatalog(client); err == nil {
		out.KeyValue("Product Catalog", fmt.Sprintf("v%s (%s)", pcVersion, desc))
	}

	return nil
}

func checkConnectivity(client *api.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	params := url.Values{}
	params.Set("limit", "1")
	_, err := client.Get(ctx, "/events", params)
	return err
}

// detectProductCatalog calls GET /configurations to determine the site's Product Catalog version.
// Returns the version string ("1" or "2") and a human-readable description.
func detectProductCatalog(client *api.Client) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := client.Get(ctx, "/configurations", nil)
	if err != nil {
		return "", "", fmt.Errorf("fetching configurations: %w", err)
	}

	var resp struct {
		Configurations []struct {
			Configuration struct {
				ProductCatalogVersion string `json:"product_catalog_version"`
			} `json:"configuration"`
		} `json:"configurations"`
	}
	if err := json.Unmarshal(result.JSON(), &resp); err != nil {
		return "", "", fmt.Errorf("parsing configurations: %w", err)
	}

	if len(resp.Configurations) == 0 {
		return "", "", fmt.Errorf("no configuration found")
	}

	version := resp.Configurations[0].Configuration.ProductCatalogVersion
	descriptions := map[string]string{
		"v1": "plans/addons",
		"v2": "items/item prices",
	}
	desc := descriptions[version]
	if desc == "" {
		desc = "unknown"
	}

	// Strip "v" prefix if present for the version number display.
	pcNum := version
	if len(pcNum) > 0 && pcNum[0] == 'v' {
		pcNum = pcNum[1:]
	}

	return pcNum, desc, nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
