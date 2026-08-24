package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterDoesNotLeakAccountExistence proves that the public registration
// endpoint does not behave as a pre-auth account-existence oracle.
//
// The endpoint is unauthenticated and, on a provisioned system, requires a valid
// invitation code. A caller presenting an invalid invitation code must receive
// the same response whether or not the submitted email already has an account —
// otherwise an attacker can enumerate registered addresses without any
// credential by reading the status code (409 "user already exists" vs 400
// "invitation code invalid").
func TestRegisterDoesNotLeakAccountExistence(t *testing.T) {
	ctx := context.Background()

	// Arrange: register a real account so this email is known to exist.
	existingEmail := itshared.Faker.Internet().Email()
	inviteCode := createPlatformInvitation(t, existingEmail)

	seedResp, err := testTenant(t).NoAuthClient.RegisterWithResponse(ctx, ct.RegisterJSONRequestBody{
		Email:          existingEmail,
		Password:       "ValidPassword123!",
		InvitationCode: &inviteCode,
	})
	require.NoError(t, err, "seed registration should not error")
	require.Equal(t, http.StatusOK, seedResp.StatusCode(), "seed registration should succeed")

	invalidCode := "invalid-code-123"

	// Act: an unauthenticated caller probes a known and an unknown email with the
	// same invalid invitation code.
	knownResp, err := testTenant(t).NoAuthClient.RegisterWithResponse(ctx, ct.RegisterJSONRequestBody{
		Email:          existingEmail,
		Password:       "ValidPassword123!",
		InvitationCode: &invalidCode,
	})
	require.NoError(t, err, "probe of existing email should not error")

	unknownResp, err := testTenant(t).NoAuthClient.RegisterWithResponse(ctx, ct.RegisterJSONRequestBody{
		Email:          itshared.Faker.Internet().Email(),
		Password:       "ValidPassword123!",
		InvitationCode: &invalidCode,
	})
	require.NoError(t, err, "probe of unknown email should not error")

	// Assert: the two responses are indistinguishable. If the known-email probe
	// returns 409 while the unknown-email probe returns 400, the status code
	// leaks account existence.
	assert.Equal(t, unknownResp.StatusCode(), knownResp.StatusCode(),
		"registration with an invalid invitation code must not reveal whether the email already exists")
	assert.Equal(t, http.StatusBadRequest, knownResp.StatusCode(),
		"an invalid invitation code should be rejected before email existence is checked")
}
