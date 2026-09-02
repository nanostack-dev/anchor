package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/nanostack-dev/pgkit/queue"
)

const (
	queueName             = "product-events"
	eventIDPrefix         = "pevt"
	eventQueueMaxAttempts = 6
)

type queuePayload struct {
	EventID   string          `json:"event_id"`
	ProductID string          `json:"product_id"`
	Body      json.RawMessage `json:"body"`
}

type Emitter interface {
	Emit(ctx context.Context, event Event) error
}

type emitter struct {
	queue *queue.Client
	now   func() time.Time
}

func NewEmitter(queueClient *queue.Client) Emitter {
	return &emitter{
		queue: queueClient,
		now:   time.Now,
	}
}

func (e *emitter) Emit(ctx context.Context, event Event) error {
	if err := validate.ValidateStruct(event); err != nil {
		return err
	}
	if !event.Type.Known() {
		return unknownTypeError(event.Type)
	}
	tx := transactor.CurrentTx(ctx)
	if tx == nil {
		return errEmitRequiresTx
	}

	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	envelope := Envelope{
		Type:      event.Type,
		Timestamp: e.now().UTC().Format(time.RFC3339Nano),
		Data:      dataJSON,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(queuePayload{
		EventID:   ids.MustNew(eventIDPrefix),
		ProductID: event.ProductID,
		Body:      body,
	})
	if err != nil {
		return err
	}

	_, err = e.queue.EnqueueTx(ctx, tx, queue.EnqueueParams{
		QueueName:   queueName,
		Payload:     payload,
		MaxAttempts: eventQueueMaxAttempts,
	})
	return err
}
