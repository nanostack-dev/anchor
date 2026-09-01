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
	if len(events.Types()) == 0 {
		t.Fatal("catalog must not be empty")
	}
	for _, eventType := range events.Types() {
		if !eventType.Known() {
			t.Fatalf("%s must be known", eventType)
		}
	}
}
