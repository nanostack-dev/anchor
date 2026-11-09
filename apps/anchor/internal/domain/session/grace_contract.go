//go:build contract

package session

import "time"

// reuseGrace is the grace PresentedAt applies. The golden contract suite
// (cmd/it/contract, build tag contract) runs the server in-process and
// shortens it, so the auth gate flow waits past a short grace instead of
// sleeping 12 s; no other build compiles this file.
var reuseGrace = ReuseGrace //nolint:gochecknoglobals // contract builds only

// SetReuseGraceForContract sets the reuse grace for the rest of the process.
// Call it before the server serves its first request.
func SetReuseGraceForContract(grace time.Duration) {
	reuseGrace = grace
}
