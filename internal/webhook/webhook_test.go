package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jhuiting/chargebee-cli/internal/webhook"
)

func TestForwarderSendsCorrectHeaders(t *testing.T) {
	secret := "test-secret-123"

	var receivedHeaders http.Header
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	forwarder := webhook.NewForwarder(server.URL, secret)

	event := webhook.Event{
		ID:         "ev_test_123",
		EventType:  "subscription_created",
		OccurredAt: time.Now().Unix(),
		Webhook:    json.RawMessage(`{"event":{"id":"ev_test_123"}}`),
	}

	result := forwarder.Forward(context.Background(), event)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	if receivedHeaders.Get("X-Chargebee-Event-Type") != "subscription_created" {
		t.Errorf("expected event type header, got %q", receivedHeaders.Get("X-Chargebee-Event-Type"))
	}
	if receivedHeaders.Get("X-Chargebee-Event-Id") != "ev_test_123" {
		t.Errorf("expected event id header, got %q", receivedHeaders.Get("X-Chargebee-Event-Id"))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected json content type, got %q", receivedHeaders.Get("Content-Type"))
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(receivedBody)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if receivedHeaders.Get("X-CB-CLI-Signature") != expectedSig {
		t.Error("HMAC signature mismatch")
	}
}

func TestForwarderHandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	forwarder := webhook.NewForwarder(server.URL, "secret")

	event := webhook.Event{
		ID:        "ev_err",
		EventType: "test_event",
		Webhook:   json.RawMessage(`{}`),
	}

	result := forwarder.Forward(context.Background(), event)
	if result.Err != nil {
		t.Fatalf("unexpected transport error: %v", result.Err)
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", result.StatusCode)
	}
}

func TestForwarderHandlesConnectionError(t *testing.T) {
	forwarder := webhook.NewForwarder("http://localhost:1", "secret")

	event := webhook.Event{
		ID:        "ev_conn",
		EventType: "test_event",
		Webhook:   json.RawMessage(`{}`),
	}

	result := forwarder.Forward(context.Background(), event)
	if result.Err == nil {
		t.Error("expected error for connection failure")
	}
}
