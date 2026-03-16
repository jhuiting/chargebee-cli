package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/output"
)

// Response types for decoding Chargebee API responses.

type customerListResponse struct {
	List []customerEntry `json:"list"`
}

type customerEntry struct {
	Customer customerData `json:"customer"`
}

type singleCustomerResponse struct {
	Customer customerData `json:"customer"`
}

type customerData struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Company   string `json:"company"`
	Email     string `json:"email"`
}

type subscriptionListResponse struct {
	List []subscriptionEntry `json:"list"`
}

type subscriptionEntry struct {
	Subscription subscriptionData `json:"subscription"`
}

type subscriptionData struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
}

type paginatedSubscriptionResponse struct {
	List       []subscriptionEntry `json:"list"`
	NextOffset string              `json:"next_offset,omitempty"`
}

type usageListResponse struct {
	List []usageEntry `json:"list"`
}

type usageEntry struct {
	Usage usageData `json:"usage"`
}

type usageData struct {
	ID              string `json:"id"`
	ItemPriceID     string `json:"item_price_id"`
	SubscriptionID  string `json:"subscription_id"`
	UsageDate       int64  `json:"usage_date"`
	Quantity        string `json:"quantity"`
	QuantityInDecml string `json:"quantity_in_decimal,omitempty"`
}

// Report types for --raw JSON output.

type usageReport struct {
	Customer      customerData         `json:"customer"`
	Subscriptions []subscriptionReport `json:"subscriptions"`
}

type subscriptionReport struct {
	Subscription subscriptionData `json:"subscription"`
	Usages       []usageData      `json:"usages"`
}

func newUsageCmd() *cobra.Command {
	return NewUsageCmdWithClient(resolveAPIClient)
}

// NewUsageCmdWithClient creates the usage command with a custom client factory (for testing).
func NewUsageCmdWithClient(factory ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage [customer_id]",
		Short: "Show usage records for a customer's subscriptions",
		Long: `Look up a customer, list their subscriptions, and show usage records.

You can find a customer by ID (positional argument), or search by company, name,
email, or any Chargebee filter using -d flags.

Examples:
  cb usage cust_ABC123
  cb usage --company "Acme"
  cb usage --name "John"
  cb usage --email "john@acme.com"
  cb usage -d "email[is]=john@example.com"
  cb usage --company "Acme" --raw`,
		Args: cobra.MaximumNArgs(1),
		RunE: makeRunUsage(factory),
	}

	cmd.Flags().String("company", "", "filter by company name (starts_with)")
	cmd.Flags().String("name", "", "filter by first name (starts_with)")
	cmd.Flags().String("email", "", "filter by email (exact match)")
	cmd.Flags().StringSliceP("data", "d", nil, "arbitrary Chargebee filter params as key=value")
	cmd.Flags().Bool("raw", false, "output raw JSON instead of formatted text")

	return cmd
}

func makeRunUsage(factory ClientFactory) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		company, _ := cmd.Flags().GetString("company")
		name, _ := cmd.Flags().GetString("name")
		email, _ := cmd.Flags().GetString("email")
		dataFlags, _ := cmd.Flags().GetStringSlice("data")
		raw, _ := cmd.Flags().GetBool("raw")

		hasID := len(args) == 1
		hasFilters := company != "" || name != "" || email != "" || len(dataFlags) > 0

		if !hasID && !hasFilters {
			return fmt.Errorf("provide a customer ID or at least one filter flag (--company, --name, --email, -d)")
		}
		if hasID && hasFilters {
			return fmt.Errorf("cannot combine a customer ID with filter flags")
		}

		client, err := factory(cmd)
		if err != nil {
			return err
		}

		out := output.Default
		ctx := cmd.Context()

		var customers []customerData

		if hasID {
			result, err := client.Get(ctx, "/customers/"+args[0], nil)
			if err != nil {
				return fmt.Errorf("fetching customer %s: %w", args[0], err)
			}
			var resp singleCustomerResponse
			if err := result.Decode(&resp); err != nil {
				return fmt.Errorf("decoding customer response: %w", err)
			}
			customers = []customerData{resp.Customer}
		} else {
			params, err := parseDataFlags(dataFlags)
			if err != nil {
				return err
			}
			if company != "" {
				params.Set("company[starts_with]", company)
			}
			if name != "" {
				params.Set("first_name[starts_with]", name)
			}
			if email != "" {
				params.Set("email[is]", email)
			}

			result, err := client.Get(ctx, "/customers", params)
			if err != nil {
				return fmt.Errorf("listing customers: %w", err)
			}
			var resp customerListResponse
			if err := result.Decode(&resp); err != nil {
				return fmt.Errorf("decoding customer list: %w", err)
			}
			for _, entry := range resp.List {
				customers = append(customers, entry.Customer)
			}
		}

		if len(customers) == 0 {
			out.Warning("No customers found")
			return nil
		}

		var reports []usageReport

		for _, cust := range customers {
			subParams := url.Values{}
			subParams.Set("customer_id[is]", cust.ID)
			subResult, err := client.Get(ctx, "/subscriptions", subParams)
			if err != nil {
				return fmt.Errorf("listing subscriptions for %s: %w", cust.ID, err)
			}
			var subResp subscriptionListResponse
			if err := subResult.Decode(&subResp); err != nil {
				return fmt.Errorf("decoding subscriptions: %w", err)
			}

			report := usageReport{Customer: cust}

			if len(subResp.List) == 0 {
				if !raw {
					out.Status("Customer: %s — %s", customerDisplayName(cust), cust.ID)
					out.Dim("  (no subscriptions)")
				}
				reports = append(reports, report)
				continue
			}

			if !raw {
				out.Status("Customer: %s — %s", customerDisplayName(cust), cust.ID)
			}

			for _, subEntry := range subResp.List {
				sub := subEntry.Subscription
				usageParams := url.Values{}
				usageParams.Set("subscription_id[is]", sub.ID)
				usageResult, err := client.Get(ctx, "/usages", usageParams)
				if err != nil {
					return fmt.Errorf("listing usages for subscription %s: %w", sub.ID, err)
				}
				var usageResp usageListResponse
				if err := usageResult.Decode(&usageResp); err != nil {
					return fmt.Errorf("decoding usages: %w", err)
				}

				var usages []usageData
				for _, u := range usageResp.List {
					usages = append(usages, u.Usage)
				}

				subReport := subscriptionReport{
					Subscription: sub,
					Usages:       usages,
				}
				report.Subscriptions = append(report.Subscriptions, subReport)

				if !raw {
					out.Data("  Subscription: %s [%s]", sub.ID, sub.Status)
					if len(usages) == 0 {
						out.Dim("    (no usage records)")
					}
					for _, u := range usages {
						date := time.Unix(u.UsageDate, 0).Format("2006-01-02")
						out.Data("    %-30s %s    %s", u.ItemPriceID, date, u.Quantity)
					}
				}
			}

			reports = append(reports, report)
		}

		if raw {
			data, err := json.Marshal(reports)
			if err != nil {
				return fmt.Errorf("marshaling report: %w", err)
			}
			return printJSON(os.Stdout, data, false)
		}

		return nil
	}
}

func customerDisplayName(c customerData) string {
	var parts []string
	fullName := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if fullName != "" {
		parts = append(parts, fullName)
	}
	if c.Company != "" {
		parts = append(parts, "("+c.Company+")")
	}
	if len(parts) == 0 {
		return c.Email
	}
	return strings.Join(parts, " ")
}
