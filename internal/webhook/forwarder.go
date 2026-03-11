package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/jhuiting/chargebee-cli/internal/version"
)

// ForwardResult holds the outcome of a single event forward attempt.
type ForwardResult struct {
	StatusCode int
	Status     string
	Err        error
}

// Forwarder POSTs webhook events to a local HTTP endpoint.
type Forwarder struct {
	url     string
	secret  string
	headers map[string]string
	client  *http.Client
}

// ForwarderOption configures a Forwarder.
type ForwarderOption func(*Forwarder)

// WithSkipVerify disables TLS certificate verification.
func WithSkipVerify(skip bool) ForwarderOption {
	return func(f *Forwarder) {
		if skip {
			f.client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, //nolint:gosec // user explicitly opted in via --skip-verify
			}
		}
	}
}

// WithHeaders adds custom HTTP headers to forwarded requests.
func WithHeaders(headers map[string]string) ForwarderOption {
	return func(f *Forwarder) {
		f.headers = headers
	}
}

// NewForwarder creates a Forwarder that sends events to the given URL,
// signing payloads with the provided HMAC-SHA256 secret.
func NewForwarder(targetURL, signingSecret string, opts ...ForwarderOption) *Forwarder {
	f := &Forwarder{
		url:    targetURL,
		secret: signingSecret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Forward sends an event to the configured local endpoint and returns the result.
func (f *Forwarder) Forward(ctx context.Context, ev Event) ForwardResult {
	body := ev.Webhook

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
	if err != nil {
		return ForwardResult{Err: fmt.Errorf("creating forward request: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Chargebee-Event-Type", ev.EventType)
	req.Header.Set("X-Chargebee-Event-Id", ev.ID)
	req.Header.Set("X-CB-CLI-Signature", sign(body, f.secret))
	req.Header.Set("User-Agent", "chargebee-cli/"+version.Version)

	for k, v := range f.headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return ForwardResult{Err: fmt.Errorf("forwarding event %s: %w", ev.ID, err)}
	}
	defer func() { _ = resp.Body.Close() }()

	return ForwardResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}

func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
