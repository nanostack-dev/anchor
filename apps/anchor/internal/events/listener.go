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
// one for a write a workflow made in reaction to it, and so on. Chain names
// those workflows, oldest first.
type QueuedEvent struct {
	EventID   string          `json:"event_id"`
	ProductID string          `json:"product_id"`
	Type      Type            `json:"type"`
	Body      json.RawMessage `json:"body"`
	Depth     int             `json:"depth,omitempty"`
	Chain     []string        `json:"chain,omitempty"`
}

// EncodeEvent builds the queue payload of an event, carrying the causation
// found in ctx.
func EncodeEvent(
	ctx context.Context,
	eventID, productID string,
	eventType Type,
	data Data,
	at time.Time,
) ([]byte, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(Envelope{
		Type:      eventType,
		Timestamp: at.UTC().Format(time.RFC3339Nano),
		Data:      dataJSON,
	})
	if err != nil {
		return nil, err
	}
	causation := CausationFrom(ctx)
	return json.Marshal(QueuedEvent{
		EventID:   eventID,
		ProductID: productID,
		Type:      eventType,
		Body:      body,
		Depth:     causation.Depth,
		Chain:     causation.WorkflowIDs,
	})
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

// Causation is what led to the writes made under a context: how many
// workflow runs, and which workflows, oldest first. Every event emitted under
// it carries it, so a workflow can refuse to run twice in one chain.
type Causation struct {
	Depth       int      `json:"depth"`
	WorkflowIDs []string `json:"workflow_ids"`
}

type causationKey struct{}

func WithCausation(ctx context.Context, causation Causation) context.Context {
	return context.WithValue(ctx, causationKey{}, causation)
}

func CausationFrom(ctx context.Context) Causation {
	causation, _ := ctx.Value(causationKey{}).(Causation)
	return causation
}
