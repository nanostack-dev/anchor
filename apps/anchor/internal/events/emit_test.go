package events_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/events"

	"github.com/stretchr/testify/require"
)

func TestEmitterAcceptsRegisteredType(t *testing.T) {
	t.Parallel()

	catalog := events.NewCatalog(events.CatalogParams{
		Registrations: []events.Registration{
			events.RegisterInternal("Test", events.Definition{Type: "custom.created"}),
		},
	})
	emitter := events.NewEmitter(nil, catalog)
	event := events.Event{
		Type:      "custom.created",
		ProductID: "prd_test",
		Data:      events.Data{"id": "test"},
	}

	err := emitter.Emit(context.Background(), event)
	require.ErrorContains(t, err, "requires transactor.InTx")

	event.Type = "unknown.created"
	err = emitter.Emit(context.Background(), event)
	require.ErrorContains(t, err, "not in the product event catalog")
}

type recordingObserver struct{ seen []events.Type }

func (o *recordingObserver) ObserveTx(_ context.Context, event events.Event) error {
	o.seen = append(o.seen, event.Type)
	return nil
}

func TestEmitterKeepsUndeliveredEventsInside(t *testing.T) {
	t.Parallel()

	t.Run("An undelivered event reaches the observers and is never queued", func(t *testing.T) {
		t.Parallel()
		catalog := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{
			events.RegisterInternal("Test", events.Definition{Type: "test.kept", Undelivered: true}),
		}})
		observer := &recordingObserver{}
		// No queue: a delivery would dereference it.
		emitter := events.NewEmitter(nil, catalog, observer)
		ctx := transactor.WithTx(context.Background(), &sql.Tx{})

		err := emitter.Emit(ctx, events.Event{Type: "test.kept", ProductID: "prd_test", Data: events.Data{"id": "1"}})

		require.NoError(t, err)
		require.Equal(t, []events.Type{"test.kept"}, observer.seen)
	})
}
