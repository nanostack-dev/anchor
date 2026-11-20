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
		`$x := transactor.ForNoKeyUpdate($_)`, `$x = transactor.ForNoKeyUpdate($_)`,
		`$x := transactor.ForKeyShare($_)`, `$x = transactor.ForKeyShare($_)`,
		`$x := transactor.WithRowLock($*_)`, `$x = transactor.WithRowLock($*_)`,
	).Report(`pass the locked context inline to the one call it locks: every query run with $x is locked`)
}

// failureAuditLeavesTheAbortedTx reports a command handler that writes an ERROR
// audit row on its caller's context. The failed write aborted that transaction,
// so the row fails with 25P02 and rolls back anyway. Return
// provider.WithFailureAudit and let the caller write it after the rollback.
func failureAuditLeavesTheAbortedTx(m dsl.Matcher) {
	m.Import("anchor/internal/integration/provider")
	m.Match(`provider.WriteAuditLog($_, $_, $_, $audit)`).
		Where(m["audit"].Text.Matches(`AuditSeverityError`)).
		Report(`return provider.WithFailureAudit: an ERROR audit row written on the failed work's context is lost with its aborted transaction`)
}
