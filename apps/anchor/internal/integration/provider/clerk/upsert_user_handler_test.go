//nolint:testpackage // tests the unexported clerkUpsertWouldChange helper
package clerk

import (
	"testing"
	"time"

	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
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

	t.Run("InvitedExistingUserBecomesActive", func(t *testing.T) {
		t.Parallel()
		invited := existing
		invited.Status = user.ProductUserStatusInvited
		assert.True(t, clerkUpsertWouldChange(invited, same))
	})

	t.Run("SuspendedExistingUserWithSameDataIsUnchanged", func(t *testing.T) {
		t.Parallel()
		suspended := existing
		suspended.Status = user.ProductUserStatusSuspended
		assert.False(t, clerkUpsertWouldChange(suspended, same))
	})

	t.Run("SuspendedExistingUserWithChangedEmailChanges", func(t *testing.T) {
		t.Parallel()
		suspended := existing
		suspended.Status = user.ProductUserStatusSuspended
		data := same
		data.Email = "countess@example.com"
		assert.True(t, clerkUpsertWouldChange(suspended, data))
	})
}

func TestClerkUpsertStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		existing functional.Option[user.ProductUser]
		want     user.ProductUserStatus
	}{
		{"NewUserIsActive", functional.None[user.ProductUser](), user.ProductUserStatusActive},
		{"ActiveStaysActive", stored(user.ProductUserStatusActive), user.ProductUserStatusActive},
		{"InvitedBecomesActive", stored(user.ProductUserStatusInvited), user.ProductUserStatusActive},
		{"SuspendedStaysSuspended", stored(user.ProductUserStatusSuspended), user.ProductUserStatusSuspended},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, clerkUpsertStatus(tc.existing))
		})
	}
}

func stored(status user.ProductUserStatus) functional.Option[user.ProductUser] {
	return functional.Some(user.ProductUser{Status: status})
}

func TestIsTombstoned(t *testing.T) {
	t.Parallel()

	deletedAt := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	older := deletedAt.Add(-time.Second)
	same := deletedAt
	newer := deletedAt.Add(time.Second)

	assert.True(t, isTombstoned(nil, deletedAt), "missing event timestamp must not bypass tombstone")
	assert.True(t, isTombstoned(&older, deletedAt))
	assert.True(t, isTombstoned(&same, deletedAt))
	assert.False(t, isTombstoned(&newer, deletedAt))
}

func TestIsStaleEvent(t *testing.T) {
	t.Parallel()

	syncedAt := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	older := syncedAt.Add(-time.Millisecond)
	same := syncedAt
	newer := syncedAt.Add(time.Millisecond)

	assert.False(t, isStaleEvent(nil, &syncedAt))
	assert.False(t, isStaleEvent(&older, nil))
	assert.True(t, isStaleEvent(&older, &syncedAt))
	assert.False(t, isStaleEvent(&same, &syncedAt))
	assert.False(t, isStaleEvent(&newer, &syncedAt))
}
