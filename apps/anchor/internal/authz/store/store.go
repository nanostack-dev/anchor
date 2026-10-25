// Package store gives each Product its OpenFGA store, the relationship store
// that mirrors its access data (ADR-0022 E5). Anchor records which store
// belongs to which Product.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"

	"anchor/internal/authz/engine"
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
)

var (
	// ErrProductNotFound is returned for a store requested for no existing Product.
	ErrProductNotFound = errors.New("product not found")
	// ErrInTransaction is returned when StoreID runs inside a transaction: the
	// OpenFGA store it may create commits on its own, so a rollback of the
	// caller's transaction would leave it behind unrecorded.
	ErrInTransaction = errors.New("authz store lookup must not run inside a transaction")
)

const productForeignKey = "authz_stores_product_id_fkey"

type Stores interface {
	// StoreID returns the Product's OpenFGA store, creating it on first use.
	// Two replicas creating one at once keep one store and delete the other.
	// It must run outside any transaction.
	StoreID(ctx context.Context, productID string) (string, error)
}

type stores struct {
	db       *sql.DB
	provider engine.Provider
	logger   zerolog.Logger
}

func New(db *sql.DB, provider engine.Provider, logger zerolog.Logger) Stores {
	return &stores{db: db, provider: provider, logger: logger.With().Str("component", "authz_store").Logger()}
}

func (s *stores) StoreID(ctx context.Context, productID string) (string, error) {
	if transactor.CurrentTx(ctx) != nil {
		return "", ErrInTransaction
	}
	recorded, err := s.find(ctx, productID)
	if err != nil || recorded != "" {
		return recorded, err
	}
	openfga, err := s.provider.Get()
	if err != nil {
		return "", err
	}
	created, err := openfga.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: productID})
	if err != nil {
		return "", fmt.Errorf("create openfga store: %w", err)
	}
	inserted, err := transactor.Query[[]model.AuthzStores](ctx, s.db,
		table.AuthzStores.INSERT(table.AuthzStores.ProductID, table.AuthzStores.StoreID).
			VALUES(productID, created.GetId()).
			ON_CONFLICT(table.AuthzStores.ProductID).DO_NOTHING().
			RETURNING(table.AuthzStores.StoreID),
	).OnForeignKey(ErrProductNotFound, productForeignKey).Value()
	if errors.Is(err, ErrProductNotFound) {
		s.discard(ctx, openfga, created.GetId())
		return "", err
	}
	if err == nil && len(inserted) > 0 {
		return inserted[0].StoreID, nil
	}
	// The insert lost a race, or failed in a way that may still have committed
	// it, such as a cancelled request: whatever is recorded now wins, and this
	// store is deleted only if it is not the one recorded.
	winner, findErr := s.find(context.WithoutCancel(ctx), productID)
	if findErr != nil {
		return "", errors.Join(err, findErr)
	}
	if winner != created.GetId() {
		s.discard(ctx, openfga, created.GetId())
	}
	if winner == "" {
		return "", err
	}
	return winner, nil
}

func (s *stores) find(ctx context.Context, productID string) (string, error) {
	found, err := transactor.QueryOptional[model.AuthzStores](ctx, s.db,
		table.AuthzStores.SELECT(table.AuthzStores.StoreID).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	)
	if err != nil {
		return "", err
	}
	return found.Map(func(row model.AuthzStores) string { return row.StoreID }).OrElse(""), nil
}

func (s *stores) discard(ctx context.Context, openfga engine.Engine, storeID string) {
	if _, err := openfga.DeleteStore(
		context.WithoutCancel(ctx),
		&openfgav1.DeleteStoreRequest{StoreId: storeID},
	); err != nil {
		s.logger.Warn().Err(err).Str("store_id", storeID).Msg("failed to delete an unused openfga store")
	}
}
