package service

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/pgkit/pglock"
)

var ErrLicensingWriteInProgress = fault.Conflict(
	"LICENSING_WRITE_IN_PROGRESS",
	"Another licensing update is in progress for this product. Please try again shortly.",
)

// ponytail: serialize licensing writes per product; use finer locks if contention warrants it.
func licenseWriteLockKey(tenantID, productID string) string {
	return "licensing-write:" + tenantID + ":" + productID
}

func acquireLicenseWriteLock(ctx context.Context, tenantID, productID string) error {
	acquired, err := pglock.TryAdvisoryXactLock(
		ctx,
		transactor.CurrentTx(ctx),
		licenseWriteLockKey(tenantID, productID),
	)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrLicensingWriteInProgress
	}
	return nil
}

func withLicenseWrite[T any](
	ctx context.Context, tx transactor.Transactor, tenantID, productID string,
	write func(context.Context) (T, error),
) (T, error) {
	var result T
	err := tx.InTx(ctx, func(txCtx context.Context) error {
		if lockErr := acquireLicenseWriteLock(txCtx, tenantID, productID); lockErr != nil {
			return lockErr
		}
		var writeErr error
		result, writeErr = write(txCtx)
		return writeErr
	})
	return result, err
}
