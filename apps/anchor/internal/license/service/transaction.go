package service

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
)

// inTx joins the transaction ctx already carries, and begins one otherwise, so
// a caller composing a licensing write into a larger unit needs no second
// method to call. [transactor.Transactor.InTx] always begins its own, which
// would put a row the caller has written out of reach of the work here.
//
// The caller that owns the transaction owns the commit and the rollback.
func inTx(
	ctx context.Context, tx transactor.Transactor, fn func(ctx context.Context) error,
) error {
	if transactor.CurrentTx(ctx) != nil {
		return fn(ctx)
	}
	return tx.InTx(ctx, fn)
}
