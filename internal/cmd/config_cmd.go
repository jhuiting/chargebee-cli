package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/output"
)

// newConfigCmd creates the config command with get, set, and list subcommands.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify CLI configuration",
		Long: `Manage CLI configuration settings.

  cb config get site
  cb config set region us
  cb config list`,
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long:  `Get the value of a configuration key. Supported keys: site, api_key, region, default_profile.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  `Set a configuration value. Supported keys: site, api_key, region, default_profile.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles and their settings",
		RunE:  runConfigList,
	}
}

var validConfigKeys = map[string]bool{
	"site":            true,
	"api_key":         true,
	"region":          true,
	"default_profile": true,
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	out := output.Default
	key := args[0]
	if !validConfigKeys[key] {
		return fmt.Errorf("unknown config key %q (valid keys: site, api_key, region, default_profile)", key)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if key == "default_profile" {
		out.Data("%s", cfg.DefaultProfile)
		return nil
	}

	profileName, _ := cmd.Flags().GetString("profile")
	profile, err := resolveProfile(cfg, profileName)
	if err != nil {
		return err
	}

	switch key {
	case "site":
		out.Data("%s", profile.Site)
	case "api_key":
		out.Data("%s", profile.APIKey)
	case "region":
		out.Data("%s", profile.Region)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	out := output.Default
	key, value := args[0], args[1]
	if !validConfigKeys[key] {
		return fmt.Errorf("unknown config key %q (valid keys: site, api_key, region, default_profile)", key)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if key == "default_profile" {
		if _, ok := cfg.Profiles[value]; !ok {
			return fmt.Errorf("profile %q does not exist", value)
		}
		cfg.DefaultProfile = value
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		out.Success("default_profile set to %q", value)
		return nil
	}

	profileName, _ := cmd.Flags().GetString("profile")
	profile, err := resolveProfile(cfg, profileName)
	if err != nil {
		return err
	}

	switch key {
	case "site":
		profile.Site = value
	case "api_key":
		profile.APIKey = value
	case "region":
		profile.Region = value
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	out.Success("%s set to %q", key, value)
	return nil
}

func runConfigList(_ *cobra.Command, _ []string) error {
	out := output.Default
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	out.KeyValue("default_profile", cfg.DefaultProfile)
	_, _ = fmt.Fprintln(output.Default.Stderr())

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := cfg.Profiles[name]
		out.KeyValue(fmt.Sprintf("[%s] site", name), p.Site)
		out.KeyValue(fmt.Sprintf("[%s] api_key", name), maskKey(p.APIKey))
		if p.Region != "" {
			out.KeyValue(fmt.Sprintf("[%s] region", name), p.Region)
		}
	}

	if len(cfg.Profiles) == 0 {
		out.Dim("No profiles configured. Run 'cb login' to get started.")
	}

	return nil
}

func resolveProfile(cfg *config.Config, profileName string) (*config.Profile, error) {
	if profileName == "" {
		profileName = cfg.DefaultProfile
		if profileName == "" {
			profileName = "default"
		}
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", profileName)
	}
	return profile, nil
}
