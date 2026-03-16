package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/output"
)

// Entitlement API response types.

type featureListResponse struct {
	List []featureEntry `json:"list"`
}

type featureEntry struct {
	Feature featureData `json:"feature"`
}

type featureData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Type        string `json:"type"`
}

type entitlementListResponse struct {
	List []entitlementEntry `json:"list"`
}

type entitlementEntry struct {
	Entitlement entitlementData `json:"entitlement"`
}

type entitlementData struct {
	ID          string `json:"id"`
	EntityID    string `json:"entity_id"`
	EntityType  string `json:"entity_type"`
	FeatureID   string `json:"feature_id"`
	FeatureName string `json:"feature_name"`
	Value       string `json:"value"`
	Name        string `json:"name"`
}

type subscriptionEntitlementListResponse struct {
	List []subscriptionEntitlementEntry `json:"list"`
}

type subscriptionEntitlementEntry struct {
	SubscriptionEntitlement subscriptionEntitlementData `json:"subscription_entitlement"`
}

type subscriptionEntitlementData struct {
	SubscriptionID string `json:"subscription_id"`
	FeatureID      string `json:"feature_id"`
	FeatureName    string `json:"feature_name"`
	FeatureUnit    string `json:"feature_unit"`
	FeatureType    string `json:"feature_type"`
	Value          string `json:"value"`
	Name           string `json:"name"`
	IsOverridden   bool   `json:"is_overridden"`
	IsEnabled      bool   `json:"is_enabled"`
}

// Report types for --raw JSON output.

type customerEntitlementReport struct {
	Customer      customerData                    `json:"customer"`
	Subscriptions []subscriptionEntitlementReport `json:"subscriptions"`
}

type subscriptionEntitlementReport struct {
	Subscription subscriptionData              `json:"subscription"`
	Entitlements []subscriptionEntitlementData `json:"entitlements"`
}

type featureEntitlementReport struct {
	Feature      featureData       `json:"feature"`
	Entitlements []entitlementData `json:"entitlements"`
	Customers    []customerMatch   `json:"customers,omitempty"`
}

type customerMatch struct {
	Customer     customerData     `json:"customer"`
	Subscription subscriptionData `json:"subscription"`
}

func newEntitlementsCmd() *cobra.Command {
	return NewEntitlementsCmdWithClient(resolveAPIClient)
}

// NewEntitlementsCmdWithClient creates the entitlements command with a custom client factory (for testing).
func NewEntitlementsCmdWithClient(factory ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entitlements [customer_id]",
		Short: "Show entitlements for a customer or by feature",
		Long: `Look up entitlements by customer or by feature name.

Use --company (or --name, --email, customer ID) to see all feature entitlements
across a customer's subscriptions. Use --feature to find which plans/items
include a specific feature and which customers have access to it.

Examples:
  cb entitlements --company "Acme"
  cb entitlements --feature "Awareness"
  cb entitlements --email "john@acme.com"
  cb entitlements cust_ABC123
  cb entitlements --feature "Awareness" --raw`,
		Args: cobra.MaximumNArgs(1),
		RunE: makeRunEntitlements(factory),
	}

	cmd.Flags().String("feature", "", "find customers/items entitled to this feature (by name)")
	cmd.Flags().String("company", "", "filter by company name (starts_with)")
	cmd.Flags().String("name", "", "filter by first name (starts_with)")
	cmd.Flags().String("email", "", "filter by email (exact match)")
	cmd.Flags().StringSliceP("data", "d", nil, "arbitrary Chargebee filter params as key=value")
	cmd.Flags().Bool("raw", false, "output raw JSON instead of formatted text")
	cmd.Flags().Int("limit", 5, "max number of customers to show in --feature mode")

	return cmd
}

func makeRunEntitlements(factory ClientFactory) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		feature, _ := cmd.Flags().GetString("feature")
		company, _ := cmd.Flags().GetString("company")
		name, _ := cmd.Flags().GetString("name")
		email, _ := cmd.Flags().GetString("email")
		dataFlags, _ := cmd.Flags().GetStringSlice("data")
		raw, _ := cmd.Flags().GetBool("raw")

		hasID := len(args) == 1
		hasCustomerFilters := company != "" || name != "" || email != "" || len(dataFlags) > 0
		hasFeature := feature != ""

		if !hasID && !hasCustomerFilters && !hasFeature {
			return fmt.Errorf("provide a customer ID, a customer filter (--company, --name, --email), or --feature")
		}
		if hasFeature && (hasID || hasCustomerFilters) {
			return fmt.Errorf("cannot combine --feature with customer filters")
		}
		if hasID && hasCustomerFilters {
			return fmt.Errorf("cannot combine a customer ID with filter flags")
		}

		client, err := factory(cmd)
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		if hasFeature {
			limit, _ := cmd.Flags().GetInt("limit")
			return entitlementsForFeature(ctx, client, feature, raw, limit)
		}
		return entitlementsForCustomer(ctx, client, args, company, name, email, dataFlags, raw)
	}
}

// entitlementsForFeature finds which items/plans include a feature and which customers have it.
func entitlementsForFeature(ctx context.Context, client *api.Client, featureName string, raw bool, maxCustomers int) error {
	out := output.Default

	feat, err := findFeature(ctx, client, featureName)
	if err != nil {
		return err
	}
	if feat == nil {
		out.Warning("No features found matching %q", featureName)
		return nil
	}

	// Get entitlements for this feature from /entitlements API.
	entParams := url.Values{}
	entParams.Set("feature_id[is]", feat.ID)
	entResult, err := client.Get(ctx, "/entitlements", entParams)
	if err != nil {
		return fmt.Errorf("listing entitlements for feature %s: %w", feat.ID, err)
	}
	var entResp entitlementListResponse
	if err := entResult.Decode(&entResp); err != nil {
		return fmt.Errorf("decoding entitlements: %w", err)
	}

	report := featureEntitlementReport{Feature: *feat}
	for _, e := range entResp.List {
		report.Entitlements = append(report.Entitlements, e.Entitlement)
	}

	if !raw {
		out.Status("Feature: %s — %s [%s]", feat.Name, feat.ID, feat.Type)
		if feat.Description != "" {
			out.Dim("  %s", feat.Description)
		}
		if len(report.Entitlements) == 0 {
			out.Dim("  (no entitlements)")
		} else {
			out.Data("  Entitled items:")
			for _, ent := range report.Entitlements {
				out.Data("    %-15s %-35s %s", ent.EntityType, ent.EntityID, ent.Value)
			}
		}
	}

	// Resolve customers that have this feature via their subscriptions.
	customers, err := findCustomersForFeature(ctx, client, feat.ID, report.Entitlements, maxCustomers)
	if err != nil {
		return fmt.Errorf("finding customers for feature %s: %w", feat.ID, err)
	}
	report.Customers = customers

	if !raw {
		if len(customers) == 0 && len(report.Entitlements) > 0 {
			out.Dim("  (no active subscriptions found)")
		}
		for _, m := range customers {
			out.Data("  → %s — Subscription %s [%s]", m.Customer.Company, m.Subscription.ID, m.Subscription.Status)
		}
		if len(customers) == maxCustomers {
			out.Dim("  (showing %d of %d+ customers, use --limit to show more)", maxCustomers, maxCustomers)
		}
	}

	if raw {
		data, err := json.Marshal(report)
		if err != nil {
			return fmt.Errorf("marshaling report: %w", err)
		}
		return printJSON(os.Stdout, data, false)
	}

	return nil
}

// entitlementsForCustomer shows all entitlements across a customer's subscriptions.
func entitlementsForCustomer(ctx context.Context, client *api.Client, args []string, company, name, email string, dataFlags []string, raw bool) error {
	out := output.Default

	customers, err := findCustomers(ctx, client, args, company, name, email, dataFlags)
	if err != nil {
		return err
	}
	if len(customers) == 0 {
		out.Warning("No customers found")
		return nil
	}

	var reports []customerEntitlementReport

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

		report := customerEntitlementReport{Customer: cust}

		if !raw {
			displayName := customerDisplayName(cust)
			if company != "" {
				displayName = cust.Company
			}
			out.Status("Customer: %s — %s", displayName, cust.ID)
		}

		if len(subResp.List) == 0 {
			if !raw {
				out.Dim("  (no subscriptions)")
			}
			reports = append(reports, report)
			continue
		}

		for _, subEntry := range subResp.List {
			sub := subEntry.Subscription

			entResult, err := client.Get(ctx, fmt.Sprintf("/subscriptions/%s/subscription_entitlements", sub.ID), nil)
			if err != nil {
				return fmt.Errorf("listing entitlements for subscription %s: %w", sub.ID, err)
			}
			var entResp subscriptionEntitlementListResponse
			if err := entResult.Decode(&entResp); err != nil {
				return fmt.Errorf("decoding subscription entitlements: %w", err)
			}

			var entitlements []subscriptionEntitlementData
			for _, e := range entResp.List {
				entitlements = append(entitlements, e.SubscriptionEntitlement)
			}

			subReport := subscriptionEntitlementReport{
				Subscription: sub,
				Entitlements: entitlements,
			}
			report.Subscriptions = append(report.Subscriptions, subReport)

			if !raw {
				out.Data("  Subscription: %s [%s]", sub.ID, sub.Status)
				if len(entitlements) == 0 {
					out.Dim("    (no entitlements)")
				}
				for _, ent := range entitlements {
					enabled := "enabled"
					if !ent.IsEnabled {
						enabled = "disabled"
					}
					out.Data("    %-30s %-15s %s", ent.FeatureName, ent.Value, enabled)
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

// findFeature looks up a feature by exact name, falling back to prefix match.
func findFeature(ctx context.Context, client *api.Client, name string) (*featureData, error) {
	params := url.Values{}
	params.Set("name[is]", name)
	result, err := client.Get(ctx, "/features", params)
	if err != nil {
		return nil, fmt.Errorf("listing features: %w", err)
	}
	var resp featureListResponse
	if err := result.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding features: %w", err)
	}
	if len(resp.List) > 0 {
		return &resp.List[0].Feature, nil
	}

	// Fall back to prefix match.
	params = url.Values{}
	params.Set("name[starts_with]", name)
	result, err = client.Get(ctx, "/features", params)
	if err != nil {
		return nil, fmt.Errorf("listing features: %w", err)
	}
	if err := result.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding features: %w", err)
	}
	if len(resp.List) > 0 {
		return &resp.List[0].Feature, nil
	}

	return nil, nil
}

// findCustomers resolves customers from ID, company, name, email, or data flags.
func findCustomers(ctx context.Context, client *api.Client, args []string, company, name, email string, dataFlags []string) ([]customerData, error) {
	if len(args) == 1 {
		result, err := client.Get(ctx, "/customers/"+args[0], nil)
		if err != nil {
			return nil, fmt.Errorf("fetching customer %s: %w", args[0], err)
		}
		var resp singleCustomerResponse
		if err := result.Decode(&resp); err != nil {
			return nil, fmt.Errorf("decoding customer: %w", err)
		}
		return []customerData{resp.Customer}, nil
	}

	params, err := parseDataFlags(dataFlags)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("listing customers: %w", err)
	}
	var resp customerListResponse
	if err := result.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding customers: %w", err)
	}

	var customers []customerData
	for _, entry := range resp.List {
		customers = append(customers, entry.Customer)
	}
	return customers, nil
}

// findCustomersForFeature finds customers whose subscriptions include the given feature.
// It first tries filtering subscriptions by the entitled entity IDs (fast path). If that
// yields no results, it falls back to scanning active subscriptions and checking each
// one's subscription_entitlements sub-resource.
func findCustomersForFeature(ctx context.Context, client *api.Client, featureID string, entitlements []entitlementData, limit int) ([]customerMatch, error) {
	if len(entitlements) == 0 {
		return nil, nil
	}

	// Fast path: find subscriptions via entity-based filters.
	matches, err := findSubscriptionsViaEntities(ctx, client, entitlements, limit)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches, nil
	}

	// Slow path: scan active subscriptions and check each one's entitlements.
	return findSubscriptionsViaScan(ctx, client, featureID, limit)
}

// findSubscriptionsViaEntities tries to find subscriptions by filtering on plan_id or
// item_price_id based on the entitlement entity type. Returns nil (not an error) if
// filters don't match any subscriptions.
func findSubscriptionsViaEntities(ctx context.Context, client *api.Client, entitlements []entitlementData, limit int) ([]customerMatch, error) {
	seen := make(map[string]bool)
	var matches []customerMatch

	for _, ent := range entitlements {
		subParams := url.Values{}
		switch ent.EntityType {
		case "plan":
			subParams.Set("plan_id[is]", ent.EntityID)
		case "plan_price", "addon_price":
			subParams.Set("item_price_id[is]", ent.EntityID)
		default:
			continue
		}

		subResult, err := client.Get(ctx, "/subscriptions", subParams)
		if err != nil {
			// Filter may not be supported; skip to next entity.
			continue
		}
		var subResp subscriptionListResponse
		if err := subResult.Decode(&subResp); err != nil {
			continue
		}

		for _, se := range subResp.List {
			sub := se.Subscription
			if seen[sub.CustomerID] {
				continue
			}

			custResult, err := client.Get(ctx, "/customers/"+sub.CustomerID, nil)
			if err != nil {
				continue
			}
			var custResp singleCustomerResponse
			if err := custResult.Decode(&custResp); err != nil {
				continue
			}

			seen[sub.CustomerID] = true
			matches = append(matches, customerMatch{
				Customer:     custResp.Customer,
				Subscription: sub,
			})
			if len(matches) >= limit {
				return matches, nil
			}
		}
	}

	return matches, nil
}

// findSubscriptionsViaScan lists active subscriptions and checks each one's entitlements
// for the given feature. This is the reliable fallback when entity-based filters don't work.
func findSubscriptionsViaScan(ctx context.Context, client *api.Client, featureID string, limit int) ([]customerMatch, error) {
	seen := make(map[string]bool)
	var matches []customerMatch
	offset := ""

	for {
		params := url.Values{}
		params.Set("limit", "100")
		if offset != "" {
			params.Set("offset", offset)
		}

		result, err := client.Get(ctx, "/subscriptions", params)
		if err != nil {
			return nil, fmt.Errorf("listing subscriptions: %w", err)
		}
		var subResp paginatedSubscriptionResponse
		if err := result.Decode(&subResp); err != nil {
			return nil, fmt.Errorf("decoding subscriptions: %w", err)
		}

		for _, se := range subResp.List {
			sub := se.Subscription
			if seen[sub.CustomerID] {
				continue
			}

			entResult, err := client.Get(ctx, fmt.Sprintf("/subscriptions/%s/subscription_entitlements", sub.ID), nil)
			if err != nil {
				continue
			}
			var entResp subscriptionEntitlementListResponse
			if err := entResult.Decode(&entResp); err != nil {
				continue
			}

			hasFeature := false
			for _, e := range entResp.List {
				if e.SubscriptionEntitlement.FeatureID == featureID {
					hasFeature = true
					break
				}
			}
			if !hasFeature {
				continue
			}

			custResult, err := client.Get(ctx, "/customers/"+sub.CustomerID, nil)
			if err != nil {
				continue
			}
			var custResp singleCustomerResponse
			if err := custResult.Decode(&custResp); err != nil {
				continue
			}

			seen[sub.CustomerID] = true
			matches = append(matches, customerMatch{
				Customer:     custResp.Customer,
				Subscription: sub,
			})
			if len(matches) >= limit {
				return matches, nil
			}
		}

		if subResp.NextOffset == "" {
			break
		}
		offset = subResp.NextOffset
	}

	return matches, nil
}
