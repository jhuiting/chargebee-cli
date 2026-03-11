package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("cb version %s (%s) built at %s\n", version.Version, version.Commit, version.Date)
		},
	}
}
