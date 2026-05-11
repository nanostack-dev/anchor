package email

import "time"

// RateLimitWindow describes a sliding-window send budget for a product. The
// service tallies sends over (now - WindowDuration) and rejects with
// ErrEmailRateLimitExceeded when Count would exceed the budget. Two budgets
// are evaluated in order (per-minute then per-hour) so a runaway loop can be
// stopped quickly without blocking expected daily volume.
type RateLimitWindow struct {
	WindowDuration time.Duration
	MaxSends       int64
}

// DefaultRateLimits ships sensible defaults for the v1 product. They are
// intentionally permissive; the real values belong in product-level config.
//
//nolint:mnd,gochecknoglobals // configuration defaults
var DefaultRateLimits = []RateLimitWindow{
	{WindowDuration: time.Minute, MaxSends: 60},
	{WindowDuration: time.Hour, MaxSends: 1000},
}
