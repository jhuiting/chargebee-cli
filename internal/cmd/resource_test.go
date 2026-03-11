package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/cmd"
)

func newResourceTestCmd(baseURL string) *cobra.Command {
	root := &cobra.Command{
		Use:           "cb",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("site", "test", "")
	root.PersistentFlags().String("api-key", "test-key", "")
	root.PersistentFlags().String("profile", "", "")

	factory := func(_ *cobra.Command) (*api.Client, error) {
		c := api.NewClient("test", "test-key")
		c.BaseURL = baseURL + "/api/v2"
		return c, nil
	}

	for _, def := range []cmd.ResourceDef{
		{
			Name:       "customers",
			Singular:   "customer",
			APIPath:    "/customers",
			Operations: cmd.ReadOps("customer"),
			Filters: []cmd.FilterDef{
				{Flag: "company", Field: "company", Operator: "starts_with", Help: "filter by company name prefix"},
				{Flag: "email", Field: "email", Operator: "is", Help: "filter by email address"},
			},
		},
	} {
		cmd.RegisterResource(root, def, factory)
	}

	return root
}

func TestResourceList(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/v2/customers", gotPath)
}

func TestResourceListWithLimit(t *testing.T) {
	var gotLimit string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "-l", "5"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "5", gotLimit)
}

func TestResourceRetrieve(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]string{"id": "cust_123"}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "retrieve", "cust_123"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/v2/customers/cust_123", gotPath)
}

func TestResourceRetrieveRequiresID(t *testing.T) {
	root := newResourceTestCmd("http://unused")
	root.SetArgs([]string{"customers", "retrieve"})
	err := root.Execute()
	assert.Error(t, err)
}

func TestResourceDataParams(t *testing.T) {
	var gotStatus, gotName string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStatus = r.URL.Query().Get("status[is]")
		gotName = r.URL.Query().Get("first_name")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "-d", "status[is]=active", "-d", "first_name=Test"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "active", gotStatus)
	assert.Equal(t, "Test", gotName)
}

func TestResourceListWithFilterFlag(t *testing.T) {
	var gotCompany string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCompany = r.URL.Query().Get("company[starts_with]")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "--company", "Acme"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Acme", gotCompany)
}

func TestResourceListMultipleFilters(t *testing.T) {
	var gotCompany, gotEmail string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCompany = r.URL.Query().Get("company[starts_with]")
		gotEmail = r.URL.Query().Get("email[is]")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "--company", "Acme", "--email", "john@acme.com"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Acme", gotCompany)
	assert.Equal(t, "john@acme.com", gotEmail)
}

func TestResourceListFilterWithDataFlag(t *testing.T) {
	var gotCompany, gotCreatedAt string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCompany = r.URL.Query().Get("company[starts_with]")
		gotCreatedAt = r.URL.Query().Get("created_at[after]")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "--company", "Acme", "-d", "created_at[after]=1700000000"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Acme", gotCompany)
	assert.Equal(t, "1700000000", gotCreatedAt)
}

func TestResourceRetrieveHasNoFilterFlags(t *testing.T) {
	root := newResourceTestCmd("http://unused")

	customersCmd, _, _ := root.Find([]string{"customers", "retrieve"})
	require.NotNil(t, customersCmd)

	assert.Nil(t, customersCmd.Flags().Lookup("company"), "retrieve should not have filter flags")
	assert.Nil(t, customersCmd.Flags().Lookup("email"), "retrieve should not have filter flags")
}
