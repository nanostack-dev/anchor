package webhook

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// retryLadder is the base delay before each attempt: eight attempts spanning
// roughly twenty-one hours. It is built per call rather than held in a package
// variable so no caller can rewrite the schedule every other caller reads.
func retryLadder() []time.Duration {
	return []time.Duration{
		0,
		15 * time.Second,
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		6 * time.Hour,
		12 * time.Hour,
	}
}

const (
	// JitterFloor and JitterCeiling bound the multiplier applied to each rung.
	//
	// Without jitter every product's endpoint retries in lockstep after *our*
	// deploy, turning one incident into a self-inflicted thundering herd.
	JitterFloor   = 0.75
	JitterCeiling = 1.25
)

// Jitterer supplies the randomness used by RetryDelay. It is an interface so
// tests can pin the multiplier and assert exact delays.
type Jitterer interface {
	// Float64 returns a pseudo-random number in [0.0, 1.0).
	Float64() float64
}

// DefaultJitterer returns the process-wide random source.
func DefaultJitterer() Jitterer {
	return globalJitterer{}
}

type globalJitterer struct{}

// Float64 draws from crypto/rand. Jitter is not a security boundary, but taking
// the bytes from the same source everything else uses avoids seeding a second
// generator for no benefit.
func (globalJitterer) Float64() float64 {
	const (
		sampleBytes   = 8
		uint64Bits    = 64
		mantissaBits  = 53
		neutralJitter = 0.5
	)

	var buffer [sampleBytes]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		// The centre of the window keeps the ladder correct; only the
		// herd-spreading property is lost.
		return neutralJitter
	}

	return float64(binary.BigEndian.Uint64(buffer[:])>>(uint64Bits-mantissaBits)) /
		float64(uint64(1)<<mantissaBits)
}

// JitterFactor maps a [0,1) sample onto the [0.75, 1.25] jitter window.
func JitterFactor(sample float64) float64 {
	return JitterFloor + sample*(JitterCeiling-JitterFloor)
}

// RetryDelay returns how long to wait before the next attempt, given how many
// attempts have already been made. attemptsMade == 0 means the delivery has
// never been tried, which is the immediate rung.
//
// Delays past the end of the ladder stay on the last rung rather than growing,
// so a queue-level over-delivery cannot push an attempt weeks out.
func RetryDelay(attemptsMade int, jitterer Jitterer) time.Duration {
	ladder := retryLadder()
	index := max(attemptsMade, 0)
	if index >= len(ladder) {
		index = len(ladder) - 1
	}

	base := ladder[index]
	if base == 0 {
		return 0
	}
	if jitterer == nil {
		jitterer = DefaultJitterer()
	}

	return time.Duration(float64(base) * JitterFactor(jitterer.Float64()))
}

// RetryLadder returns the base delays, for tests and documentation.
func RetryLadder() []time.Duration {
	return retryLadder()
}
