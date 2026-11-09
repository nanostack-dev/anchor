package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

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
		nanorepo.DefaultLimit(10),
	)
}

func productUsersUpdatableColumns() postgres.ColumnList {
	return table.ProductUsers.AllColumns.Except(
		table.ProductUsers.CreatedAt,
		table.ProductUsers.UpdatedAt,
	)
}

type ProductUserRepository interface {
	FindByProductIDAndID(
		ctx context.Context,
		productID string,
		id string,
	) (functional.Option[user.ProductUser], error)
	FindByProductID(
		ctx context.Context, productID string,
	) ([]user.ProductUser, error)
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
	// DeleteByProductIDAndID returns true when it deleted the row.
	DeleteByProductIDAndID(ctx context.Context, productID string, id string) (bool, error)
	DeleteByProductIDAndExternalID(ctx context.Context, productID string, externalID string) error
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

func (r *productUserRepositoryImpl) FindByProductID(
	ctx context.Context,
	productID string,
) ([]user.ProductUser, error) {
	stmt := table.ProductUsers.SELECT(
		table.ProductUsers.AllColumns,
	).FROM(
		table.ProductUsers,
	).WHERE(
		table.ProductUsers.ProductID.EQ(postgres.String(productID)),
	)

	rows, err := transactor.Query[[]model.ProductUsers](ctx, r.db, stmt).Value()
	if err != nil {
		return nil, err
	}
	return r.rows.page(ctx, rows)
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

