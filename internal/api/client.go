package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jhuiting/chargebee-cli/internal/version"
)

const (
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
)

// Client is an HTTP client for the Chargebee API.
type Client struct {
	Site       string
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string
}

// NewClient creates a Chargebee API client for the given site and key.
func NewClient(site, apiKey string) *Client {
	return &Client{
		Site:   site,
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
		BaseURL: fmt.Sprintf("https://%s.chargebee.com/api/v2", site),
	}
}

// Result wraps a raw Chargebee API JSON response.
type Result struct {
	raw     json.RawMessage
	Headers http.Header
	Status  string
}

// JSON returns the raw JSON bytes.
func (r *Result) JSON() json.RawMessage { return r.raw }

// Decode unmarshals the result into the given target.
func (r *Result) Decode(target any) error {
	return json.Unmarshal(r.raw, target)
}

// Get performs a GET request to the given path with optional query parameters.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (*Result, error) {
	reqURL := c.BaseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	buildReq := func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	}

	return c.doWithRetry(ctx, buildReq)
}

// doWithRetry executes a request with automatic retry on 429 rate limit responses.
// It accepts a request builder function so a fresh request can be created on each
// retry attempt.
func (c *Client) doWithRetry(ctx context.Context, buildReq func() (*http.Request, error)) (*Result, error) {
	var lastErr error

	for attempt := range maxRetries {
		req, err := buildReq()
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		result, err := c.do(req)
		if err == nil {
			return result, nil
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
			return nil, err
		}

		lastErr = err
		backoff := retryBackoff(attempt, apiErr.RetryAfter)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("rate limited after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) do(req *http.Request) (*Result, error) {
	req.SetBasicAuth(c.APIKey, "")
	req.Header.Set("User-Agent", "chargebee-cli/"+version.Version)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp.StatusCode, resp.Header, data)
	}

	return &Result{raw: data, Headers: resp.Header, Status: resp.Status}, nil
}

// retryBackoff calculates the backoff duration with exponential backoff + jitter.
// If retryAfterSecs >= 0 (from Retry-After header), that value is used directly.
// Otherwise, exponential backoff is applied.
func retryBackoff(attempt int, retryAfterSecs int) time.Duration {
	if retryAfterSecs >= 0 {
		return time.Duration(retryAfterSecs) * time.Second
	}
	base := time.Duration(1<<min(attempt, 4)) * time.Second //nolint:gosec // attempt is bounded by maxRetries
	//nolint:gosec // jitter doesn't need cryptographic randomness
	jitter := time.Duration(rand.IntN(1000)) * time.Millisecond
	return base + jitter
}

// APIError represents an error returned by the Chargebee API.
type APIError struct {
	StatusCode int
	RetryAfter int
	Message    string `json:"message"`
	ErrorCode  string `json:"error_code"`
	ErrorMsg   string `json:"error_msg"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.ErrorMsg != "" {
		return fmt.Sprintf("Chargebee API error (%d): [%s] %s", e.StatusCode, e.ErrorCode, e.ErrorMsg)
	}
	return fmt.Sprintf("Chargebee API error (%d): %s", e.StatusCode, e.Message)
}

func parseAPIError(statusCode int, headers http.Header, data []byte) error {
	apiErr := &APIError{StatusCode: statusCode, RetryAfter: -1}
	if err := json.Unmarshal(data, apiErr); err != nil {
		apiErr.Message = string(data)
	}
	if ra := headers.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			apiErr.RetryAfter = secs
		}
	}
	return apiErr
}
