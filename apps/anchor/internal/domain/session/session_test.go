package session_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/session"
)

func TestPresentedAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rotatedAgo := func(ago time.Duration) *time.Time {
		at := now.Add(-ago)
		return &at
	}

	cases := []struct {
		name      string
		rotatedAt *time.Time
		want      session.Presentation
	}{
		{name: "UnrotatedTokenIsAFirstUse", rotatedAt: nil, want: session.FirstUse},
		{name: "JustRotatedTokenIsWithinGrace", rotatedAt: rotatedAgo(time.Second), want: session.RepeatWithinGrace},
		{
			name:      "TokenRotatedExactlyAtGraceIsWithinGrace",
			rotatedAt: rotatedAgo(session.ReuseGrace),
			want:      session.RepeatWithinGrace,
		},
		{
			name:      "TokenRotatedPastGraceIsAReplay",
			rotatedAt: rotatedAgo(session.ReuseGrace + time.Millisecond),
			want:      session.Replay,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := session.RefreshToken{RotatedAt: tc.rotatedAt}
			assert.Equal(t, tc.want, token.PresentedAt(now))
		})
	}
}
