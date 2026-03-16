package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// JSON helpers for entitlement responses.

func featureJSON(id, name, featureType, status string) map[string]any {
	return map[string]any{
		"feature": map[string]any{
			"id":     id,
			"name":   name,
			"type":   featureType,
			"status": status,
		},
	}
}

func entitlementJSON(id, entityID, entityType, featureID, featureName, value string) map[string]any {
	return map[string]any{
		"entitlement": map[string]any{
			"id":           id,
			"entity_id":    entityID,
			"entity_type":  entityType,
			"feature_id":   featureID,
			"feature_name": featureName,
			"value":        value,
		},
	}
}

func subscriptionEntitlementJSON(subID, featureID, featureName, value string, isEnabled bool) map[string]any {
	return map[string]any{
		"subscription_entitlement": map[string]any{
			"subscription_id": subID,
			"feature_id":      featureID,
			"feature_name":    featureName,
			"feature_type":    "switch",
			"value":           value,
			"is_enabled":      isEnabled,
			"is_overridden":   false,
		},
	}
}

func TestEntitlementsCmd_CustomerByCompany(t *testing.T) {
	var gotCompanyFilter string
	var gotSubFilter string
	var gotEntPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			gotCompanyFilter = r.URL.Query().Get("company[starts_with]")
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			gotSubFilter = r.URL.Query().Get("customer_id[is]")
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions/sub_1/subscription_entitlements":
			gotEntPath = r.URL.Path
			resp := map[string]any{"list": []any{
				subscriptionEntitlementJSON("sub_1", "feat_1", "Awareness", "true", true),
				subscriptionEntitlementJSON("sub_1", "feat_2", "Analytics", "basic", true),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--company", "Acme"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Acme", gotCompanyFilter)
	assert.Equal(t, "cust_1", gotSubFilter)
	assert.Equal(t, "/api/v2/subscriptions/sub_1/subscription_entitlements", gotEntPath)
}

func TestEntitlementsCmd_CustomerByID(t *testing.T) {
	var gotCustomerPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers/cust_123":
			gotCustomerPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(customerJSON("cust_123", "Jane", "Smith", "BigCorp", "jane@bigcorp.com"))
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "cust_123"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "/api/v2/customers/cust_123", gotCustomerPath)
}

func TestEntitlementsCmd_ByFeature(t *testing.T) {
	var gotFeatureFilter string
	var gotEntitlementFilter string
	var gotPlanFilter string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/features":
			gotFeatureFilter = r.URL.Query().Get("name[is]")
			resp := map[string]any{"list": []any{featureJSON("feat_1", "Awareness", "switch", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/entitlements":
			gotEntitlementFilter = r.URL.Query().Get("feature_id[is]")
			resp := map[string]any{"list": []any{
				entitlementJSON("ent_1", "plan-basic", "plan", "feat_1", "Awareness", "true"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			gotPlanFilter = r.URL.Query().Get("plan_id[is]")
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "Awareness"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Awareness", gotFeatureFilter)
	assert.Equal(t, "feat_1", gotEntitlementFilter)
	assert.Equal(t, "plan-basic", gotPlanFilter)
}

func TestEntitlementsCmd_ByFeatureFallbackToPrefix(t *testing.T) {
	var gotStartsWith string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/features":
			if r.URL.Query().Get("name[is]") != "" {
				resp := map[string]any{"list": []any{}}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			gotStartsWith = r.URL.Query().Get("name[starts_with]")
			resp := map[string]any{"list": []any{featureJSON("feat_1", "Awareness Pro", "switch", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/entitlements":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "Awareness"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Awareness", gotStartsWith)
}

func TestEntitlementsCmd_FeatureNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"list": []any{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "NonExistent"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestEntitlementsCmd_NoCustomersFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"list": []any{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--company", "NonExistent"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestEntitlementsCmd_NoSubscriptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "Acme", "j@a.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--company", "Acme"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestEntitlementsCmd_NoEntitlements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/customers":
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "Acme", "j@a.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasPrefix(r.URL.Path, "/api/v2/subscriptions/") && strings.HasSuffix(r.URL.Path, "/subscription_entitlements"):
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--company", "Acme"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestEntitlementsCmd_RequiresInput(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"entitlements"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provide a customer ID")
}

func TestEntitlementsCmd_RejectsFeatureWithCustomerID(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"entitlements", "cust_123", "--feature", "Awareness"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot combine")
}

func TestEntitlementsCmd_RejectsIDWithFilters(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"entitlements", "cust_123", "--company", "Acme"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot combine")
}

func TestEntitlementsCmd_RawJSON_Customer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "j@a.com"))
		case r.URL.Path == "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasPrefix(r.URL.Path, "/api/v2/subscriptions/") && strings.HasSuffix(r.URL.Path, "/subscription_entitlements"):
			resp := map[string]any{"list": []any{
				subscriptionEntitlementJSON("sub_1", "feat_1", "Awareness", "true", true),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "cust_1", "--raw"})
	err := root.Execute()

	require.NoError(t, err)
}

func TestEntitlementsCmd_RawJSON_Feature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/features":
			resp := map[string]any{"list": []any{featureJSON("feat_1", "Awareness", "switch", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/entitlements":
			resp := map[string]any{"list": []any{
				entitlementJSON("ent_1", "plan-basic", "plan", "feat_1", "Awareness", "true"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "Awareness", "--raw"})
	err := root.Execute()

	require.NoError(t, err)
}

func TestEntitlementsCmd_ByFeature_MultipleCustomers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/features":
			resp := map[string]any{"list": []any{featureJSON("feat_1", "Awareness", "switch", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/entitlements":
			resp := map[string]any{"list": []any{
				entitlementJSON("ent_1", "plan-basic", "plan", "feat_1", "Awareness", "true"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			// Fast path: return subscriptions with this plan (includes duplicates for dedup test).
			resp := map[string]any{"list": []any{
				subscriptionJSON("sub_1", "cust_1", "active"),
				subscriptionJSON("sub_2", "cust_2", "active"),
				subscriptionJSON("sub_3", "cust_1", "active"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com"))
		case "/api/v2/customers/cust_2":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_2", "Jane", "Smith", "BigCorp", "jane@bigcorp.com"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "Awareness", "--raw"})
	err := root.Execute()

	require.NoError(t, err)
}

func TestEntitlementsCmd_ByFeature_ScanFallback(t *testing.T) {
	// When the fast path (plan_id filter) returns empty, the slow path scans subscriptions
	// and checks each one's subscription_entitlements for the feature.
	var gotSubEntPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/features":
			resp := map[string]any{"list": []any{featureJSON("feat_1", "Awareness", "switch", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/entitlements":
			resp := map[string]any{"list": []any{
				entitlementJSON("ent_1", "plan-basic", "plan", "feat_1", "Awareness", "true"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			if r.URL.Query().Get("plan_id[is]") != "" {
				// Fast path returns empty — filter not supported or no match.
				resp := map[string]any{"list": []any{}}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Slow path: list active subscriptions.
			resp := map[string]any{"list": []any{
				subscriptionJSON("sub_1", "cust_1", "active"),
				subscriptionJSON("sub_2", "cust_2", "active"),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions/sub_1/subscription_entitlements":
			gotSubEntPath = r.URL.Path
			resp := map[string]any{"list": []any{
				subscriptionEntitlementJSON("sub_1", "feat_1", "Awareness", "true", true),
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions/sub_2/subscription_entitlements":
			// sub_2 does not have this feature.
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"entitlements", "--feature", "Awareness"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "/api/v2/subscriptions/sub_1/subscription_entitlements", gotSubEntPath)
}
