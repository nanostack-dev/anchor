package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/nanostack-dev/pgkit/queue"
	"go.uber.org/fx"
)

const (
	queueName             = "product-events"
	eventIDPrefix         = "pevt"
	eventQueueMaxAttempts = 6
)

type queuePayload struct {
	EventID   string          `json:"event_id"`
	ProductID string          `json:"product_id"`
	Type      Type            `json:"type"`
	Body      json.RawMessage `json:"body"`
}

type Emitter interface {
	Emit(ctx context.Context, event Event) error
}

// TxObserver sees every emitted event inside the transaction that emits it, so
// whatever it records commits or rolls back with the write that caused it. An
// error aborts the write.
type TxObserver interface {
	ObserveTx(ctx context.Context, event Event) error
}

// AsTxObserver annotates a constructor so the emitter receives its observer.
func AsTxObserver(constructor any) any {
	return fx.Annotate(constructor, fx.As(new(TxObserver)), fx.ResultTags(`group:"event_tx_observers"`))
}

type emitter struct {
	queue     *queue.Client
	catalog   Catalog
	observers []TxObserver
	now       func() time.Time
}

func NewEmitter(queueClient *queue.Client, catalog Catalog, observers ...TxObserver) Emitter {
	return &emitter{
		queue:     queueClient,
		catalog:   catalog,
		observers: observers,
		now:       time.Now,
	}
}

type emitterParams struct {
	fx.In
	Queue     *queue.Client
	Catalog   Catalog
	Observers []TxObserver `group:"event_tx_observers"`
}

func provideEmitter(params emitterParams) Emitter {
	return NewEmitter(params.Queue, params.Catalog, params.Observers...)
}

func (e *emitter) Emit(ctx context.Context, event Event) error {
	if err := validate.ValidateStruct(event); err != nil {
		return err
	}
	if !e.catalog.IsKnown(event.Type) {
		return unknownTypeError(event.Type)
	}
	tx := transactor.CurrentTx(ctx)
	if tx == nil {
		return errEmitRequiresTx
	}
	if e.catalog.IsDeliverable(event.Type) {
		if err := e.enqueue(ctx, tx, event); err != nil {
			return err
		}
	}
	for _, observer := range e.observers {
		if err := observer.ObserveTx(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (e *emitter) enqueue(ctx context.Context, tx *sql.Tx, event Event) error {
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
		Type:      event.Type,
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
