package events

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"

	"github.com/nanostack-dev/pgkit/queue"

	serviceconfig "anchor/internal/service/config"
)

// A deployment (prod, dev) retries a failing delivery for about a day, close
// to the Standard Webhooks (Svix) schedule: 20 attempts, 10 s doubling to a
// 2 h cap, 15.6 h to 20.8 h from the first attempt to the last. Six attempts
// at 1 s once gave up after 31 s, so a short endpoint outage dropped events.
// The test suites wait for six failures, so they keep six attempts within a
// second.
const (
	deployedDeliveryMaxAttempts = 20
	deployedDeliveryBackoffBase = 10 * time.Second
	deployedDeliveryBackoffMax  = 2 * time.Hour
	testDeliveryMaxAttempts     = 6
	testDeliveryBackoffBase     = 25 * time.Millisecond
	testDeliveryBackoffMax      = time.Second
	// deliveryJitterDivisor bounds the jitter to a quarter of the delay, so
	// replicas do not retry in step and the retry window keeps a floor.
	deliveryJitterDivisor = 4
	// deployedGoneMinFailingFor is how long an endpoint must have failed
	// without a success before a 410 Gone removes it. It is shorter than the
	// shortest retry window (15.6 h), so a single event can still retire a
	// gone endpoint, and far longer than a deploy or a proxy answering 410.
	deployedGoneMinFailingFor = 12 * time.Hour
	// deployedBreakerMinFailedCalls is the failure streak from which the
	// worker stops dialing an endpoint until its cooldown has passed.
	deployedBreakerMinFailedCalls = 5
	// breakerCooldownDivisor halves the retry curve for the breaker's cooldown.
	breakerCooldownDivisor = 2
)

// deliveryRetryPolicy is the one schedule that the emitter (the attempts of a
// delivery job) and the worker (the delay between them) share, with the
// worker's rules for a failing endpoint.
type deliveryRetryPolicy struct {
	maxAttempts int
	base        time.Duration
	max         time.Duration
	// goneMinFailingFor is the youngest failure streak that a 410 Gone may
	// end by removing the endpoint.
	goneMinFailingFor time.Duration
	// breakerMinFailedCalls is the failure streak from which the worker skips
	// dialing the endpoint during its cooldown. Zero turns the breaker off.
	breakerMinFailedCalls int32
}

func deliveryRetryPolicyFor(core *serviceconfig.CoreConfig) deliveryRetryPolicy {
	if core != nil && core.IsTest() {
		// The suites assert every failed call and a removal after three
		// 410s, so they keep neither the age rule nor the breaker.
		return deliveryRetryPolicy{
			maxAttempts: testDeliveryMaxAttempts,
			base:        testDeliveryBackoffBase,
			max:         testDeliveryBackoffMax,
		}
	}
	return deliveryRetryPolicy{
		maxAttempts:           deployedDeliveryMaxAttempts,
		base:                  deployedDeliveryBackoffBase,
		max:                   deployedDeliveryBackoffMax,
		goneMinFailingFor:     deployedGoneMinFailingFor,
		breakerMinFailedCalls: deployedBreakerMinFailedCalls,
	}
}

// breakerWait is how long the worker still skips dialing the target at now.
// From breakerMinFailedCalls failed calls in a row, each failed call opens a
// cooldown of half the retry curve: half the base after the first, doubled per
// further failure, capped at half of max. One endpoint that hangs then costs
// the worker one call per cooldown, not one per queued event and attempt.
//
// Half keeps every cooldown below the shortest jittered retry delay (three
// quarters of the curve), so a job's own retries are never skipped, with room
// for skew between the database clock (last_failed_at) and the worker's.
func (p deliveryRetryPolicy) breakerWait(target DeliveryTarget, now time.Time) time.Duration {
	if p.breakerMinFailedCalls <= 0 || target.ConsecutiveFailedCalls < p.breakerMinFailedCalls ||
		target.LastFailedAt.IsZero() {
		return 0
	}
	cooldown := p.nominalDelay(int(target.ConsecutiveFailedCalls-p.breakerMinFailedCalls)+1) / breakerCooldownDivisor
	return max(target.LastFailedAt.Add(cooldown).Sub(now), 0)
}

// nominalDelay is the wait after a failed attempt (counted from 1) before
// jitter: base doubled per attempt, capped at max.
func (p deliveryRetryPolicy) nominalDelay(attempt int) time.Duration {
	delay := p.base
	for range max(attempt, 1) - 1 {
		if delay >= p.max/2 {
			return p.max
		}
		delay *= 2
	}
	return min(delay, p.max)
}

// retryDelay is the worker's RetryDelay. pgkit's default backoff jitters over
// the whole delay, which leaves the retry window no floor. The jitter comes
// from a hash of the job and its attempt: jobs that fail together spread out,
// and no random source is needed.
func (p deliveryRetryPolicy) retryDelay(job queue.Job, _ error) time.Duration {
	nominal := p.nominalDelay(job.Attempts)
	jitter := nominal / deliveryJitterDivisor
	if jitter <= 0 {
		return nominal
	}
	hash := fnv.New32a()
	_, _ = fmt.Fprintf(hash, "%d:%d", job.ID, job.Attempts)
	share := float64(hash.Sum32()) / (math.MaxUint32 + 1)
	return nominal - time.Duration(share*float64(jitter))
}

// retryWindow is the shortest and the longest time from a delivery's first
// attempt to its last.
func (p deliveryRetryPolicy) retryWindow() (time.Duration, time.Duration) {
	var shortest, longest time.Duration
	for attempt := 1; attempt < p.maxAttempts; attempt++ {
		nominal := p.nominalDelay(attempt)
		longest += nominal
		shortest += nominal - nominal/deliveryJitterDivisor
	}
	return shortest, longest
}
