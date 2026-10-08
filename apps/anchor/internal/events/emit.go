package events

import (
	"context"
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

type Emitter interface {
	Emit(ctx context.Context, event Event) error
}

type emitter struct {
	queue     *queue.Client
	catalog   Catalog
	listeners []Listener
	now       func() time.Time
}

func NewEmitter(queueClient *queue.Client, catalog Catalog, listeners ...Listener) Emitter {
	return &emitter{
		queue:     queueClient,
		catalog:   catalog,
		listeners: listeners,
		now:       time.Now,
	}
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

	payload, err := EncodeEvent(ctx, ids.MustNew(eventIDPrefix), event.ProductID, event.Type, event.Data, e.now())
	if err != nil {
		return err
	}

	if _, err = e.queue.EnqueueTx(ctx, tx, queue.EnqueueParams{
		QueueName:   queueName,
		Payload:     payload,
		MaxAttempts: eventQueueMaxAttempts,
	}); err != nil {
		return err
	}
	for _, listener := range e.listeners {
		accepted, acceptErr := listener.Accepts(ctx, event.ProductID, event.Type)
		if acceptErr != nil {
			return acceptErr
		}
		if !accepted {
			continue
		}
		if _, err = e.queue.EnqueueTx(ctx, tx, queue.EnqueueParams{
			QueueName:   listener.QueueName,
			Payload:     payload,
			MaxAttempts: eventQueueMaxAttempts,
		}); err != nil {
			return err
		}
	}
	return nil
}
