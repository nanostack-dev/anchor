package ct_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
)

func TestOwnerContext(t *testing.T) {
	t.Parallel()
	t.Run(
		"A new owner context contains the tenant, owner, and authenticated client", func(t *testing.T) {
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
		"The generated owner's credentials authenticate successfully", func(t *testing.T) {
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
