package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/output"
)

// newLoginCmd creates the login command that authenticates with a Chargebee site.
func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with your Chargebee site",
		Long: `Log in to your Chargebee site by providing your site name and API key.
Credentials are validated against the Chargebee API and stored locally.

Interactive mode (default):
  cb login

Non-interactive mode (for CI/CD):
  cb login --site mysite --api-key live_xxx`,
		RunE: runLogin,
	}
	return cmd
}

func runLogin(cmd *cobra.Command, _ []string) error {
	out := output.Default
	site, _ := cmd.Flags().GetString("site")
	apiKey, _ := cmd.Flags().GetString("api-key")
	profileName, _ := cmd.Flags().GetString("profile")
	explicitProfile := profileName != ""

	scanner := bufio.NewScanner(os.Stdin)

	if site == "" {
		out.Prompt("Enter your Chargebee site name: ")
		if scanner.Scan() {
			site = strings.TrimSpace(scanner.Text())
		}
		if site == "" {
			return fmt.Errorf("site name is required")
		}
	}

	if apiKey == "" {
		out.Prompt("Enter your API key: ")
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
		}
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}
	}

	// Prompt for profile name if not provided via --profile flag.
	if !explicitProfile {
		out.Prompt(fmt.Sprintf("Profile name [%s]: ", site))
		if scanner.Scan() {
			if input := strings.TrimSpace(scanner.Text()); input != "" {
				profileName = input
			}
		}
		if profileName == "" {
			profileName = site
		}
	}

	out.Status("Validating credentials for site %q...", site)

	if err := validateCredentials(site, apiKey); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cfg.SetProfile(profileName, &config.Profile{
		Site:   site,
		APIKey: apiKey,
	})
	cfg.DefaultProfile = profileName

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	out.Success("Logged in to site %q (profile: %s)", site, profileName)
	return nil
}

func validateCredentials(site, apiKey string) error {
	client := api.NewClient(site, apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	params := url.Values{}
	params.Set("limit", "1")
	_, err := client.Get(ctx, "/customers", params)
	return err
}
