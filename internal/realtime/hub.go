// Package realtime pushes operational events (repair status changes, part
// collections, payment confirmations, ...) to connected browsers over
// Server-Sent Events, so staff screens update live instead of needing a
// manual refresh. It piggybacks on the existing in-process events.Bus, so no
// new infrastructure (Redis pub/sub, websocket server, etc.) is required.
package realtime

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/authz"
	"github.com/techlane/techlane/packages/pkg/events"
)

const (
	subscriberBuffer = 32
	heartbeatEvery   = 20 * time.Second
)

type Hub struct {
	jwtSecret string

	mu   sync.Mutex
	subs map[uuid.UUID]map[chan []byte]struct{} // tenantID -> subscriber channels
}

func NewHub(jwtSecret string) *Hub {
	return &Hub{jwtSecret: jwtSecret, subs: map[uuid.UUID]map[chan []byte]struct{}{}}
}

// Broadcast is meant to be wired as an events.Bus subscriber:
// bus.Subscribe("*", hub.Broadcast)
func (h *Hub) Broadcast(e events.Envelope) {
	msg, err := json.Marshal(struct {
		EventType  string         `json:"event_type"`
		OccurredAt time.Time      `json:"occurred_at"`
		BranchID   *uuid.UUID     `json:"branch_id,omitempty"`
		Payload    map[string]any `json:"payload"`
	}{
		EventType:  e.EventType,
		OccurredAt: e.OccurredAt,
		BranchID:   e.BranchID,
		Payload:    e.Payload,
	})
	if err != nil {
		return
	}

	h.mu.Lock()
	subs := h.subs[e.TenantID]
	targets := make([]chan []byte, 0, len(subs))
	for ch := range subs {
		targets = append(targets, ch)
	}
	h.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- msg:
		default:
			// Slow/stuck subscriber — drop the event rather than block publishers.
		}
	}
}

func (h *Hub) subscribe(tenantID uuid.UUID) chan []byte {
	ch := make(chan []byte, subscriberBuffer)
	h.mu.Lock()
	if h.subs[tenantID] == nil {
		h.subs[tenantID] = map[chan []byte]struct{}{}
	}
	h.subs[tenantID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribe(tenantID uuid.UUID, ch chan []byte) {
	h.mu.Lock()
	delete(h.subs[tenantID], ch)
	if len(h.subs[tenantID]) == 0 {
		delete(h.subs, tenantID)
	}
	h.mu.Unlock()
}

// ServeSSE streams events scoped to the caller's tenant. Browsers'
// EventSource API can't set an Authorization header, so the access token may
// also arrive as ?token=; both are accepted.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = authz.BearerToken(r.Header.Get("Authorization"))
	}
	claims, err := authz.ParseAccessToken(h.jwtSecret, token)
	if err != nil || claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ch := h.subscribe(claims.TenantID)
	defer h.unsubscribe(claims.TenantID, ch)

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write([]byte("data: ")); err != nil {
				return
			}
			if _, err := w.Write(msg); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// SubscriberCount reports how many tenants currently have at least one
// connected client — useful for a quick sanity check in logs/health checks.
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := 0
	for _, subs := range h.subs {
		total += len(subs)
	}
	return total
}
