package fgasync

import "time"

// ShortenLockWait makes a busy lock fail fast in tests.
func (s *Syncer) ShortenLockWait(wait time.Duration) {
	s.timing = timing{lockWait: wait, lockRetryMin: time.Millisecond, lockRetryMax: time.Millisecond}
}

var SmallestCover = smallestCover
