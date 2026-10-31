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

func TestDeliveredData(t *testing.T) {
	t.Parallel()

	t.Run("A delivered event leaves out Anchor's resource row ID and keeps every other field", func(t *testing.T) {
		t.Parallel()
		data := events.Data{
			events.FieldOrganizationID: "org_1",
			events.FieldResourceID:     "ext-1",
			events.FieldResourceRowID:  "res_1",
		}

		delivered := events.DeliveredDataForTest(data)

		require.Equal(t, events.Data{events.FieldOrganizationID: "org_1", events.FieldResourceID: "ext-1"}, delivered)
		require.Equal(t, "res_1", data[events.FieldResourceRowID], "the observers still read it")
	})
}

type callLog struct{ calls []string }

type alphaObserver struct{ log *callLog }

func (o alphaObserver) ObserveTx(context.Context, events.Event) error {
	o.log.calls = append(o.log.calls, "alpha")
	return nil
}

type betaObserver struct{ log *callLog }

func (o betaObserver) ObserveTx(context.Context, events.Event) error {
	o.log.calls = append(o.log.calls, "beta")
	return nil
}

func TestEmitterRunsObserversInOneOrderWhateverTheGroupOrder(t *testing.T) {
	t.Parallel()

	t.Run("Observers given in either order run in the same order", func(t *testing.T) {
		t.Parallel()
		catalog := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{
			events.RegisterInternal("Test", events.Definition{Type: "test.kept", Undelivered: true}),
		}})
		ctx := transactor.WithTx(context.Background(), &sql.Tx{})
		event := events.Event{Type: "test.kept", ProductID: "prd_test", Data: events.Data{"id": "1"}}

		forward, backward := &callLog{}, &callLog{}
		require.NoError(t, events.NewEmitter(nil, catalog,
			alphaObserver{forward}, betaObserver{forward}).Emit(ctx, event))
		require.NoError(t, events.NewEmitter(nil, catalog,
			betaObserver{backward}, alphaObserver{backward}).Emit(ctx, event))

		require.Equal(t, []string{"alpha", "beta"}, forward.calls)
		require.Equal(t, forward.calls, backward.calls)
	})
}
