package service

import (
	"context"

	"github.com/rs/zerolog"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
)

// NewSyncerWithLock builds a syncer whose product lock is the test's, so the
// sync logic runs without Postgres. lock reports false when busy.
func NewSyncerWithLock(
	openfga engine.Engine, repo repository.Repository,
	lock func(ctx context.Context, productID string, fn func(context.Context) error) (bool, error),
) *Syncer {
	return newSyncer(openfga, repo, lockFunc(lock), zerolog.Nop())
}

type lockFunc func(ctx context.Context, productID string, fn func(context.Context) error) (bool, error)

func (f lockFunc) TryDo(ctx context.Context, productID string, fn func(context.Context) error) (bool, error) {
	return f(ctx, productID, fn)
}
