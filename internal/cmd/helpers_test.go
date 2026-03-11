package cmd_test

import (
	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/cmd"
)

// newTestRootCmd creates a minimal root command wired to a test HTTP server.
func newTestRootCmd(baseURL, site, apiKey string) *cobra.Command {
	root := &cobra.Command{
		Use:           "cb",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("site", site, "")
	root.PersistentFlags().String("api-key", apiKey, "")
	root.PersistentFlags().String("profile", "", "")

	clientFactory := func(_ *cobra.Command) (*api.Client, error) {
		c := api.NewClient(site, apiKey)
		c.BaseURL = baseURL + "/api/v2"
		return c, nil
	}
	root.AddCommand(cmd.NewGetCmdWithClient(clientFactory))
	root.AddCommand(cmd.NewUsageCmdWithClient(clientFactory))

	return root
}

// resolvePageURLForTest wraps the exported ResolvePageURL for test access.
func resolvePageURLForTest(page string) (string, error) {
	return cmd.ResolvePageURL(page, "")
}
