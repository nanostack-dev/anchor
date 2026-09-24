package events_test

import (
	"testing"

	"anchor/internal/events"
)

func TestValidateEndpointURL(t *testing.T) {
	t.Parallel()

	err := events.ValidateEndpointURLForTest("not-a-url", false)
	if err == nil {
		t.Fatal("invalid URL must be rejected")
	}
	err = events.ValidateEndpointURLForTest("http://example.com/hooks", true)
	if err == nil {
		t.Fatal("production must reject HTTP")
	}
	err = events.ValidateEndpointURLForTest("http://127.0.0.1:9/hooks", false)
	if err != nil {
		t.Fatalf("non-production HTTP must be allowed: %v", err)
	}
	err = events.ValidateEndpointURLForTest("https://example.com/hooks", true)
	if err != nil {
		t.Fatalf("production HTTPS must be allowed: %v", err)
	}
	err = events.ValidateEndpointURLForTest("http://169.254.169.254/latest", false)
	if err == nil {
		t.Fatal("link-local metadata address must be rejected")
	}
}

func TestDeliveryTargetAllows(t *testing.T) {
	t.Parallel()

	// Invalid or absent subscriptions must never expand delivery scope.
	noSubscriptions := events.DeliveryTarget{
		URL:    "https://example.com",
		Secret: "whsec_test",
		Events: nil,
	}
	if noSubscriptions.Allows(events.OrganizationCreated) {
		t.Fatal("nil events must allow no events")
	}

	// Filtered events only allows listed event types.
	filtered := events.DeliveryTarget{
		URL:    "https://example.com",
		Secret: "whsec_test",
		Events: []string{string(events.OrganizationCreated), string(events.OrganizationDeleted)},
	}
	if !filtered.Allows(events.OrganizationCreated) {
		t.Fatal("organization.created must be allowed")
	}
	if !filtered.Allows(events.OrganizationDeleted) {
		t.Fatal("organization.deleted must be allowed")
	}
	if filtered.Allows(events.OrganizationUpdated) {
		t.Fatal("organization.updated must not be allowed")
	}
	if filtered.Allows(events.WorkspaceCreated) {
		t.Fatal("workspace.created must not be allowed")
	}
}
