package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/output"
	"github.com/jhuiting/chargebee-cli/internal/update"
	"github.com/jhuiting/chargebee-cli/internal/version"
)

// NewRootCmd creates the root cobra command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cb",
		Short: "Explore and manage your Chargebee integration",
		Long: `Explore and manage your Chargebee integration from the terminal.

To get started, run:
  cb login`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("profile", "", "named profile to use (overrides CB_PROFILE)")
	root.PersistentFlags().String("site", "", "Chargebee site name (overrides config)")
	root.PersistentFlags().String("api-key", "", "API key (overrides config)")

	root.AddGroup(
		&cobra.Group{ID: "auth", Title: "Auth & Config:"},
		&cobra.Group{ID: "data", Title: "Data:"},
	)

	// Auth & config commands
	addCmds(root, "auth",
		newLoginCmd(),
		newLogoutCmd(),
		newSwitchCmd(),
		newConfigCmd(),
		newStatusCmd(),
	)

	// Data commands
	dataCmds := []*cobra.Command{
		newGetCmd(),
		newListenCmd(),
		newUsageCmd(),
		newEntitlementsCmd(),
	}
	for _, def := range allResources() {
		dataCmds = append(dataCmds, buildResourceCmd(def, resolveAPIClient))
	}
	addCmds(root, "data", dataCmds...)

	// Ungrouped utilities
	root.AddCommand(newOpenCmd())
	root.AddCommand(newCompletionCmd())
	root.AddCommand(newVersionCmd())

	return root
}

func addCmds(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		parent.AddCommand(c)
	}
}

// Execute runs the root command.
func Execute() {
	root := NewRootCmd()

	updateCh := make(chan *update.Info, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		updateCh <- update.CheckForUpdate(ctx, version.Version)
	}()

	if err := root.Execute(); err != nil {
		output.Default.Error("%s", err)
		os.Exit(1)
	}

	select {
	case info := <-updateCh:
		if info != nil && info.UpdateAvailable && !info.NotifiedRecently {
			_, _ = fmt.Fprintln(os.Stderr)
			output.Default.Warning("A new version of cb is available: %s → %s", info.CurrentVersion, info.LatestVersion)
			output.Default.Dim("Update with: brew upgrade chargebee-cli")
			update.MarkNotified()
		}
	default:
	}
}
