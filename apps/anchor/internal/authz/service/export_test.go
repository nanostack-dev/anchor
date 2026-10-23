package service

import (
	"context"

	"github.com/rs/zerolog"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
)

// NewSyncerWithLock builds a syncer whose product lock is the test's, so the
// sync logic runs without Postgres.
func NewSyncerWithLock(
	openfga engine.Engine, repo repository.Repository,
	lock func(ctx context.Context, productID string, fn func(context.Context) error) error,
) *Syncer {
	return newSyncer(openfga, repo, lockFunc(lock), zerolog.Nop())
}

type lockFunc func(ctx context.Context, productID string, fn func(context.Context) error) error

func (f lockFunc) Do(ctx context.Context, productID string, fn func(context.Context) error) error {
	return f(ctx, productID, fn)
}
