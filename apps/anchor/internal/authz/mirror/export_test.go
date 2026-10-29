package mirror

import "time"

// ShortenLockWait makes a busy lock fail fast in tests.
func (m *Mirror) ShortenLockWait(wait time.Duration) {
	m.timing = timing{lockWait: wait, lockRetryMin: time.Millisecond, lockRetryMax: time.Millisecond}
}

var SmallestCover = smallestCover
