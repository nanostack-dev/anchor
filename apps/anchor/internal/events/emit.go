package events

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
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

type emitter struct {
	queue     *queue.Client
	catalog   Catalog
	observers []TxObserver
	now       func() time.Time
}

// NewEmitter runs the observers in the order of their type names. fx handed a
// value group over in a random order per process, and observers take locks:
// two replicas running them in opposite orders deadlock their writes
// ([postmortem](../../../../docs/postmortems/2026-10-31-tx-observer-order-deadlock.md)).
func NewEmitter(queueClient *queue.Client, catalog Catalog, observers ...TxObserver) Emitter {
	ordered := slices.Clone(observers)
	slices.SortStableFunc(ordered, func(a, b TxObserver) int {
		return cmp.Compare(fmt.Sprintf("%T", a), fmt.Sprintf("%T", b))
	})
	return &emitter{
		queue:     queueClient,
		catalog:   catalog,
		observers: ordered,
		now:       time.Now,
	}
}

// ProvideEmitter hands the emitter every TxObserver that a module adds to the
// nano.Collect[TxObserver] collection.
func ProvideEmitter(queueClient *queue.Client, catalog Catalog, observers []TxObserver) Emitter {
	return NewEmitter(queueClient, catalog, observers...)
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
	dataJSON, err := json.Marshal(deliveredData(event.Data))
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

// deliveredData is what a product endpoint receives: Anchor's resource row ID
// is for the observers only.
func deliveredData(data Data) Data {
	if _, internal := data[FieldResourceRowID]; !internal {
		return data
	}
	delivered := make(Data, len(data)-1)
	for field, value := range data {
		if field != FieldResourceRowID {
			delivered[field] = value
		}
	}
	return delivered
}
