package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/output"
)

// pageEntry describes a page shortcut.
type pageEntry struct {
	Description string
	URL         string // static URL (no site needed)
	SitePath    string // path under https://{site}.chargebee.com/
}

var pages = map[string]pageEntry{
	"dashboard":     {Description: "Site dashboard", SitePath: ""},
	"docs":          {Description: "Product documentation", URL: "https://www.chargebee.com/docs/2.0/"},
	"api":           {Description: "API reference", URL: "https://apidocs.chargebee.com/docs/api"},
	"settings":      {Description: "Site settings", SitePath: "settings"},
	"webhooks":      {Description: "Webhook settings", SitePath: "settings/webhooks"},
	"customers":     {Description: "Customers list", SitePath: "customers"},
	"subscriptions": {Description: "Subscriptions list", SitePath: "subscriptions"},
	"invoices":      {Description: "Invoices list", SitePath: "invoices"},
	"catalog":       {Description: "Product Catalog management", SitePath: "product-catalog"},
	"events":        {Description: "Events list", SitePath: "events"},
}

func newOpenCmd() *cobra.Command {
	validArgs := make([]string, 0, len(pages))
	for name := range pages {
		validArgs = append(validArgs, name)
	}
	sort.Strings(validArgs)

	cmd := &cobra.Command{
		Use:   "open [page]",
		Short: "Open Chargebee pages in the browser",
		Long: `Open Chargebee dashboard, documentation, or other pages in your default browser.

Examples:
  cb open
  cb open dashboard
  cb open docs
  cb open api
  cb open --list`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: validArgs,
		RunE:      runOpen,
	}

	cmd.Flags().Bool("list", false, "list all available pages")

	return cmd
}

func runOpen(cmd *cobra.Command, args []string) error {
	listPages, _ := cmd.Flags().GetBool("list")

	if listPages {
		return printPageList()
	}

	page := "dashboard"
	if len(args) > 0 {
		page = args[0]
	}

	site, _, _ := resolveCredentials(cmd)
	url, err := ResolvePageURL(page, site)
	if err != nil {
		return err
	}

	output.Default.Status("Opening %s...", page)
	return openBrowser(url)
}

func printPageList() error {
	out := output.Default
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := pages[name]
		url := entry.URL
		if url == "" {
			url = fmt.Sprintf("https://{site}.chargebee.com/%s", entry.SitePath)
		}
		out.KeyValue(fmt.Sprintf("%-15s", name), fmt.Sprintf("%s  %s", entry.Description, url))
	}
	return nil
}

// ResolvePageURL returns the URL for the given page name.
// For site-specific pages, site must be non-empty.
func ResolvePageURL(page, site string) (string, error) {
	entry, ok := pages[page]
	if !ok {
		names := make([]string, 0, len(pages))
		for name := range pages {
			names = append(names, name)
		}
		sort.Strings(names)
		return "", fmt.Errorf("unknown page %q; valid pages: %s", page, joinNames(names))
	}

	if entry.URL != "" {
		return entry.URL, nil
	}

	if site == "" {
		return "", fmt.Errorf("not logged in — run 'cb login' to use 'cb open %s'", page)
	}

	if entry.SitePath == "" {
		return fmt.Sprintf("https://%s.chargebee.com", site), nil
	}
	return fmt.Sprintf("https://%s.chargebee.com/%s", site, entry.SitePath), nil
}

func joinNames(names []string) string {
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(n)
	}
	return b.String()
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %q", runtime.GOOS)
	}
	return cmd.Start()
}
