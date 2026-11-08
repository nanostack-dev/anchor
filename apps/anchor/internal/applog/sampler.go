package applog

import (
	"sync"
	"time"
)

// BurstSampler lets Burst lines through per Period and drops the rest, as
// zerolog's BurstSampler without a next sampler did. It bounds the volume of
// a line that can repeat once per request, such as a refused authorization
// write. The zero value drops every line; a nil sampler keeps every line, as a
// zerolog logger without a sampler did.
type BurstSampler struct {
	Burst  int
	Period time.Duration

	mu      sync.Mutex
	resetAt time.Time
	count   int
}

// Sample reports whether the next line should be written.
func (s *BurstSampler) Sample() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.After(s.resetAt) {
		s.resetAt = now.Add(s.Period)
		s.count = 0
	}
	if s.count >= s.Burst {
		return false
	}
	s.count++
	return true
}
