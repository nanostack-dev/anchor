package events_test

import (
	"testing"

	"anchor/internal/events"
)

func TestCatalogRegistrationAndGrouping(t *testing.T) {
	t.Parallel()

	domainReg := events.RegisterInternal(
		"Test Domain",
		events.Definition{
			Type:        "test.resource.created",
			Name:        "Test Resource Created",
			Description: "Emitted when test resource is created",
		},
	)

	integrationReg := events.RegisterIntegration(
		"MOCK_WH",
		events.Definition{
			Type:        "mock.item.created",
			Name:        "Mock Item Created",
			Description: "Emitted when a mock item is created.",
		},
	)

	cat := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{domainReg, integrationReg}})

	if !cat.IsKnown("test.resource.created") {
		t.Fatal("test.resource.created should be known in registered catalog")
	}
	if !cat.IsKnown("mock.item.created") {
		t.Fatal("mock.item.created should be registered by the integration")
	}
	if cat.IsKnown("unknown.event") {
		t.Fatal("unknown.event should not be known")
	}
	defs := cat.All()
	if len(defs) != 2 {
		t.Fatalf("expected 2 registered definitions, got %d", len(defs))
	}
	var foundWebhookProvider bool
	for _, d := range defs {
		if d.GroupName == "MOCK_WH" {
			foundWebhookProvider = true
			if d.GroupType != events.GroupTypeIntegration {
				t.Fatalf("expected GroupTypeIntegration, got %s", d.GroupType)
			}
		}
	}

	if !foundWebhookProvider {
		t.Fatal("mock webhook provider events must be in catalog")
	}
}

func TestCatalogUndeliveredEvents(t *testing.T) {
	t.Parallel()

	catalog := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{
		events.RegisterInternal("Test",
			events.Definition{Type: "test.delivered"},
			events.Definition{Type: "test.kept", Undelivered: true},
		),
	}})

	t.Run("An undelivered event can be emitted but never delivered", func(t *testing.T) {
		t.Parallel()
		if !catalog.IsKnown("test.kept") {
			t.Fatal("test.kept should be known, so it can be emitted")
		}
		if catalog.IsDeliverable("test.kept") {
			t.Fatal("test.kept should not be deliverable")
		}
		if !catalog.IsDeliverable("test.delivered") {
			t.Fatal("test.delivered should be deliverable")
		}
	})

	t.Run("The catalog lists only the events an endpoint may receive", func(t *testing.T) {
		t.Parallel()
		if got := catalog.AllEventTypesStrings(); len(got) != 1 || got[0] != "test.delivered" {
			t.Fatalf("expected only test.delivered, got %v", got)
		}
		if got := catalog.All(); len(got) != 1 || got[0].Type != "test.delivered" {
			t.Fatalf("expected only test.delivered, got %v", got)
		}
	})
}
