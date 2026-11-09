//go:build !contract

package session

// reuseGrace is the grace PresentedAt applies: ReuseGrace in every build but
// the contract suite's (grace_contract.go).
const reuseGrace = ReuseGrace
