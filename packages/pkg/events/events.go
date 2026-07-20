package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	EventID       uuid.UUID      `json:"event_id"`
	EventType     string         `json:"event_type"`
	EventVersion  int            `json:"event_version"`
	OccurredAt    time.Time      `json:"occurred_at"`
	TenantID      uuid.UUID      `json:"tenant_id"`
	BranchID      *uuid.UUID     `json:"branch_id,omitempty"`
	CorrelationID uuid.UUID      `json:"correlation_id"`
	ActorID       *uuid.UUID     `json:"actor_id,omitempty"`
	Payload       map[string]any `json:"payload"`
}

func New(eventType string, tenantID uuid.UUID, correlationID uuid.UUID, payload map[string]any) Envelope {
	return Envelope{
		EventID:       uuid.New(),
		EventType:     eventType,
		EventVersion:  1,
		OccurredAt:    time.Now().UTC(),
		TenantID:      tenantID,
		CorrelationID: correlationID,
		Payload:       payload,
	}
}

func (e Envelope) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// Bus is a minimal in-process event bus for local/dev; replace with RabbitMQ publisher in deploy.
type Bus struct {
	handlers map[string][]func(Envelope)
}

func NewBus() *Bus {
	return &Bus{handlers: map[string][]func(Envelope){}}
}

func (b *Bus) Subscribe(eventType string, h func(Envelope)) {
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

func (b *Bus) Publish(e Envelope) {
	for _, h := range b.handlers[e.EventType] {
		h(e)
	}
	for _, h := range b.handlers["*"] {
		h(e)
	}
}
