package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jhuiting/chargebee-cli/internal/api"
	"github.com/jhuiting/chargebee-cli/internal/output"
	"github.com/jhuiting/chargebee-cli/internal/timeutil"
	"github.com/jhuiting/chargebee-cli/internal/webhook"
)

func newListenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for Chargebee events and optionally forward them to a local endpoint",
		Long: `Poll the Chargebee Events API and display events in the terminal.
Optionally forward each event as an HTTP POST to a local webhook endpoint.

Examples:
  cb listen -f http://localhost:4242/webhook
  cb listen -f http://localhost:4242/webhook -e customer_created,payment_succeeded
  cb listen --print-signing-key`,
		RunE: runListen,
	}

	cmd.Flags().StringP("forward-to", "f", "", "local URL to forward events to (e.g. http://localhost:4242/webhook)")
	cmd.Flags().StringSliceP("events", "e", nil, "filter by event types (comma-separated)")
	cmd.Flags().Duration("poll-interval", 2*time.Second, "how often to poll for events")
	cmd.Flags().BoolP("print-json", "j", false, "print full JSON payloads to stdout")
	cmd.Flags().String("since", "", "only process events after this time (ISO 8601, unix, or relative like 24h, 7d)")
	cmd.Flags().StringSliceP("headers", "H", nil, `custom headers to forward (format: "Key:Value")`)
	cmd.Flags().Bool("skip-verify", false, "skip TLS certificate verification when forwarding to HTTPS endpoints")
	cmd.Flags().Bool("print-signing-key", false, "print the HMAC signing key used to sign forwarded events (X-CB-CLI-Signature)")

	return cmd
}

func runListen(cmd *cobra.Command, _ []string) error {
	out := output.Default

	site, apiKey, err := resolveCredentials(cmd)
	if err != nil {
		return err
	}

	forwardTo, _ := cmd.Flags().GetString("forward-to")
	eventTypes, _ := cmd.Flags().GetStringSlice("events")
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
	printJSON, _ := cmd.Flags().GetBool("print-json")
	sinceStr, _ := cmd.Flags().GetString("since")
	headerFlags, _ := cmd.Flags().GetStringSlice("headers")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	printSigningKey, _ := cmd.Flags().GetBool("print-signing-key")

	var sinceUnix int64
	if sinceStr != "" {
		sinceUnix, err = timeutil.ParseTimestamp(sinceStr, time.Now())
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
	}

	signingSecret, err := generateSecret(32)
	if err != nil {
		return fmt.Errorf("generating signing secret: %w", err)
	}

	if printSigningKey {
		out.Data("%s", signingSecret)
		return nil
	}

	customHeaders, err := parseHeaders(headerFlags)
	if err != nil {
		return err
	}

	client := api.NewClient(site, apiKey)
	listener := webhook.NewListener(client, pollInterval, eventTypes, sinceUnix)

	var forwarder *webhook.Forwarder
	if forwardTo != "" {
		var opts []webhook.ForwarderOption
		if skipVerify {
			opts = append(opts, webhook.WithSkipVerify(true))
		}
		if len(customHeaders) > 0 {
			opts = append(opts, webhook.WithHeaders(customHeaders))
		}
		forwarder = webhook.NewForwarder(forwardTo, signingSecret, opts...)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if forwardTo != "" {
		out.Status("Ready! Forwarding events from %s to %s", site, forwardTo)
	} else {
		out.Status("Ready! Listening for events on %s", site)
	}
	out.KeyValue("Signing key", signingSecret)
	if len(eventTypes) > 0 {
		out.KeyValue("Filtering", strings.Join(eventTypes, ", "))
	}
	if len(customHeaders) > 0 {
		keys := make([]string, 0, len(customHeaders))
		for k := range customHeaders {
			keys = append(keys, k)
		}
		out.Dim("Custom headers: %s", strings.Join(keys, ", "))
	}
	if skipVerify {
		out.Warning("TLS certificate verification disabled")
	}

	events := listener.Listen(ctx)

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	dim := color.New(color.Faint)

	for ev := range events {
		ts := time.Unix(ev.OccurredAt, 0).Format("2006-01-02 15:04:05")
		_, _ = dim.Fprintf(os.Stderr, "%s", ts)
		_, _ = fmt.Fprintf(os.Stderr, "  -->  %s ", ev.EventType)
		_, _ = dim.Fprintf(os.Stderr, "[%s]", ev.ID)

		if forwarder != nil {
			result := forwarder.Forward(ctx, ev)
			if result.Err != nil {
				_, _ = red.Fprintf(os.Stderr, "  <--  [ERR] %v", result.Err)
			} else if result.StatusCode >= 200 && result.StatusCode < 300 {
				_, _ = green.Fprintf(os.Stderr, "  <--  [%d]", result.StatusCode)
			} else {
				_, _ = red.Fprintf(os.Stderr, "  <--  [%d]", result.StatusCode)
			}
		}

		_, _ = fmt.Fprintln(os.Stderr)

		if printJSON {
			formatted, err := json.MarshalIndent(json.RawMessage(ev.Webhook), "", "  ")
			if err != nil {
				_, _ = fmt.Fprintln(os.Stdout, string(ev.Webhook))
			} else {
				_, _ = fmt.Fprintln(os.Stdout, string(formatted))
			}
		}
	}

	out.Dim("\nDone.")
	return nil
}

// parseHeaders parses "Key:Value" strings into a map.
func parseHeaders(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	headers := make(map[string]string, len(raw))
	for _, h := range raw {
		key, value, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected Key:Value format", h)
		}
		headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return headers, nil
}

func generateSecret(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
