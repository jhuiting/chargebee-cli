package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jhuiting/chargebee-cli/internal/api"
)

// Event represents a single Chargebee webhook event.
type Event struct {
	ID         string          `json:"id"`
	EventType  string          `json:"event_type"`
	OccurredAt int64           `json:"occurred_at"`
	Content    json.RawMessage `json:"content"`
	Webhook    json.RawMessage `json:"-"`
}

// eventWrapper matches the Chargebee API list response shape.
type eventWrapper struct {
	Event Event `json:"event"`
}

// listResponse matches the top-level Chargebee list API response.
type listResponse struct {
	List       []eventWrapper `json:"list"`
	NextOffset string         `json:"next_offset,omitempty"`
}

// Listener polls the Chargebee Events API and delivers new events on a channel.
type Listener struct {
	client      *api.Client
	interval    time.Duration
	eventTypes  []string
	lastSeenAt  int64
	lastSeenIDs map[string]bool
}

// NewListener creates a Listener that polls the Chargebee Events API.
// The sinceUnix parameter sets the initial occurred_at lower bound (0 means now).
func NewListener(client *api.Client, interval time.Duration, eventTypes []string, sinceUnix int64) *Listener {
	if sinceUnix == 0 {
		sinceUnix = time.Now().Unix()
	}
	return &Listener{
		client:      client,
		interval:    interval,
		eventTypes:  eventTypes,
		lastSeenAt:  sinceUnix,
		lastSeenIDs: make(map[string]bool),
	}
}

// Listen polls for events and sends them to the returned channel.
// It blocks until the context is cancelled.
func (l *Listener) Listen(ctx context.Context) <-chan Event {
	ch := make(chan Event, 64)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := l.pollAll(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					continue
				}
				for _, ev := range events {
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch
}

// pollAll fetches all new events, following pagination if needed.
func (l *Listener) pollAll(ctx context.Context) ([]Event, error) {
	var allEvents []Event
	offset := ""

	for {
		events, nextOffset, err := l.pollPage(ctx, offset)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, events...)

		if nextOffset == "" {
			break
		}
		offset = nextOffset
	}

	return allEvents, nil
}

// pollPage fetches a single page of events from the Chargebee API.
func (l *Listener) pollPage(ctx context.Context, offset string) ([]Event, string, error) {
	params := url.Values{}
	params.Set("limit", "100")
	params.Set("sort_by[asc]", "occurred_at")
	// Use [gte] (>=) instead of [after] (>) to avoid missing events
	// at the exact same-second boundary. ID dedup handles duplicates.
	params.Set("occurred_at[after]", strconv.FormatInt(l.lastSeenAt-1, 10))

	if len(l.eventTypes) > 0 {
		// Chargebee expects a single comma-separated value for [in] filters.
		params.Set("event_type[in]", strings.Join(l.eventTypes, ","))
	}

	if offset != "" {
		params.Set("offset", offset)
	}

	result, err := l.client.Get(ctx, "/events", params)
	if err != nil {
		return nil, "", fmt.Errorf("polling events: %w", err)
	}

	var resp listResponse
	if err := result.Decode(&resp); err != nil {
		return nil, "", fmt.Errorf("decoding events response: %w", err)
	}

	var events []Event
	for _, wrapper := range resp.List {
		ev := wrapper.Event
		if l.lastSeenIDs[ev.ID] {
			continue
		}

		raw, err := json.Marshal(wrapper)
		if err != nil {
			return nil, "", fmt.Errorf("marshaling event payload: %w", err)
		}
		ev.Webhook = raw

		events = append(events, ev)

		if ev.OccurredAt >= l.lastSeenAt {
			l.lastSeenAt = ev.OccurredAt
		}
	}

	// Rebuild the dedup set for the current timestamp boundary.
	l.lastSeenIDs = make(map[string]bool)
	for _, wrapper := range resp.List {
		if wrapper.Event.OccurredAt >= l.lastSeenAt {
			l.lastSeenIDs[wrapper.Event.ID] = true
		}
	}

	return events, resp.NextOffset, nil
}
