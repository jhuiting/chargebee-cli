package cmd

// allResources returns all Chargebee API resource definitions.
func allResources() []ResourceDef {
	resources := []struct {
		name, singular, apiPath, timestampField string
		filters                                 []FilterDef
	}{
		{"customers", "customer", "/customers", "created_at", []FilterDef{
			{Flag: "company", Field: "company", Operator: "starts_with", Help: "filter by company name (prefix match)"},
			{Flag: "email", Field: "email", Operator: "is", Help: "filter by email (exact match)"},
		}},
		{"subscriptions", "subscription", "/subscriptions", "created_at", []FilterDef{
			{Flag: "status", Field: "status", Operator: "is", Help: "filter by status (active, cancelled, etc.)",
				ValidValues: []string{"future", "in_trial", "active", "non_renewing", "paused", "cancelled", "transferred"}},
			{Flag: "customer-id", Field: "customer_id", Operator: "is", Help: "filter by customer ID"},
		}},
		{"invoices", "invoice", "/invoices", "created_at", []FilterDef{
			{Flag: "status", Field: "status", Operator: "is", Help: "filter by status (paid, payment_due, etc.)",
				ValidValues: []string{"paid", "posted", "payment_due", "not_paid", "voided", "pending"}},
			{Flag: "customer-id", Field: "customer_id", Operator: "is", Help: "filter by customer ID"},
		}},
		{"items", "item", "/items", "created_at", []FilterDef{
			{Flag: "status", Field: "status", Operator: "is", Help: "filter by status (active, archived, deleted)",
				ValidValues: []string{"active", "archived", "deleted"}},
			{Flag: "name", Field: "name", Operator: "starts_with", Help: "filter by item name (prefix match)"},
			{Flag: "type", Field: "type", Operator: "is", Help: "filter by item type (plan, addon, charge)",
				ValidValues: []string{"plan", "addon", "charge"}},
		}},
		{"item-prices", "item price", "/item_prices", "created_at", []FilterDef{
			{Flag: "status", Field: "status", Operator: "is", Help: "filter by status (active, archived, deleted)",
				ValidValues: []string{"active", "archived", "deleted"}},
			{Flag: "item-id", Field: "item_id", Operator: "is", Help: "filter by parent item ID"},
		}},
		{"events", "event", "/events", "occurred_at", []FilterDef{
			{Flag: "event-type", Field: "event_type", Operator: "is", Help: "filter by event type (e.g. customer_created)"},
		}},
	}

	defs := make([]ResourceDef, len(resources))
	for i, r := range resources {
		defs[i] = ResourceDef{
			Name:           r.name,
			Singular:       r.singular,
			APIPath:        r.apiPath,
			Operations:     ReadOps(r.singular),
			Filters:        r.filters,
			TimestampField: r.timestampField,
		}
	}
	return defs
}
