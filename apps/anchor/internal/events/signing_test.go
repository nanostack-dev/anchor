package events_test

import (
	"testing"
	"time"

	"anchor/internal/events"
)

func TestSignAndVerify(t *testing.T) {
	t.Parallel()

	secret, err := events.NewSigningSecret()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(
		`{"type":"organization.created","timestamp":"2026-09-01T00:00:00Z","data":{"organization_id":"org_1"}}`,
	)
	msgID := "pevt_test"
	now := time.Now()

	headers, err := events.Sign(secret, msgID, now, body)
	if err != nil {
		t.Fatal(err)
	}
	if verifyErr := events.Verify(secret, headers, body); verifyErr != nil {
		t.Fatalf("verify: %v", verifyErr)
	}
	if verifyErr := events.Verify(secret, headers, []byte(`{"tampered":true}`)); verifyErr == nil {
		t.Fatal("tampered body must fail verification")
	}
}
