package platformsession_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/platformsession"
)

func TestExchangeAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rotatedAt := func(ago time.Duration) *time.Time {
		at := now.Add(-ago)
		return &at
	}

	cases := []struct {
		name      string
		rotatedAt *time.Time
		want      platformsession.Exchange
	}{
		{name: "UnrotatedTokenRotates", rotatedAt: nil, want: platformsession.ExchangeRotate},
		{
			name:      "JustRotatedTokenIsWithinGrace",
			rotatedAt: rotatedAt(time.Second),
			want:      platformsession.ExchangeWithinGrace,
		},
		{
			name:      "TokenRotatedExactlyAtGraceIsWithinGrace",
			rotatedAt: rotatedAt(platformsession.ReuseGrace),
			want:      platformsession.ExchangeWithinGrace,
		},
		{
			name:      "TokenRotatedPastGraceIsReuse",
			rotatedAt: rotatedAt(platformsession.ReuseGrace + time.Millisecond),
			want:      platformsession.ExchangeReuse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := platformsession.RefreshToken{RotatedAt: tc.rotatedAt}
			assert.Equal(t, tc.want, token.ExchangeAt(now))
		})
	}
}
