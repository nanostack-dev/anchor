package ct_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
)

func TestOwnerContext(t *testing.T) {
	t.Run(
		"ContextGeneration", func(t *testing.T) {
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
