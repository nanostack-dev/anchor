package anchorsdk

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

// Default retry parameters. One initial attempt plus two retries, which covers a
// brief restart or a transient 5xx without holding a caller for long.
const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 200 * time.Millisecond
	defaultMaxDelay    = 2 * time.Second
)

// RetryPolicy bounds how a call is retried after a transient failure. The zero
// value means "use the defaults"; set MaxAttempts to 1 to disable retrying.
//
// Only transient failures are retried: transport errors, 5xx, and 429. Every
// other 4xx is permanent and returns on the first attempt — see [Error.Permanent].
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts int
	// BaseDelay is the delay before the second attempt. It doubles each retry.
	BaseDelay time.Duration
	// MaxDelay caps the backoff.
	MaxDelay time.Duration
}

// withDefaults fills zero-valued fields, leaving explicit settings alone.
func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = defaultMaxAttempts
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = defaultBaseDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = defaultMaxDelay
	}
	return p
}

// do runs fn until it succeeds, fails permanently, exhausts attempts, or ctx is
// cancelled. It returns the last error observed.
func (p RetryPolicy) do(ctx context.Context, fn func(context.Context) error) error {
	policy := p.withDefaults()
	delay := policy.BaseDelay

	var err error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err = fn(ctx); err == nil {
			return nil
		}
		if errors.Is(err, ErrPermanent) || attempt == policy.MaxAttempts {
			return err
		}

		select {
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		case <-time.After(jitter(delay)):
		}

		if delay *= 2; delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return err
}

// jitterSpread is the fraction of the delay left fixed: half of it is kept and
// the rest is randomized, giving the [d/2, d) window below.
const jitterSpread = 2

// jitter spreads retries across a window so that concurrent callers recovering
// from the same outage do not re-converge on Anchor in lockstep. It returns a
// duration in [d/2, d).
func jitter(d time.Duration) time.Duration {
	half := d / jitterSpread
	if half <= 0 {
		return d
	}
	// Backoff spread is a scheduling decision, not a security one, so the cheap
	// PRNG is the right pick here.
	return half + time.Duration(rand.Int64N(int64(half))) //nolint:gosec // G404: jitter needs no CSPRNG
}

// retrying runs a call that produces a value, applying the client's policy.
func retrying[T any](ctx context.Context, c *Client, fn func(context.Context) (*T, error)) (*T, error) {
	var out *T
	err := c.retry.do(ctx, func(ctx context.Context) error {
		var callErr error
		out, callErr = fn(ctx)
		return callErr
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
