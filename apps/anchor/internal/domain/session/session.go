package session

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// ReuseGrace is how long a rotated refresh token is still exchanged without
// revoking its session. Browser tabs that restore together each refresh with
// the same cookie, and a client retries after a lost response; both present a
// token another request just rotated.
const ReuseGrace = 10 * time.Second

type Session struct {
	ID             string
	PlatformUserID string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
}

func NewID() string {
	return ids.MustNew("psess")
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

type RefreshToken struct {
	ID        string    `validate:"required"`
	SessionID string    `validate:"required"`
	ExpiresAt time.Time `validate:"required"`
	RotatedAt *time.Time
}

func NewRefreshTokenID() string {
	return ids.MustNew("prtok")
}

type Presentation int

const (
	FirstUse Presentation = iota
	RepeatWithinGrace
	Replay
)

func (t *RefreshToken) PresentedAt(now time.Time) Presentation {
	if t.RotatedAt == nil {
		return FirstUse
	}
	if now.Sub(*t.RotatedAt) <= ReuseGrace {
		return RepeatWithinGrace
	}
	return Replay
}
