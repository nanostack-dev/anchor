package events_test

import (
	"context"
	"testing"

	"anchor/internal/events"

	"github.com/stretchr/testify/require"
)

func TestEmitterAcceptsRegisteredType(t *testing.T) {
	t.Parallel()

	catalog := events.NewCatalog(events.CatalogParams{
		DomainRegistrations: []events.DomainRegistration{
			events.RegisterDomain(events.Definition{Type: "custom.created"}),
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
