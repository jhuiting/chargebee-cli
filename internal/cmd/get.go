package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
)

// ClientFactory creates an API client from a cobra command's flags/config.
type ClientFactory func(cmd *cobra.Command) (*api.Client, error)

func newGetCmd() *cobra.Command {
	return NewGetCmdWithClient(resolveAPIClient)
}

// NewGetCmdWithClient creates the get command with a custom client factory (for testing).
func NewGetCmdWithClient(factory ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Make a GET request to the Chargebee API",
		Long: `Send a GET request to the Chargebee API and print the JSON response.

Examples:
  cb get /customers/cust_123
  cb get /customers -l 10
  cb get /customers -d "status[is]=active"
  cb get /customers -d "company[starts_with]=Acme"
  cb get /customers -d "email[is]=john@example.com"
  cb get /subscriptions -d "status[is]=active" -s
  cb get /events -d "event_type[is]=customer_created" --raw`,
		Args: cobra.ExactArgs(1),
		RunE: makeRunGet(factory),
	}

	cmd.Flags().StringSliceP("data", "d", nil, "query parameters as key=value pairs")
	cmd.Flags().IntP("limit", "l", 0, "shorthand for -d limit=N")
	cmd.Flags().BoolP("show-headers", "s", false, "display HTTP response headers")
	cmd.Flags().Bool("raw", false, "print compact JSON (no pretty-printing)")

	return cmd
}

func makeRunGet(factory ClientFactory) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		path := args[0]
		dataFlags, _ := cmd.Flags().GetStringSlice("data")
		limit, _ := cmd.Flags().GetInt("limit")
		showHeaders, _ := cmd.Flags().GetBool("show-headers")
		raw, _ := cmd.Flags().GetBool("raw")

		client, err := factory(cmd)
		if err != nil {
			return err
		}

		params, err := parseDataFlags(dataFlags)
		if err != nil {
			return err
		}

		if limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", limit))
		}

		result, err := client.Get(cmd.Context(), path, params)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}

		if showHeaders {
			printResponseHeaders(os.Stderr, result)
		}

		return printJSON(os.Stdout, result.JSON(), raw)
	}
}
