package events_test

import (
	"testing"

	"anchor/internal/events"
)

func TestCatalogRegistrationAndGrouping(t *testing.T) {
	t.Parallel()

	domainReg := events.RegisterDomain(
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
