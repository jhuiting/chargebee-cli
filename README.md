# Chargebee CLI

[![CI](https://github.com/jhuiting/chargebee-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/jhuiting/chargebee-cli/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Explore and monitor your Chargebee integration from the terminal.

## Why Chargebee CLI?

**Look up a customer's usage in seconds, not clicks.** Instead of navigating through the Chargebee dashboard to find a customer, opening their subscriptions, and hunting for usage records — just ask:

```sh
cb usage --company "Acme"
cb usage --email "john@acme.com"
cb usage cust_HxAz1mNQ           # or directly by ID
```

That's it. One command gives you the customer, their subscriptions, and all usage records — formatted and ready to read. Pipe `--raw` for JSON you can feed into scripts.

**Develop webhook integrations without deploying.** Chargebee doesn't offer a local event stream — `cb listen` bridges that gap. It polls your site's events and forwards them to your local server with HMAC signatures, just like production:

```sh
cb listen -f http://localhost:4242/webhook -e payment_succeeded,subscription_cancelled
```

Filter by event type, inspect payloads with `-j`, and iterate on your handler without touching staging.

**Query any Chargebee resource without leaving your terminal.** Browse customers, subscriptions, invoices, items, and more with built-in filter flags. Timestamp fields accept human-readable dates — no more unix math:

```sh
cb customers list --company Acme -l 5
cb customers list --email john@acme.com
cb subscriptions list --customer-id cust_ABC123 --status active
cb invoices list -d "created_at[after]=2024-01-01"
```

**Switch between sites instantly.** Manage production, staging, and test environments as named profiles. No more juggling API keys:

```sh
cb login --profile staging
cb switch staging
cb customers list                 # now hitting staging
```

**Jump to the right dashboard page.** Stop bookmarking Chargebee URLs:

```sh
cb open webhooks                  # straight to webhook settings
cb open api                       # API reference
```

## Install

### Homebrew (macOS)

```sh
brew install jhuiting/tap/chargebee-cli
```

To upgrade:

```sh
brew update && brew upgrade chargebee-cli
```

### Binary download

Download pre-built binaries from the [Releases](https://github.com/jhuiting/chargebee-cli/releases) page.

## Quick Start

```sh
# Authenticate with your Chargebee site
cb login

# List customers
cb customers list -l 5

# Retrieve a specific customer
cb customers retrieve cust_123

# Listen for events and forward to a local webhook endpoint
cb listen -f http://localhost:4242/webhook

# Make arbitrary API calls
cb get /subscriptions -l 10

# Open the Chargebee dashboard
cb open
```

## Commands

| Command | Description |
|---------|-------------|
| `cb login` | Authenticate with your Chargebee site |
| `cb logout` | Remove stored credentials |
| `cb switch [profile]` | Switch the active profile |
| `cb status` | Show configuration and verify API connectivity |
| `cb config get\|set\|list` | Manage CLI configuration |
| `cb customers <op>` | List or retrieve customers |
| `cb subscriptions <op>` | List or retrieve subscriptions |
| `cb invoices <op>` | List or retrieve invoices |
| `cb items <op>` | List or retrieve items (plans, addons, charges) |
| `cb item-prices <op>` | List or retrieve item prices |
| `cb events <op>` | List or retrieve events |
| `cb get <path>` | GET request to the Chargebee API |
| `cb listen` | Poll events and forward webhooks locally |
| `cb usage [customer_id]` | Show usage records for a customer |
| `cb open [page]` | Open dashboard, docs, or API reference in the browser |
| `cb completion` | Generate shell completion scripts |
| `cb version` | Print the CLI version |

### Resources

All resource commands support `list` and `retrieve` with convenience filter flags.
List commands accept `--after` and `--before` for time-based filtering — values can be
ISO dates (`2024-01-01`), RFC3339 (`2024-01-01T15:04:05Z`), relative durations (`7d`, `24h`, `30m`), or unix timestamps.

```sh
cb customers list -l 10                                             # Limit results
cb customers list --company Acme                                     # Filter by company prefix
cb customers list --email john@acme.com                              # Filter by email
cb customers list --after 2024-01-01                                 # Created after a date
cb customers list --after 7d                                         # Created in the last 7 days
cb subscriptions list --customer-id cust_ABC123 --status active      # Filter by customer and status
cb subscriptions list --status active --before 30d                   # Active, created before 30 days ago
cb invoices list --customer-id cust_ABC123 --after 7d                # Invoices by customer, last 7 days
cb events list --event-type customer_created --after 24h             # Recent events by type
cb invoices list -d "created_at[after]=2024-01-01"                   # Date in -d flags works too
cb customers retrieve cust_123                                       # Get by ID
cb customers retrieve cust_123 --raw                                 # Compact JSON output
cb subscriptions list -s                                             # Show response headers
```

Use `cb <resource> list --help` to see all available filter flags for a resource.

### Listen

```sh
cb listen -f http://localhost:4242/webhook                 # Forward events
cb listen -e customer_created,payment_succeeded            # Filter by event type
cb listen -j                                               # Print JSON to stdout
cb listen --since 2024-01-01                               # Events after a date
cb listen --since 7d                                       # Events from last 7 days
cb listen --poll-interval 5s                               # Custom poll interval
cb listen --print-signing-key                              # Print signing key and exit
cb listen --skip-verify                                    # Skip TLS verification
cb listen -H "Authorization:Bearer tok123"                 # Custom forwarding headers
```

Forwarded events are signed with an HMAC key sent in the `X-CB-CLI-Signature` header.

### GET

```sh
cb get /customers -l 10                                    # List with limit
cb get /customers -d "status[is]=active"                   # Query parameters
cb get /customers/cust_123 -s                              # Show response headers
cb get /subscriptions --raw                                # Compact JSON output
```

### Open

```sh
cb open                                                    # Dashboard (default)
cb open docs                                               # Product documentation
cb open api                                                # API reference
cb open --list                                             # List available pages
```

## Configuration

Credentials are stored in `~/.config/chargebee/config.toml`. Multiple profiles are supported:

```sh
cb login --profile staging            # Login to a named profile
cb switch                             # List profiles (* marks active)
cb switch staging                     # Activate a profile
cb get /customers --profile staging   # Use a profile for one command
```

Environment variables for CI/CD:

```sh
export CB_SITE=mysite
export CB_API_KEY=live_xxx
export CB_NO_UPDATE_CHECK=1          # Disable update notifications (any non-empty value)
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--site` | Chargebee site name (overrides config) |
| `--api-key` | API key (overrides config) |
| `--profile` | Named profile to use (overrides `CB_PROFILE`) |

## Development

```sh
make deps      # Install dependencies
make verify    # Format, lint, and build
make test      # Run tests
make build     # Build binary to ./bin/cb
```

## License

MIT License — see [LICENSE](LICENSE) for details.
