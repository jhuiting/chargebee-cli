package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/output"
)

// newLogoutCmd creates the logout command that removes stored credentials.
func newLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored Chargebee credentials",
		Long: `Remove stored credentials from the local configuration.

Remove current profile:
  cb logout

Remove a specific profile:
  cb logout --profile staging

Remove all profiles:
  cb logout --all`,
		RunE: runLogout,
	}

	cmd.Flags().Bool("all", false, "remove all stored profiles")

	return cmd
}

func runLogout(cmd *cobra.Command, _ []string) error {
	out := output.Default
	removeAll, _ := cmd.Flags().GetBool("all")
	profileName, _ := cmd.Flags().GetString("profile")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if removeAll {
		cfg.Profiles = make(map[string]*config.Profile)
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		out.Success("All profiles removed.")
		return nil
	}

	if profileName == "" {
		profileName = cfg.DefaultProfile
		if profileName == "" {
			profileName = "default"
		}
	}

	if _, ok := cfg.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	cfg.RemoveProfile(profileName)

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	out.Success("Profile %q removed.", profileName)
	return nil
}
