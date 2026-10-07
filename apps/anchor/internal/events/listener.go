package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/fx"
)

// Listener is an in-process consumer of product events. Emit enqueues a job
// on the listener's queue for every event it accepts, in the same transaction
// as the write that produced it, next to the job that delivers the event to
// the Product's endpoint. Accepts runs inside that transaction.
type Listener struct {
	QueueName string
	Accepts   func(ctx context.Context, productID string, eventType Type) (bool, error)
}

func AsListener(fn any) any {
	return fx.Annotate(fn, fx.ResultTags(`group:"product_event_listeners"`))
}

// QueuedEvent is the job payload of every product event queue. Depth counts
// how many workflow runs led to the event: zero for a write made by a caller,
// one for a write a workflow made in reaction to it, and so on.
type QueuedEvent struct {
	EventID   string          `json:"event_id"`
	ProductID string          `json:"product_id"`
	Type      Type            `json:"type"`
	Body      json.RawMessage `json:"body"`
	Depth     int             `json:"depth,omitempty"`
}

func DecodeQueuedEvent(payload []byte) (QueuedEvent, error) {
	var event QueuedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return QueuedEvent{}, fmt.Errorf("events: decode job: %w", err)
	}
	if event.Type == "" {
		var env Envelope
		if err := json.Unmarshal(event.Body, &env); err == nil {
			event.Type = env.Type
		}
	}
	return event, nil
}

// OccurredAt is when the event was emitted, on the emitting process's clock.
func (e QueuedEvent) OccurredAt() (time.Time, error) {
	var env Envelope
	if err := json.Unmarshal(e.Body, &env); err != nil {
		return time.Time{}, fmt.Errorf("events: decode envelope: %w", err)
	}
	return time.Parse(time.RFC3339Nano, env.Timestamp)
}

func (e QueuedEvent) Data() (Data, error) {
	var env Envelope
	if err := json.Unmarshal(e.Body, &env); err != nil {
		return nil, fmt.Errorf("events: decode envelope: %w", err)
	}
	data := Data{}
	if len(env.Data) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, fmt.Errorf("events: decode data: %w", err)
	}
	return data, nil
}

type causationDepthKey struct{}

func WithCausationDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, causationDepthKey{}, depth)
}

func CausationDepth(ctx context.Context) int {
	depth, _ := ctx.Value(causationDepthKey{}).(int)
	return depth
}
