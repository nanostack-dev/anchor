package fgasync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"

	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
)

const (
	syncLockPrefix = "authz-sync:"
	sweepLockKey   = "authz-sweep"
)

var errNotAcquired = errors.New("lock not acquired")

type postgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository keeps the marks in authz_sync_marks and each store's
// model and last rebuild in authz_stores.
func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Mark(ctx context.Context, scope Scope) error {
	return transactor.Exec(
		ctx,
		r.db,
		table.AuthzSyncMarks.INSERT(table.AuthzSyncMarks.ProductID, table.AuthzSyncMarks.Source, table.AuthzSyncMarks.ScopeKey).
			VALUES(scope.ProductID, scope.Source, scope.Key).
			ON_CONFLICT(table.AuthzSyncMarks.ProductID, table.AuthzSyncMarks.Source, table.AuthzSyncMarks.ScopeKey).
			DO_UPDATE(postgres.SET(
				table.AuthzSyncMarks.Revision.SET(postgres.IntExp(postgres.Raw("nextval('authz_sync_mark_revisions')"))),
				table.AuthzSyncMarks.MarkedAt.SET(postgres.TimestampzExp(postgres.NOW())),
			),
			),
	).Err()
}

func toMark(row dbmodel.AuthzSyncMarks) Mark {
	return Mark{
		ProductID: row.ProductID, Source: row.Source, Key: row.ScopeKey,
		Revision: row.Revision, MarkedAt: row.MarkedAt,
	}
}

func (r *postgresRepository) Pending(ctx context.Context, productID string) ([]Mark, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
			WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.AuthzSyncMarks.Source.ASC(), table.AuthzSyncMarks.ScopeKey.ASC()),
		toMark,
	).Value()
}

func (r *postgresRepository) ClearIfUnchanged(ctx context.Context, mark Mark) error {
	return transactor.Exec(ctx, r.db, table.AuthzSyncMarks.DELETE().WHERE(
		table.AuthzSyncMarks.ProductID.EQ(postgres.String(mark.ProductID)).
			AND(table.AuthzSyncMarks.Source.EQ(postgres.String(mark.Source))).
			AND(table.AuthzSyncMarks.ScopeKey.EQ(postgres.String(mark.Key))).
			AND(table.AuthzSyncMarks.Revision.EQ(postgres.Int(mark.Revision))),
	)).Err()
}

func (r *postgresRepository) Oldest(ctx context.Context, limit int) ([]Mark, error) {
	return transactor.QueryMapSlice(
		ctx,
		r.db,
		table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
			ORDER_BY(table.AuthzSyncMarks.MarkedAt.ASC()).
			LIMIT(int64(limit)),
		toMark,
	).Value()
}

func (r *postgresRepository) Postpone(ctx context.Context, productID string, until time.Time) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzSyncMarks.UPDATE(table.AuthzSyncMarks.MarkedAt).
			SET(postgres.TimestampzT(until)).
			WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *postgresRepository) RecordedModel(ctx context.Context, productID string) (RecordedModel, error) {
	found, err := transactor.QueryOptionalMap(ctx, r.db,
		table.AuthzStores.SELECT(table.AuthzStores.ModelID, table.AuthzStores.ModelHash).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
		func(row dbmodel.AuthzStores) RecordedModel {
			return RecordedModel{
				ID:   functional.FromPtr(row.ModelID).OrElse(""),
				Hash: functional.FromPtr(row.ModelHash).OrElse(""),
			}
		},
	)
	return found.OrElse(RecordedModel{}), err
}

func (r *postgresRepository) SetModel(ctx context.Context, productID string, model RecordedModel) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzStores.UPDATE(table.AuthzStores.ModelID, table.AuthzStores.ModelHash).
			SET(postgres.String(model.ID), postgres.String(model.Hash)).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *postgresRepository) SetReconciled(ctx context.Context, productID string, at time.Time) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzStores.UPDATE(table.AuthzStores.ReconciledAt).
			SET(postgres.TimestampzT(at)).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *postgresRepository) DueForReconcile(ctx context.Context, before time.Time) (functional.Option[string], error) {
	return transactor.QueryOptionalMap(ctx, r.db,
		postgres.SELECT(table.Products.ID).
			FROM(table.Products.LEFT_JOIN(table.AuthzStores, table.AuthzStores.ProductID.EQ(table.Products.ID))).
			WHERE(table.AuthzStores.ReconciledAt.IS_NULL().
				OR(table.AuthzStores.ReconciledAt.LT(postgres.TimestampzT(before)))).
			ORDER_BY(table.AuthzStores.ReconciledAt.ASC().NULLS_FIRST(), table.Products.ID.ASC()).
			LIMIT(1),
		func(row dbmodel.Products) string { return row.ID },
	)
}

func (r *postgresRepository) ProductExists(ctx context.Context, productID string) (bool, error) {
	count, err := transactor.QueryCount(ctx, r.db,
		table.Products.SELECT(postgres.COUNT(postgres.STAR)).
			WHERE(table.Products.ID.EQ(postgres.String(productID))),
	).Value()
	return count > 0, err
}

type postgresLocker struct {
	tx    transactor.Transactor
	locks *pglock.Client
}

// NewPostgresLocker holds a Product's sync lock as a transaction advisory lock,
// so COMMIT or ROLLBACK releases it and a crash cannot leave it held, and the
// sweep lock as a session advisory lock.
func NewPostgresLocker(tx transactor.Transactor, locks *pglock.Client) Locker {
	return &postgresLocker{tx: tx, locks: locks}
}

func (l *postgresLocker) TryLocked(
	ctx context.Context,
	productID string,
	fn func(context.Context) error,
) (bool, error) {
	return l.tryInTx(ctx, fn, func(txCtx context.Context, tx *sql.Tx) (bool, error) {
		return pglock.TryAdvisoryXactLock(txCtx, tx, syncLockPrefix+productID)
	})
}

// tryInTx runs fn in a transaction once acquire got its transaction locks.
func (l *postgresLocker) tryInTx(
	ctx context.Context,
	fn func(context.Context) error,
	acquire func(context.Context, *sql.Tx) (bool, error),
) (bool, error) {
	err := l.tx.InTx(ctx, func(txCtx context.Context) error {
		acquired, err := acquire(txCtx, transactor.CurrentTx(txCtx))
		if err != nil {
			return fmt.Errorf("lock authz sync: %w", err)
		}
		if !acquired {
			return errNotAcquired
		}
		return fn(txCtx)
	})
	if errors.Is(err, errNotAcquired) {
		return false, nil
	}
	return true, err
}

func (l *postgresLocker) TryKeyLocked(
	ctx context.Context,
	productID, key string,
	fn func(context.Context) error,
) (bool, error) {
	return l.tryInTx(ctx, fn, func(txCtx context.Context, tx *sql.Tx) (bool, error) {
		var shared bool
		if err := tx.QueryRowContext(txCtx, `SELECT pg_try_advisory_xact_lock_shared($1)`,
			pglock.KeyHash(syncLockPrefix+productID)).Scan(&shared); err != nil || !shared {
			return false, err
		}
		return pglock.TryAdvisoryXactLock(txCtx, tx, syncLockPrefix+productID+":"+key)
	})
}

func (l *postgresLocker) TrySweep(ctx context.Context, fn func(context.Context) error) (bool, error) {
	return l.locks.TryWithSessionLock(ctx, sweepLockKey, fn)
}
