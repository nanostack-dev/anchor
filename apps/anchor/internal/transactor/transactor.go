package transactor

import (
	"context"
	"database/sql"
	"errors"

	jet "github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

type txContextKey struct{}

// Transactor runs work inside a SQL transaction and propagates that transaction via context.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type sqlTransactor struct {
	db *sql.DB
}

// New creates a transaction runner backed by the provided database handle.
func New(db *sql.DB) Transactor {
	return &sqlTransactor{db: db}
}

func (t *sqlTransactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ctxWithTx := WithTx(ctx, tx)
	runErr := fn(ctxWithTx)
	if runErr != nil {
		return runErr
	}
	commitErr := tx.Commit()
	if commitErr != nil {
		return commitErr
	}
	committed = true
	return nil
}

// WithTx returns a context that carries the provided transaction.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, txContextKey{}, tx)
}

// CurrentTx returns the transaction stored in ctx when present.
func CurrentTx(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(txContextKey{}).(*sql.Tx)
	return tx
}

// Executor returns the transaction stored in ctx when present, otherwise the provided db.
func Executor(ctx context.Context, db qrm.DB) qrm.DB {
	if tx := CurrentTx(ctx); tx != nil {
		return tx
	}
	return db
}

// Query executes a query and returns the results.
func Query[T any](ctx context.Context, db qrm.DB, stmt jet.Statement) (T, error) {
	var result T
	err := stmt.QueryContext(ctx, Executor(ctx, db), &result)
	return result, err
}

// QueryOptional executes a query that may return 0 rows.
func QueryOptional[T any](ctx context.Context, db qrm.DB, stmt jet.Statement) (*T, error) {
	var result T
	err := stmt.QueryContext(ctx, Executor(ctx, db), &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, qrm.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// QueryMap executes a query and maps the result.
func QueryMap[T any, R any](ctx context.Context, db qrm.DB, stmt jet.Statement, mapFunc func(T) R) (R, error) {
	result, err := Query[T](ctx, db, stmt)
	if err != nil {
		var zero R
		return zero, err
	}
	return mapFunc(result), nil
}

// QueryOptionalMap executes a query that may return 0 rows and maps the result when present.
func QueryOptionalMap[T any, R any](ctx context.Context, db qrm.DB, stmt jet.Statement, mapFunc func(T) R) (*R, error) {
	result, err := QueryOptional[T](ctx, db, stmt)
	if err != nil || result == nil {
		return nil, err
	}
	mapped := mapFunc(*result)
	return &mapped, nil
}

// QueryMapSlice executes a query and maps a slice of results.
func QueryMapSlice[T any, R any](ctx context.Context, db qrm.DB, stmt jet.Statement, mapFunc func(T) R) ([]R, error) {
	var results []T
	if err := stmt.QueryContext(ctx, Executor(ctx, db), &results); err != nil {
		return nil, err
	}
	mapped := make([]R, len(results))
	for i, result := range results {
		mapped[i] = mapFunc(result)
	}
	return mapped, nil
}

// Exec executes a statement.
func Exec(ctx context.Context, db qrm.DB, stmt jet.Statement) error {
	_, err := stmt.ExecContext(ctx, Executor(ctx, db))
	return err
}

// QueryCount executes a query count statement.
func QueryCount(ctx context.Context, db qrm.DB, statement jet.Statement) (int64, error) {
	query, args := statement.Sql()
	rows, err := Executor(ctx, db).QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, sql.ErrNoRows
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
