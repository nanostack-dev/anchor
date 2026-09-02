package events_test

import (
	"context"
	"testing"

	"anchor/internal/domain/integration"
	"anchor/internal/events"
	"anchor/internal/integration/provider"

	"github.com/rs/zerolog"
)

type mockWebhookProvider struct{}

func (mockWebhookProvider) Type() string                                 { return "MOCK_WH" }
func (mockWebhookProvider) LatestConfigVersion() int32                   { return 1 }
func (mockWebhookProvider) ValidateConfig(context.Context, []byte) error { return nil }
func (mockWebhookProvider) MigrateConfig(context.Context, int32, []byte) ([]byte, error) {
	return nil, nil
}
func (mockWebhookProvider) ValidateWebhook(context.Context, []byte, map[string]string, string) error {
	return nil
}
func (mockWebhookProvider) ExtractEventType([]byte) string { return "" }
func (mockWebhookProvider) ParseEvent(context.Context, string, []byte) (any, error) {
	return nil, nil
}
func (mockWebhookProvider) ToStandardsCommands(context.Context, string, any) ([]provider.Command, error) {
	return nil, nil
}
func (mockWebhookProvider) ExecuteCommand(
	context.Context, zerolog.Logger, *integration.Instance, provider.Command,
) error {
	return nil
}
func (mockWebhookProvider) WebhookEvents() []provider.WebhookEvent {
	return []provider.WebhookEvent{
		{
			Type:        "mock.item.created",
			Name:        "Mock Item Created",
			Description: "Emitted when a mock item is created.",
		},
	}
}

type mockNonWebhookProvider struct{}

func (mockNonWebhookProvider) Type() string                                 { return "MOCK_OUTBOUND_ONLY" }
func (mockNonWebhookProvider) LatestConfigVersion() int32                   { return 1 }
func (mockNonWebhookProvider) ValidateConfig(context.Context, []byte) error { return nil }
func (mockNonWebhookProvider) MigrateConfig(context.Context, int32, []byte) ([]byte, error) {
	return nil, nil
}

func TestKnownTypes(t *testing.T) {
	t.Parallel()

	if !events.OrganizationCreated.Known() {
		t.Fatal("organization.created must be in the catalog")
	}
	if events.Type("organization.invited").Known() {
		t.Fatal("unknown types must be rejected")
	}
	if len(events.Types()) == 0 {
		t.Fatal("catalog must not be empty")
	}
	for _, eventType := range events.Types() {
		if !eventType.Known() {
			t.Fatalf("%s must be known", eventType)
		}
	}
}

func TestCatalogRegistrationAndGrouping(t *testing.T) {
	t.Parallel()

	domainReg := events.RegisterDomain(
		events.Definition{
			Type:        "test.resource.created",
			Name:        "Test Resource Created",
			Description: "Emitted when test resource is created",
			GroupType:   events.GroupTypeTheme,
			GroupName:   "Test Domain",
			Theme:       "Test Domain",
		},
	)

	whProvider := mockWebhookProvider{}
	nonWhProvider := mockNonWebhookProvider{}

	cat := events.NewCatalog(events.CatalogParams{
		DomainRegistrations: []events.DomainRegistration{domainReg},
		Providers:           []provider.Provider{whProvider, nonWhProvider},
	})

	if !cat.IsKnown("test.resource.created") {
		t.Fatal("test.resource.created should be known in registered catalog")
	}
	if !cat.IsKnown("mock.item.created") {
		t.Fatal("mock.item.created should be registered because provider implements WebhookIngestor")
	}
	if cat.IsKnown("unknown.event") {
		t.Fatal("unknown.event should not be known")
	}

	defs := cat.All()
	var foundWebhookProvider bool
	var foundNonWebhookProvider bool
	for _, d := range defs {
		if d.Integration == "MOCK_WH" {
			foundWebhookProvider = true
			if d.GroupType != events.GroupTypeIntegration {
				t.Fatalf("expected GroupTypeIntegration, got %s", d.GroupType)
			}
		}
		if d.Integration == "MOCK_OUTBOUND_ONLY" {
			foundNonWebhookProvider = true
		}
	}

	if !foundWebhookProvider {
		t.Fatal("mock webhook provider events must be in catalog")
	}
	if foundNonWebhookProvider {
		t.Fatal("provider without WebhookIngestor must not appear in catalog")
	}
}
