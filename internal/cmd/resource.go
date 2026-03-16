package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/output"
	"github.com/jhuiting/chargebee-cli/internal/timeutil"
)

// FilterDef defines a convenience filter flag for a resource's list command.
type FilterDef struct {
	Flag        string   // CLI flag name, e.g. "status", "email"
	Field       string   // Chargebee API field, e.g. "status", "email"
	Operator    string   // Filter operator, e.g. "is", "starts_with"
	Help        string   // Flag help text
	ValidValues []string // if non-empty, value must be one of these
}

// ResourceDef declaratively defines a Chargebee API resource and its operations.
type ResourceDef struct {
	Name           string
	Singular       string
	APIPath        string
	Operations     []OpDef
	Filters        []FilterDef
	TimestampField string // field for --after/--before, e.g. "created_at" or "occurred_at"
}

// OpDef defines a single operation on a resource.
type OpDef struct {
	Name     string
	Use      string
	Short    string
	PathFunc func(apiPath string, args []string) string
	Args     cobra.PositionalArgs
}

// ReadOps returns the read-only operations (list + retrieve) for a resource.
func ReadOps(singular string) []OpDef {
	return []OpDef{
		ListOp(singular),
		RetrieveOp(singular),
	}
}

// ListOp returns a list operation.
func ListOp(singular string) OpDef {
	return OpDef{
		Name:  "list",
		Use:   "list",
		Short: fmt.Sprintf("List %ss", singular),
		Args:  cobra.NoArgs,
		PathFunc: func(apiPath string, _ []string) string {
			return apiPath
		},
	}
}

// RetrieveOp returns a retrieve operation.
func RetrieveOp(singular string) OpDef {
	return OpDef{
		Name:  "retrieve",
		Use:   "retrieve <id>",
		Short: fmt.Sprintf("Retrieve a %s by ID", singular),
		Args:  cobra.ExactArgs(1),
		PathFunc: func(apiPath string, args []string) string {
			return fmt.Sprintf("%s/%s", apiPath, args[0])
		},
	}
}

// buildResourceCmd creates a parent command with subcommands for each operation.
func buildResourceCmd(def ResourceDef, factory ClientFactory) *cobra.Command {
	resCmd := &cobra.Command{
		Use:   def.Name,
		Short: fmt.Sprintf("List and retrieve %s", def.Name),
	}

	for _, op := range def.Operations {
		resCmd.AddCommand(buildOpCmd(def, op, factory))
	}

	return resCmd
}

// RegisterResource creates a resource command and adds it to the parent.
func RegisterResource(parent *cobra.Command, def ResourceDef, factory ClientFactory) {
	parent.AddCommand(buildResourceCmd(def, factory))
}

func buildOpCmd(res ResourceDef, op OpDef, factory ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   op.Use,
		Short: op.Short,
		Long:  opLongHelp(res, op),
		Args:  op.Args,
		RunE:  makeRunOp(res, op, factory),
	}

	cmd.Flags().StringSliceP("data", "d", nil, "query parameters as key=value pairs (e.g. -d \"status[is]=active\")")
	cmd.Flags().BoolP("show-headers", "s", false, "display HTTP response headers")
	cmd.Flags().Bool("raw", false, "print compact JSON (no pretty-printing)")

	if op.Name == "list" {
		cmd.Flags().IntP("limit", "l", 0, "maximum number of results to return (1-100, default 10)")
		cmd.Flags().String("offset", "", "pagination offset from a previous list response")
		for _, f := range res.Filters {
			cmd.Flags().String(f.Flag, "", f.Help)
		}
		if res.TimestampField != "" {
			cmd.Flags().String("after", "", "filter results after this time (e.g. 2024-01-01, 7d, 24h, 1700000000)")
			cmd.Flags().String("before", "", "filter results before this time (e.g. 2024-01-01, 7d, 24h, 1700000000)")
			cmd.Example = fmt.Sprintf("  cb %s list --after 7d\n  cb %s list --before 2024-01-01\n  cb %s list -d \"%s[after]=2024-01-01\" --raw", res.Name, res.Name, res.Name, res.TimestampField)
		}
	}

	return cmd
}

func opLongHelp(res ResourceDef, op OpDef) string {
	switch op.Name {
	case "list":
		var b strings.Builder
		fmt.Fprintf(&b, "List %s from the Chargebee API.\n", res.Name)

		if len(res.Filters) > 0 {
			b.WriteString("\nCommon filters:\n")
			for _, f := range res.Filters {
				fmt.Fprintf(&b, "  --%s <value>   %s[%s]=<value>\n", f.Flag, f.Field, f.Operator)
			}
		}

		b.WriteString("\nUse -d to pass additional query filters. Chargebee supports operators like [is], [in],\n")
		b.WriteString("[starts_with], [between], and more. See https://apidocs.chargebee.com for the\n")
		b.WriteString("full list of filterable fields.\n")

		b.WriteString("\nExamples:\n")
		if len(res.Filters) > 0 {
			fmt.Fprintf(&b, "  cb %s list --%s %s\n", res.Name, res.Filters[0].Flag, filterExampleValue(res.Filters[0]))
		}
		fmt.Fprintf(&b, "  cb %s list -l 10\n", res.Name)
		if res.TimestampField == "" {
			fmt.Fprintf(&b, "  cb %s list -d \"created_at[after]=1700000000\" --raw", res.Name)
		}

		return b.String()
	case "retrieve":
		return fmt.Sprintf(`Retrieve a single %s by ID from the Chargebee API.

Examples:
  cb %s retrieve <id>
  cb %s retrieve <id> -s
  cb %s retrieve <id> --raw`, res.Singular, res.Name, res.Name, res.Name)
	default:
		return ""
	}
}

func makeRunOp(res ResourceDef, op OpDef, factory ClientFactory) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		client, err := factory(cmd)
		if err != nil {
			return err
		}

		dataFlags, _ := cmd.Flags().GetStringSlice("data")
		showHeaders, _ := cmd.Flags().GetBool("show-headers")
		raw, _ := cmd.Flags().GetBool("raw")

		params, err := parseDataFlags(dataFlags)
		if err != nil {
			return err
		}

		if op.Name == "list" {
			if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
				if limit > 100 {
					return fmt.Errorf("--limit must be between 1 and 100 (Chargebee API maximum)")
				}
				params.Set("limit", fmt.Sprintf("%d", limit))
			}
			if offset, _ := cmd.Flags().GetString("offset"); offset != "" {
				params.Set("offset", offset)
			}
			for _, f := range res.Filters {
				if val, _ := cmd.Flags().GetString(f.Flag); val != "" {
					if len(f.ValidValues) > 0 && !slices.Contains(f.ValidValues, val) {
						return fmt.Errorf("invalid --%s value %q: must be one of: %s", f.Flag, val, strings.Join(f.ValidValues, ", "))
					}
					params.Set(fmt.Sprintf("%s[%s]", f.Field, f.Operator), val)
				}
			}
			if res.TimestampField != "" {
				now := time.Now()
				if after, _ := cmd.Flags().GetString("after"); after != "" {
					ts, err := timeutil.ParseTimestamp(after, now)
					if err != nil {
						return fmt.Errorf("invalid --after value: %w", err)
					}
					params.Set(fmt.Sprintf("%s[after]", res.TimestampField), fmt.Sprintf("%d", ts))
				}
				if before, _ := cmd.Flags().GetString("before"); before != "" {
					ts, err := timeutil.ParseTimestamp(before, now)
					if err != nil {
						return fmt.Errorf("invalid --before value: %w", err)
					}
					params.Set(fmt.Sprintf("%s[before]", res.TimestampField), fmt.Sprintf("%d", ts))
				}
			}
		}

		path := op.PathFunc(res.APIPath, args)

		var result *api.Result
		result, err = client.Get(cmd.Context(), path, params)
		if err != nil {
			return fmt.Errorf("%s %s: %w", op.Name, res.Name, err)
		}

		if showHeaders {
			printResponseHeaders(os.Stderr, result)
		}

		if op.Name == "list" && !raw {
			var envelope struct {
				List []json.RawMessage `json:"list"`
			}
			if json.Unmarshal(result.JSON(), &envelope) == nil && envelope.List != nil && len(envelope.List) == 0 {
				output.Default.Dim("No results.")
				return nil
			}
		}

		return printJSON(os.Stdout, result.JSON(), raw)
	}
}

func filterExampleValue(f FilterDef) string {
	switch f.Operator {
	case "is":
		switch f.Field {
		case "status":
			return "active"
		case "event_type":
			return "customer_created"
		default:
			return "<value>"
		}
	case "starts_with":
		return "Acme"
	default:
		return "<value>"
	}
}
