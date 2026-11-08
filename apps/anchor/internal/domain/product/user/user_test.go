package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/product/user"
)

func TestParseProductUserStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want user.ProductUserStatus
	}{
		{"active", user.ProductUserStatusActive},
		{"invited", user.ProductUserStatusInvited},
		{"suspended", user.ProductUserStatusSuspended},
		// Stored and returned until 2026-11-08: clients and old rows keep working.
		{"ACTIVE", user.ProductUserStatusActive},
		{"Suspended", user.ProductUserStatusSuspended},
	} {
		got, err := user.ParseProductUserStatus(tc.in)
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}

	for _, in := range []string{"", "INACTIVE", "banana", " active"} {
		_, err := user.ParseProductUserStatus(in)
		require.ErrorIs(t, err, user.ErrUnknownProductUserStatus, "%q is outside the contract enum", in)
	}
}

func TestProductUserStatusesAreValid(t *testing.T) {
	t.Parallel()

	for _, status := range []user.ProductUserStatus{
		user.ProductUserStatusActive, user.ProductUserStatusInvited, user.ProductUserStatusSuspended,
	} {
		assert.True(t, status.Valid(), status)
	}
	assert.False(t, user.ProductUserStatus("ACTIVE").Valid(), "only the contract spelling is valid")
}
