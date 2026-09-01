package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
)

const (
	eventQueueName        = "anchor.events"
	eventQueueMaxAttempts = 6
)

var errEmitRequiresTx = errors.New("events: Emit requires transactor.InTx")

type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Emitter struct {
	queue *queue.Client
}

func (e *Emitter) Emit(ctx context.Context, event Event) error {
	tx := transactor.CurrentTx(ctx)
	if tx == nil {
		return errEmitRequiresTx
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = e.queue.EnqueueTx(ctx, tx, queue.EnqueueParams{
		QueueName:   eventQueueName,
		Payload:     payload,
		MaxAttempts: eventQueueMaxAttempts,
	})
	return err
}
