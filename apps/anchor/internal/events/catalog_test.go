package events_test

import (
	"testing"

	"anchor/internal/events"
)

func TestKnownTypes(t *testing.T) {
	t.Parallel()

	if !events.OrganizationCreated.Known() {
		t.Fatal("organization.created must be in the catalog")
	}
	if events.Type("organization.invited").Known() {
		t.Fatal("unknown types must be rejected")
	}
}
