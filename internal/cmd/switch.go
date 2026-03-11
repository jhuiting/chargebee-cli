package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/config"
	"github.com/jhuiting/chargebee-cli/internal/output"
)

// newSwitchCmd creates the switch command for changing the active profile.
func newSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch [profile]",
		Short: "Switch the active profile",
		Long: `Switch the active Chargebee profile. With an argument, sets the default
profile. Without arguments, lists all profiles and highlights the active one.

Examples:
  cb switch staging
  cb switch`,
		Args: cobra.MaximumNArgs(1),
		RunE: runSwitch,
	}
}

func runSwitch(_ *cobra.Command, args []string) error {
	out := output.Default

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(args) == 1 {
		return switchTo(cfg, out, args[0])
	}

	return listProfiles(cfg, out)
}

func switchTo(cfg *config.Config, out *output.Printer, name string) error {
	resolved, err := resolveProfileName(cfg, name)
	if err != nil {
		return err
	}

	cfg.DefaultProfile = resolved
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	site := cfg.Profiles[resolved].Site
	out.Success("Switched to profile %q (site: %s)", resolved, site)
	return nil
}

// resolveProfileName finds a profile by exact name or unique prefix.
func resolveProfileName(cfg *config.Config, name string) (string, error) {
	if _, ok := cfg.Profiles[name]; ok {
		return name, nil
	}

	var matches []string
	for pName := range cfg.Profiles {
		if strings.HasPrefix(pName, name) {
			matches = append(matches, pName)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("profile %q does not exist (run 'cb switch' to see available profiles)", name)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("ambiguous profile %q — matches: %s", name, strings.Join(matches, ", "))
	}
}

func listProfiles(cfg *config.Config, out *output.Printer) error {
	if len(cfg.Profiles) == 0 {
		out.Dim("No profiles configured. Run 'cb login' to get started.")
		return nil
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := cfg.Profiles[name]
		marker := "  "
		if name == cfg.DefaultProfile {
			marker = "* "
		}
		out.Data("%s%s (site: %s)", marker, name, p.Site)
	}

	return nil
}
