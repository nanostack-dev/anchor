package fgasync

import "time"

// ShortenLockWait makes a busy lock fail fast in tests.
func (s *Syncer) ShortenLockWait(wait time.Duration) {
	s.timing = timing{lockWait: wait, minLockRetry: time.Millisecond, maxLockRetry: time.Millisecond}
}

var SmallestCover = smallestCover
