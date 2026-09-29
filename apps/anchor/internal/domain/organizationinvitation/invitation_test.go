package organizationinvitation_test

import (
	"testing"
	"time"

	"anchor/internal/domain/organizationinvitation"
)

func TestDeriveStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	acceptedAt := now.Add(-time.Hour)

	tests := []struct {
		name       string
		acceptedAt *time.Time
		expiresAt  time.Time
		want       organizationinvitation.Status
	}{
		{"not accepted and expiry ahead is pending", nil, now.Add(time.Minute), organizationinvitation.StatusPending},
		{"not accepted and expiry now is expired", nil, now, organizationinvitation.StatusExpired},
		{"not accepted and expiry behind is expired", nil, now.Add(-time.Minute), organizationinvitation.StatusExpired},
		{
			"accepted before the expiry passed is accepted",
			&acceptedAt,
			now.Add(time.Minute),
			organizationinvitation.StatusAccepted,
		},
		{
			"accepted and expiry behind is still accepted",
			&acceptedAt,
			now.Add(-time.Minute),
			organizationinvitation.StatusAccepted,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := organizationinvitation.DeriveStatus(test.acceptedAt, test.expiresAt, now); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
