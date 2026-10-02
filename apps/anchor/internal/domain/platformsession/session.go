package platformsession

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
	CreatedAt      time.Time
}

func (s *Session) GenerateID() {
	s.ID = ids.MustNew("psess")
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

type RefreshToken struct {
	ID        string
	SessionID string
	TokenHash string
	ExpiresAt time.Time
	RotatedAt *time.Time
	CreatedAt time.Time
}

func (t *RefreshToken) GenerateID() {
	t.ID = ids.MustNew("prtok")
}

// Exchange is what presenting a refresh token at a given moment amounts to.
type Exchange int

const (
	ExchangeRotate Exchange = iota
	ExchangeWithinGrace
	ExchangeReuse
)

func (t *RefreshToken) ExchangeAt(now time.Time) Exchange {
	if t.RotatedAt == nil {
		return ExchangeRotate
	}
	if now.Sub(*t.RotatedAt) <= ReuseGrace {
		return ExchangeWithinGrace
	}
	return ExchangeReuse
}
