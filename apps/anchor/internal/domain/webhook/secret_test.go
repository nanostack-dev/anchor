package webhook_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestSecretIsUsable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name   string
		secret webhook.Secret
		want   bool
	}{
		{
			name:   "active without expiry",
			secret: webhook.Secret{Status: webhook.SecretStatusActive},
			want:   true,
		},
		{
			name:   "expiring but still inside the rotation window",
			secret: webhook.Secret{Status: webhook.SecretStatusExpiring, ExpiresAt: &future},
			want:   true,
		},
		{
			name:   "expiring past its window",
			secret: webhook.Secret{Status: webhook.SecretStatusExpiring, ExpiresAt: &past},
			want:   false,
		},
		{
			name:   "unknown status never signs",
			secret: webhook.Secret{Status: webhook.SecretStatus("RETIRED")},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			secret := tt.secret
			assert.Equal(t, tt.want, secret.IsUsable(now))
		})
	}
}

func TestUsableSecretsDuringRotation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	within := now.Add(webhook.SecretRotationGrace)
	elapsed := now.Add(-time.Second)

	all := []webhook.Secret{
		{ID: "whs_new", Status: webhook.SecretStatusActive},
		{ID: "whs_rotating", Status: webhook.SecretStatusExpiring, ExpiresAt: &within},
		{ID: "whs_old", Status: webhook.SecretStatusExpiring, ExpiresAt: &elapsed},
	}

	usable := webhook.UsableSecrets(all, now)
	require.Len(t, usable, 2, "both sides of a live rotation sign; the retired one does not")
	assert.Equal(t, "whs_new", usable[0].ID)
	assert.Equal(t, "whs_rotating", usable[1].ID)
}

func TestSecretStatusValidity(t *testing.T) {
	t.Parallel()

	assert.True(t, webhook.SecretStatusActive.IsValid())
	assert.True(t, webhook.SecretStatusExpiring.IsValid())
	assert.False(t, webhook.SecretStatus("REVOKED").IsValid())
}
