package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

func _() {
	nanorepo.Implement[ProductUserRepository, productUserRepositoryImpl](
		table.ProductUsers, nanorepo.NoMapper[model.ProductUsers],
	)

	// POST /v1/products/{product_id}/product-users/search
	nanorepo.Search[model.ProductUsers]("ProductUser", table.ProductUsers,
		nanorepo.MapPage((*productUserRows).page),
		nanorepo.Scope(table.ProductUsers.ProductID),
		nanorepo.Filter(table.ProductUsers.ID, nanorepo.In),
		nanorepo.Filter(table.ProductUsers.Email, nanorepo.In),
		nanorepo.Filter(table.ProductUsers.Name, nanorepo.ContainsAny),
		nanorepo.FilterAs[user.ProductUserStatus](table.ProductUsers.Status, nanorepo.In),
		nanorepo.Filter(table.ProductUsers.ExternalID, nanorepo.In),
		nanorepo.FullText(table.ProductUsers.Name, table.ProductUsers.Email),
		nanorepo.Sort(
			table.ProductUsers.CreatedAt, table.ProductUsers.UpdatedAt, table.ProductUsers.Email,
			table.ProductUsers.Name, table.ProductUsers.Status,
		),
		nanorepo.DefaultSort(table.ProductUsers.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

func productUsersUpdatableColumns() postgres.ColumnList {
	return table.ProductUsers.AllColumns.Except(
		table.ProductUsers.CreatedAt,
		table.ProductUsers.UpdatedAt,
		table.ProductUsers.LastSyncedEventAt,
	)
}

type ProductUserRepository interface {
	FindByProductIDAndID(
		ctx context.Context,
		productID string,
		id string,
	) (functional.Option[user.ProductUser], error)
	// FindByProductAndEmail looks the email up on the (product_id, email)
	// unique index; the match is exact, as the index is.
	FindByProductAndEmail(
		ctx context.Context, productID string, email string,
	) (functional.Option[user.ProductUser], error)
	FindByExternalID(
		ctx context.Context, productID string, externalID string,
	) (functional.Option[user.ProductUser], error)
	Create(
		ctx context.Context, user user.ProductUser,
	) (user.ProductUser, error)
	Update(
		ctx context.Context,
		productID string,
		id string,
		entity user.ProductUser,
	) (user.ProductUser, error)
	// UpsertByExternalID inserts or updates a product user by external ID.
	// The returned bool is true when a new row was inserted, false when an
	// existing row was updated.
	UpsertByExternalID(
		ctx context.Context, entity user.ProductUser,
	) (user.ProductUser, bool, error)
	// LockClerkUser acquires a transaction-scoped advisory lock on
	// (productID, externalID) so concurrent upserts and deletes serialize even
	// when no product_users row exists yet.
	LockClerkUser(ctx context.Context, productID, externalID string) error
	// FindTombstone returns the provider deletion timestamp recorded for a
	// Clerk user, if any.
	FindTombstone(ctx context.Context, productID, externalID string) (functional.Option[time.Time], error)
	// PutTombstone records or advances the tombstone for a deleted Clerk user.
	PutTombstone(ctx context.Context, productID, externalID string, deletedAt time.Time) error
	// InsertIfAbsent inserts a synced product user with ON CONFLICT DO NOTHING
	// (no conflict target) so neither an email nor an external_id race aborts
	// the surrounding transaction. Returns None when a conflict skipped insert.
	InsertIfAbsent(ctx context.Context, entity user.ProductUser) (functional.Option[user.ProductUser], error)
	// AdoptExternalID links a verified Clerk external ID onto an existing
	// product_users row whose external_id is currently NULL.
	AdoptExternalID(
		ctx context.Context,
		productID, id, externalID, name string,
		status user.ProductUserStatus,
		eventAt *time.Time,
	) (functional.Option[user.ProductUser], error)
	// UpdateSynced updates an existing Clerk-synced product user and advances
	// its last_synced_event_at monotonically when eventAt is non-nil.
	UpdateSynced(
		ctx context.Context,
		productID, id, email, name string,
		status user.ProductUserStatus,
		eventAt *time.Time,
	) (user.ProductUser, error)
	// AdvanceLastSyncedEventAt moves last_synced_event_at forward when it is
	// NULL or older than eventAt, avoiding no-op UPDATEs that would bump updated_at.
	AdvanceLastSyncedEventAt(ctx context.Context, productID, id string, eventAt time.Time) error
	// CountClerkStampedUsers counts product users in the product that have both
	// an external_id and a non-NULL last_synced_event_at.
	CountClerkStampedUsers(ctx context.Context, productID string) (int64, error)
	// ListClerkStampedExternalIDsBefore returns external IDs of Clerk-stamped
	// product users whose last_synced_event_at and created_at are strictly
	// before the given snapshot start time.
	ListClerkStampedExternalIDsBefore(ctx context.Context, productID string, before time.Time) ([]string, error)
	// DeleteByProductAndID returns true when it deleted the row.
	DeleteByProductAndID(ctx context.Context, productID string, id string) (bool, error)
	DeleteByProductAndExternalID(ctx context.Context, productID string, externalID string) error
	SearchByProductID(ctx context.Context, q ProductUserSearch) (search.Result[user.ProductUser], error)
}

type productUserRepositoryImpl struct {
	productUsersBase

	db                *sql.DB
	productUserMapper *mapper.ProductUserMapper
	rows              *productUserRows
	logger            *slog.Logger
}

// productUserRows maps rows of product users. The mapper can fail on a row, and
// the generated search calls a method of a field of the implementation.
type productUserRows struct {
	mapper *mapper.ProductUserMapper
}

func (m *productUserRows) page(_ context.Context, rows []model.ProductUsers) ([]user.ProductUser, error) {
	mapped := make([]user.ProductUser, 0, len(rows))
	for _, row := range rows {
		item, err := m.mapper.ToDomain(row)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, item)
	}
	return mapped, nil
}

func NewProductUserRepository(
	db *sql.DB,
	productUserMapper *mapper.ProductUserMapper,
	logger *slog.Logger,
) ProductUserRepository {
	return &productUserRepositoryImpl{
		productUsersBase:  newProductUsersBase(db),
		db:                db,
		productUserMapper: productUserMapper,
		rows:              &productUserRows{mapper: productUserMapper},
		logger:            logger.With(slog.String("component", "product_user_repository")),
	}
}

func (r *productUserRepositoryImpl) FindByProductIDAndID(
	ctx context.Context,
	productID string,
	id string,
) (functional.Option[user.ProductUser], error) {
	stmt := table.ProductUsers.SELECT(
		table.ProductUsers.AllColumns,
	).FROM(
		table.ProductUsers,
	).WHERE(
		table.ProductUsers.ID.EQ(postgres.String(id)).AND(
			table.ProductUsers.ProductID.EQ(postgres.String(productID)),
		),
	).LIMIT(1)

	return r.optional(transactor.QueryOptional[model.ProductUsers](ctx, r.db, stmt))
}

func (r *productUserRepositoryImpl) FindByProductAndEmail(
	ctx context.Context,
	productID string,
	email string,
) (functional.Option[user.ProductUser], error) {
	stmt := table.ProductUsers.SELECT(
		table.ProductUsers.AllColumns,
	).FROM(
		table.ProductUsers,
	).WHERE(
		table.ProductUsers.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductUsers.Email.EQ(postgres.String(email)),
		),
	).LIMIT(1)

	return r.optional(transactor.QueryOptional[model.ProductUsers](ctx, r.db, stmt))
}

func (r *productUserRepositoryImpl) FindByExternalID(
	ctx context.Context,
	productID string,
	externalID string,
) (functional.Option[user.ProductUser], error) {
	stmt := table.ProductUsers.SELECT(
		table.ProductUsers.AllColumns,
	).FROM(
		table.ProductUsers,
	).WHERE(
		table.ProductUsers.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductUsers.ExternalID.EQ(postgres.String(externalID)),
		),
	).LIMIT(1)

	return r.optional(transactor.QueryOptional[model.ProductUsers](ctx, r.db, stmt))
}

const (
	productUserEmailUniqueConstraint      = "product_users_product_id_email_key"
	productUserExternalIDUniqueConstraint = "idx_product_users_external_lookup"
)

// ErrProductUserEmailExists is returned by Create when the product already has
// a user with that email: the unique constraint settles the race the service
// pre-check cannot.
var ErrProductUserEmailExists = errors.New("product user email already exists in product")

// ErrProductUserExternalIDExists is returned by Create when the product already
// has a user with that external ID, for the same reason.
var ErrProductUserExternalIDExists = errors.New("product user external ID already exists in product")

// ErrProductUserEmailConflict is matched when a Clerk sync encounters a
// different external_id already holding the target email in the same product.
var ErrProductUserEmailConflict = errors.New("product user email belongs to another external ID")

// ProductUserEmailConflictError carries the existing holder's product user ID
// so callers can log and audit the conflict.
type ProductUserEmailConflictError struct {
	Email    string
	HolderID string
}

func (e *ProductUserEmailConflictError) Error() string {
	return fmt.Sprintf("product user email %q is already held by product user %s", e.Email, e.HolderID)
}

func (e *ProductUserEmailConflictError) Unwrap() error {
	return ErrProductUserEmailConflict
}

func (r *productUserRepositoryImpl) Create(
	ctx context.Context,
	newUser user.ProductUser,
) (user.ProductUser, error) {
	entity := r.productUserMapper.ToEntity(newUser)

	stmt := table.ProductUsers.INSERT(
		productUsersUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductUsers.AllColumns)

	return r.one(transactor.Query[model.ProductUsers](ctx, r.db, stmt).
		OnUnique(ErrProductUserEmailExists, productUserEmailUniqueConstraint).
		OnUnique(ErrProductUserExternalIDExists, productUserExternalIDUniqueConstraint).
		Value())
}

func (r *productUserRepositoryImpl) Update(
	ctx context.Context,
	productID string,
	id string,
	entity user.ProductUser,
) (user.ProductUser, error) {
	entity.UpdatedAt = time.Now()
	entityToUpdate := r.productUserMapper.ToEntity(entity)

	updateStmt := table.ProductUsers.UPDATE(
		productUsersUpdatableColumns().Except(
			table.ProductUsers.ID,
			table.ProductUsers.ProductID,
		),
	).MODEL(
		entityToUpdate,
	).WHERE(
		table.ProductUsers.ID.EQ(postgres.String(id)).AND(
			table.ProductUsers.ProductID.EQ(postgres.String(productID)),
		),
	).RETURNING(table.ProductUsers.AllColumns)

	return r.one(transactor.Query[model.ProductUsers](ctx, r.db, updateStmt).Value())
}

var ErrProductUserExternalIDMissing = errors.New("product user upsert requires an external ID")

func (r *productUserRepositoryImpl) UpsertByExternalID(
	ctx context.Context,
	entity user.ProductUser,
) (user.ProductUser, bool, error) {
	if entity.ExternalID == nil {
		return user.ProductUser{}, false, ErrProductUserExternalIDMissing
	}
	dbEntity := r.productUserMapper.ToEntity(entity)

	insertStmt := table.ProductUsers.INSERT(
		productUsersUpdatableColumns(),
	).MODEL(dbEntity).
		ON_CONFLICT(table.ProductUsers.ProductID, table.ProductUsers.ExternalID).
		WHERE(table.ProductUsers.ExternalID.IS_NOT_NULL()).
		DO_NOTHING().
		RETURNING(table.ProductUsers.AllColumns)

	inserted, err := r.optional(transactor.QueryOptional[model.ProductUsers](ctx, r.db, insertStmt))
	if err != nil {
		return user.ProductUser{}, false, err
	}
	if inserted.IsPresent() {
		return inserted.Value(), true, nil
	}

	updateStmt := table.ProductUsers.UPDATE(
		table.ProductUsers.Email,
		table.ProductUsers.Name,
		table.ProductUsers.Status,
	).MODEL(dbEntity).
		WHERE(
			table.ProductUsers.ProductID.EQ(postgres.String(dbEntity.ProductID)).AND(
				table.ProductUsers.ExternalID.EQ(postgres.String(*entity.ExternalID)),
			),
		).
		RETURNING(table.ProductUsers.AllColumns)

	updated, err := r.one(transactor.Query[model.ProductUsers](ctx, r.db, updateStmt).Value())
	if err != nil {
		return user.ProductUser{}, false, err
	}

	return updated, false, nil
}

func (r *productUserRepositoryImpl) LockClerkUser(ctx context.Context, productID, externalID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"clerk-user:"+productID+":"+externalID,
	)
	return err
}

func (r *productUserRepositoryImpl) FindTombstone(
	ctx context.Context, productID, externalID string,
) (functional.Option[time.Time], error) {
	stmt := table.ProductUserTombstones.SELECT(
		table.ProductUserTombstones.DeletedAt,
	).FROM(
		table.ProductUserTombstones,
	).WHERE(
		table.ProductUserTombstones.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductUserTombstones.ExternalID.EQ(postgres.String(externalID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, func(row model.ProductUserTombstones) time.Time {
		return row.DeletedAt
	})
}

func (r *productUserRepositoryImpl) PutTombstone(
	ctx context.Context, productID, externalID string, deletedAt time.Time,
) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(
		ctx,
		`INSERT INTO product_user_tombstones (product_id, external_id, deleted_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (product_id, external_id)
		 DO UPDATE SET deleted_at = GREATEST(product_user_tombstones.deleted_at, EXCLUDED.deleted_at)`,
		productID, externalID, deletedAt.UTC(),
	)
	return err
}

func (r *productUserRepositoryImpl) InsertIfAbsent(
	ctx context.Context, entity user.ProductUser,
) (functional.Option[user.ProductUser], error) {
	dbEntity := r.productUserMapper.ToEntity(entity)
	stmt := table.ProductUsers.INSERT(
		table.ProductUsers.ID,
		table.ProductUsers.ProductID,
		table.ProductUsers.Email,
		table.ProductUsers.Name,
		table.ProductUsers.ExternalID,
		table.ProductUsers.Status,
		table.ProductUsers.LastSyncedEventAt,
	).MODEL(dbEntity).
		ON_CONFLICT().
		DO_NOTHING().
		RETURNING(table.ProductUsers.AllColumns)

	return r.optional(transactor.QueryOptional[model.ProductUsers](ctx, r.db, stmt))
}

func (r *productUserRepositoryImpl) AdoptExternalID(
	ctx context.Context,
	productID, id, externalID, name string,
	status user.ProductUserStatus,
	eventAt *time.Time,
) (functional.Option[user.ProductUser], error) {
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	var syncedAt any
	if eventAt != nil {
		syncedAt = eventAt.UTC()
	}

	rows, err := transactor.Executor(ctx, r.db).QueryContext(
		ctx,
		`UPDATE product_users
		 SET external_id = $1,
		     name = $2,
		     status = $3,
		     last_synced_event_at = CASE
		         WHEN $4::timestamptz IS NULL THEN last_synced_event_at
		         ELSE GREATEST(COALESCE(last_synced_event_at, $4::timestamptz), $4::timestamptz)
		     END
		 WHERE product_id = $5 AND id = $6 AND external_id IS NULL
		 RETURNING id, product_id, email, name, external_id, status, created_at, updated_at, last_synced_event_at`,
		externalID, namePtr, string(status), syncedAt, productID, id,
	)
	if err != nil {
		if pgerr.IsUniqueViolation(err, productUserExternalIDUniqueConstraint) {
			return functional.None[user.ProductUser](), ErrProductUserExternalIDExists
		}
		return functional.None[user.ProductUser](), err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			if pgerr.IsUniqueViolation(rowsErr, productUserExternalIDUniqueConstraint) {
				return functional.None[user.ProductUser](), ErrProductUserExternalIDExists
			}
			return functional.None[user.ProductUser](), rowsErr
		}
		return functional.None[user.ProductUser](), nil
	}
	var dbRow model.ProductUsers
	if scanErr := rows.Scan(
		&dbRow.ID, &dbRow.ProductID, &dbRow.Email, &dbRow.Name,
		&dbRow.ExternalID, &dbRow.Status, &dbRow.CreatedAt, &dbRow.UpdatedAt, &dbRow.LastSyncedEventAt,
	); scanErr != nil {
		return functional.None[user.ProductUser](), scanErr
	}
	mapped, err := r.productUserMapper.ToDomain(dbRow)
	if err != nil {
		return functional.None[user.ProductUser](), err
	}
	return functional.Some(mapped), nil
}

func (r *productUserRepositoryImpl) UpdateSynced(
	ctx context.Context,
	productID, id, email, name string,
	status user.ProductUserStatus,
	eventAt *time.Time,
) (user.ProductUser, error) {
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	var syncedAt any
	if eventAt != nil {
		syncedAt = eventAt.UTC()
	}

	rows, err := transactor.Executor(ctx, r.db).QueryContext(
		ctx,
		`UPDATE product_users
		 SET email = $1,
		     name = $2,
		     status = $3,
		     last_synced_event_at = CASE
		         WHEN $4::timestamptz IS NULL THEN last_synced_event_at
		         ELSE GREATEST(COALESCE(last_synced_event_at, $4::timestamptz), $4::timestamptz)
		     END
		 WHERE product_id = $5 AND id = $6
		 RETURNING id, product_id, email, name, external_id, status, created_at, updated_at, last_synced_event_at`,
		email, namePtr, string(status), syncedAt, productID, id,
	)
	if err != nil {
		if pgerr.IsUniqueViolation(err, productUserEmailUniqueConstraint) {
			return user.ProductUser{}, ErrProductUserEmailExists
		}
		return user.ProductUser{}, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			if pgerr.IsUniqueViolation(rowsErr, productUserEmailUniqueConstraint) {
				return user.ProductUser{}, ErrProductUserEmailExists
			}
			return user.ProductUser{}, rowsErr
		}
		return user.ProductUser{}, sql.ErrNoRows
	}
	var dbRow model.ProductUsers
	if scanErr := rows.Scan(
		&dbRow.ID, &dbRow.ProductID, &dbRow.Email, &dbRow.Name,
		&dbRow.ExternalID, &dbRow.Status, &dbRow.CreatedAt, &dbRow.UpdatedAt, &dbRow.LastSyncedEventAt,
	); scanErr != nil {
		return user.ProductUser{}, scanErr
	}
	return r.productUserMapper.ToDomain(dbRow)
}

func (r *productUserRepositoryImpl) AdvanceLastSyncedEventAt(
	ctx context.Context, productID, id string, eventAt time.Time,
) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(
		ctx,
		`UPDATE product_users
		 SET last_synced_event_at = $1
		 WHERE product_id = $2 AND id = $3
		   AND (last_synced_event_at IS NULL OR last_synced_event_at < $1)`,
		eventAt.UTC(), productID, id,
	)
	return err
}

func (r *productUserRepositoryImpl) CountClerkStampedUsers(ctx context.Context, productID string) (int64, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(
		ctx,
		`SELECT COUNT(*)
		 FROM product_users
		 WHERE product_id = $1 AND external_id IS NOT NULL AND last_synced_event_at IS NOT NULL`,
		productID,
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var count int64
	if rows.Next() {
		if scanErr := rows.Scan(&count); scanErr != nil {
			return 0, scanErr
		}
	}
	return count, rows.Err()
}

func (r *productUserRepositoryImpl) ListClerkStampedExternalIDsBefore(
	ctx context.Context, productID string, before time.Time,
) ([]string, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(
		ctx,
		`SELECT external_id
		 FROM product_users
		 WHERE product_id = $1
		   AND external_id IS NOT NULL
		   AND last_synced_event_at IS NOT NULL
		   AND last_synced_event_at < $2
		   AND created_at < $2
		 ORDER BY id`,
		productID, before.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var externalIDs []string
	for rows.Next() {
		var externalID string
		if scanErr := rows.Scan(&externalID); scanErr != nil {
			return nil, scanErr
		}
		externalIDs = append(externalIDs, externalID)
	}
	return externalIDs, rows.Err()
}

// one maps a row the query returned. The mapper rejects a status outside the
// contract enum, so a bad row fails the read instead of reaching a response.
func (r *productUserRepositoryImpl) one(row model.ProductUsers, err error) (user.ProductUser, error) {
	if err != nil {
		return user.ProductUser{}, err
	}
	return r.productUserMapper.ToDomain(row)
}

func (r *productUserRepositoryImpl) optional(
	found functional.Option[model.ProductUsers], err error,
) (functional.Option[user.ProductUser], error) {
	if err != nil || !found.IsPresent() {
		return functional.None[user.ProductUser](), err
	}
	mapped, err := r.productUserMapper.ToDomain(found.Value())
	if err != nil {
		return functional.None[user.ProductUser](), err
	}
	return functional.Some(mapped), nil
}
