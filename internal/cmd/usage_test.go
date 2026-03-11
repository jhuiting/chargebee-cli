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

// JSON helpers for building Chargebee-style responses.

func customerJSON(id, first, last, company, email string) map[string]any {
	return map[string]any{
		"customer": map[string]any{
			"id":         id,
			"first_name": first,
			"last_name":  last,
			"company":    company,
			"email":      email,
		},
	}
}

func subscriptionJSON(id, customerID, status string) map[string]any {
	return map[string]any{
		"subscription": map[string]any{
			"id":          id,
			"customer_id": customerID,
			"status":      status,
		},
	}
}

func usageJSON(id, itemPriceID, subID string, date int64, qty string) map[string]any {
	return map[string]any{
		"usage": map[string]any{
			"id":              id,
			"item_price_id":   itemPriceID,
			"subscription_id": subID,
			"usage_date":      date,
			"quantity":        qty,
		},
	}
}

func usageTestServer(t *testing.T, customers []map[string]any, subs []map[string]any, usages []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			resp := map[string]any{"list": customers}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": subs}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/usages":
			resp := map[string]any{"list": usages}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/v2/customers/") {
				if len(customers) > 0 {
					_ = json.NewEncoder(w).Encode(customers[0])
				} else {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]string{"error_msg": "not found", "error_code": "resource_not_found"})
				}
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error_msg": "unknown path: " + r.URL.Path})
		}
	}))
}

func TestUsageCmd_DirectCustomerID(t *testing.T) {
	var gotCustomerPath string
	var gotSubQuery, gotUsageQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers/cust_123":
			gotCustomerPath = r.URL.Path
			resp := customerJSON("cust_123", "John", "Doe", "Acme Corp", "john@acme.com")
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			gotSubQuery = r.URL.Query().Get("customer_id[is]")
			resp := map[string]any{"list": []any{subscriptionJSON("sub_001", "cust_123", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/usages":
			gotUsageQuery = r.URL.Query().Get("subscription_id[is]")
			resp := map[string]any{"list": []any{usageJSON("use_1", "item-price-monthly", "sub_001", 1740787200, "150")}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "cust_123"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "/api/v2/customers/cust_123", gotCustomerPath)
	assert.Equal(t, "cust_123", gotSubQuery)
	assert.Equal(t, "sub_001", gotUsageQuery)
}

func TestUsageCmd_FilterByCompany(t *testing.T) {
	var gotCompanyFilter string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			gotCompanyFilter = r.URL.Query().Get("company[starts_with]")
			resp := map[string]any{"list": []any{customerJSON("cust_1", "Jane", "Smith", "Acme", "jane@acme.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--company", "Acme"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Acme", gotCompanyFilter)
}

func TestUsageCmd_FilterByName(t *testing.T) {
	var gotNameFilter string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			gotNameFilter = r.URL.Query().Get("first_name[starts_with]")
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "", "john@test.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--name", "John"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "John", gotNameFilter)
}

func TestUsageCmd_FilterByEmail(t *testing.T) {
	var gotEmailFilter string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			gotEmailFilter = r.URL.Query().Get("email[is]")
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "", "john@acme.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--email", "john@acme.com"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "john@acme.com", gotEmailFilter)
}

func TestUsageCmd_DataFlagFilter(t *testing.T) {
	var gotEmailFilter string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			gotEmailFilter = r.URL.Query().Get("email[is]")
			resp := map[string]any{"list": []any{customerJSON("cust_1", "A", "B", "C", "a@b.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "-d", "email[is]=john@example.com"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "john@example.com", gotEmailFilter)
}

func TestUsageCmd_NoCustomersFound(t *testing.T) {
	srv := usageTestServer(t, []map[string]any{}, nil, nil)
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--company", "NonExistent"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestUsageCmd_NoSubscriptions(t *testing.T) {
	custs := []map[string]any{customerJSON("cust_1", "John", "Doe", "Acme", "john@acme.com")}
	srv := usageTestServer(t, custs, []map[string]any{}, nil)
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--company", "Acme"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestUsageCmd_NoUsageRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers":
			resp := map[string]any{"list": []any{customerJSON("cust_1", "John", "Doe", "Acme", "j@a.com")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/usages":
			resp := map[string]any{"list": []any{}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "--company", "Acme"})
	err := root.Execute()

	assert.NoError(t, err)
}

func TestUsageCmd_RequiresInput(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"usage"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provide a customer ID or at least one filter")
}

func TestUsageCmd_RejectsIDWithFilters(t *testing.T) {
	root := newTestRootCmd("http://unused", "s", "k")
	root.SetArgs([]string{"usage", "cust_123", "--company", "Acme"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot combine")
}

func TestUsageCmd_RawJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/customers/cust_1":
			_ = json.NewEncoder(w).Encode(customerJSON("cust_1", "John", "Doe", "Acme", "j@a.com"))
		case "/api/v2/subscriptions":
			resp := map[string]any{"list": []any{subscriptionJSON("sub_1", "cust_1", "active")}}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v2/usages":
			resp := map[string]any{"list": []any{usageJSON("use_1", "item-monthly", "sub_1", 1740787200, "100")}}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	root := newTestRootCmd(srv.URL, "test-site", "test-key")
	root.SetArgs([]string{"usage", "cust_1", "--raw"})
	err := root.Execute()

	require.NoError(t, err)
}
