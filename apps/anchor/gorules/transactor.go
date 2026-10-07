//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// rowLockStaysInline reports a locked context stored in a variable. Every query
// run with it is locked, so it belongs inline in the one call it locks.
func rowLockStaysInline(m dsl.Matcher) {
	m.Import("github.com/nanostack-dev/nanostack-framework/pkg/db/transactor")
	m.Match(
		`$x := transactor.ForShare($_)`, `$x = transactor.ForShare($_)`,
		`$x := transactor.ForUpdate($_)`, `$x = transactor.ForUpdate($_)`,
		`$x := transactor.WithRowLock($*_)`, `$x = transactor.WithRowLock($*_)`,
	).Report(`pass the locked context inline to the one call it locks: every query run with $x is locked`)
}
