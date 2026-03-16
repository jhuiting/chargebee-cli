package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/cmd"
	"github.com/jhuiting/chargebee-cli/internal/output"
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

func TestResourceListLimitExceedsMax(t *testing.T) {
	root := newResourceTestCmd("http://unused")
	root.SetArgs([]string{"customers", "list", "-l", "101"})
	err := root.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--limit must be between 1 and 100")
}

func TestResourceListFilterValidation(t *testing.T) {
	factory := func(_ *cobra.Command) (*api.Client, error) {
		c := api.NewClient("test", "test-key")
		c.BaseURL = "http://unused/api/v2"
		return c, nil
	}

	t.Run("rejects invalid value", func(t *testing.T) {
		root := &cobra.Command{Use: "cb", SilenceUsage: true, SilenceErrors: true}
		root.PersistentFlags().String("site", "test", "")
		root.PersistentFlags().String("api-key", "test-key", "")
		root.PersistentFlags().String("profile", "", "")

		cmd.RegisterResource(root, cmd.ResourceDef{
			Name:       "items",
			Singular:   "item",
			APIPath:    "/items",
			Operations: cmd.ReadOps("item"),
			Filters: []cmd.FilterDef{
				{Flag: "type", Field: "type", Operator: "is", Help: "filter by type",
					ValidValues: []string{"plan", "addon", "charge"}},
			},
		}, factory)

		root.SetArgs([]string{"items", "list", "--type", "invalid"})
		err := root.Execute()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid --type value "invalid"`)
		assert.Contains(t, err.Error(), "plan, addon, charge")
	})

	t.Run("accepts valid value", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
		}))
		defer srv.Close()

		srvFactory := func(_ *cobra.Command) (*api.Client, error) {
			c := api.NewClient("test", "test-key")
			c.BaseURL = srv.URL + "/api/v2"
			return c, nil
		}

		root := &cobra.Command{Use: "cb", SilenceUsage: true, SilenceErrors: true}
		root.PersistentFlags().String("site", "test", "")
		root.PersistentFlags().String("api-key", "test-key", "")
		root.PersistentFlags().String("profile", "", "")

		cmd.RegisterResource(root, cmd.ResourceDef{
			Name:       "items",
			Singular:   "item",
			APIPath:    "/items",
			Operations: cmd.ReadOps("item"),
			Filters: []cmd.FilterDef{
				{Flag: "type", Field: "type", Operator: "is", Help: "filter by type",
					ValidValues: []string{"plan", "addon", "charge"}},
			},
		}, srvFactory)

		root.SetArgs([]string{"items", "list", "--type", "addon"})
		err := root.Execute()

		require.NoError(t, err)
	})
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

func TestResourceListEmptyShowsNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	}))
	defer srv.Close()

	var stderrBuf bytes.Buffer
	out := output.New(&stderrBuf, io.Discard)

	origDefault := output.Default
	output.Default = out
	defer func() { output.Default = origDefault }()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list"})
	err := root.Execute()

	require.NoError(t, err)
	assert.Contains(t, stderrBuf.String(), "No results.")
}

func TestResourceListEmptyRawPassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"list":[]}`))
	}))
	defer srv.Close()

	var stderrBuf bytes.Buffer
	out := output.New(&stderrBuf, io.Discard)

	origDefault := output.Default
	output.Default = out
	defer func() { output.Default = origDefault }()

	root := newResourceTestCmd(srv.URL)
	root.SetArgs([]string{"customers", "list", "--raw"})
	err := root.Execute()

	require.NoError(t, err)
	assert.NotContains(t, stderrBuf.String(), "No results.")
}
