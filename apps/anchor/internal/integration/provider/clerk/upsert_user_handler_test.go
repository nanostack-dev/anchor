//nolint:testpackage // tests the unexported clerkUpsertWouldChange helper
package clerk

import (
	"testing"

	"anchor/internal/domain/product/user"

	"github.com/stretchr/testify/assert"
)

func TestClerkUpsertWouldChange(t *testing.T) {
	t.Parallel()

	existing := user.ProductUser{
		Email:  "ada@example.com",
		Name:   "Ada Lovelace",
		Status: user.ProductUserStatusActive,
	}
	same := UpsertUserData{
		ExternalID: "user_clerk_1",
		Email:      "ada@example.com",
		Name:       "Ada Lovelace",
	}

	t.Run("SameEmailNameAndActiveStatus", func(t *testing.T) {
		t.Parallel()
		assert.False(t, clerkUpsertWouldChange(existing, same))
	})

	t.Run("EmailChanged", func(t *testing.T) {
		t.Parallel()
		data := same
		data.Email = "countess@example.com"
		assert.True(t, clerkUpsertWouldChange(existing, data))
	})

	t.Run("NameChanged", func(t *testing.T) {
		t.Parallel()
		data := same
		data.Name = "Ada King"
		assert.True(t, clerkUpsertWouldChange(existing, data))
	})

	t.Run("InactiveExistingUser", func(t *testing.T) {
		t.Parallel()
		inactive := existing
		inactive.Status = user.ProductUserStatusInactive
		assert.True(t, clerkUpsertWouldChange(inactive, same))
	})
}
