package ct_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
)

func TestOwnerContext(t *testing.T) {
	// Owner credentials create an authenticated context for the tenant owner.
	t.Parallel()
	t.Run(
		"ContextGeneration", func(t *testing.T) {
			// A new owner context contains the tenant, owner, and authenticated client.
			assert.NotNil(
				t, testOwnerUser(t),
				"owner user should be initialized",
			)
			assert.NotNil(
				t, testTenant(t), "tenant context should be initialized",
			)
			assert.NotNil(t, itshared.TestLogger, "TestLogger should be initialized")
		},
	)

	t.Run(
		"OwnerCredentials", func(t *testing.T) {
			// The generated owner's credentials authenticate successfully.
			assert.NotEmpty(
				t, testOwnerUser(t).AccessToken, "owner should have access token",
			)
			assert.NotEmpty(
				t, testOwnerUser(t).RefreshToken, "owner should have refresh token",
			)
			assert.NotEmpty(t, testOwnerUser(t).Email, "owner should have email")
		},
	)
}
