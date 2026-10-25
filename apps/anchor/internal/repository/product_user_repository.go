package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ ProductUserRepository = (*productUserRepositoryImpl)(nil)

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
	DeleteByID(ctx context.Context, productID string, id string) (bool, error)
	DeleteByExternalID(ctx context.Context, productID string, externalID string) error
	SearchByProductID(
		ctx context.Context,
		productID string,
		request search.Request[user.SearchProductUserFilter, user.SortFieldProductUser],
	) (search.Result[user.ProductUser], error)
}

type productUserRepositoryImpl struct {
	db                *sql.DB
	productUserMapper *mapper.ProductUserMapper
	logger            zerolog.Logger
}

func NewProductUserRepository(
	db *sql.DB,
	productUserMapper *mapper.ProductUserMapper,
	logger zerolog.Logger,
) ProductUserRepository {
	return &productUserRepositoryImpl{
		db:                db,
		productUserMapper: productUserMapper,
		logger: logger.With().Str(
			"component", "product_user_repository",
		).Logger(),
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

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		r.productUserMapper.ToDomain,
	)
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

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		r.productUserMapper.ToDomain,
	).Value()
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

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		r.productUserMapper.ToDomain,
	)
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

	return transactor.QueryMap(
		ctx,
		r.db,
		stmt,
		r.productUserMapper.ToDomain,
	).OnUnique(ErrProductUserEmailExists, productUserEmailUniqueConstraint).
		OnUnique(ErrProductUserExternalIDExists, productUserExternalIDUniqueConstraint).
		Value()
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

	return transactor.QueryMap(
		ctx, r.db, updateStmt, r.productUserMapper.ToDomain,
	).Value()
}

func (r *productUserRepositoryImpl) DeleteByID(
	ctx context.Context,
	productID string,
	id string,
) (bool, error) {
	stmt := table.ProductUsers.DELETE().WHERE(
		table.ProductUsers.ID.EQ(postgres.String(id)).AND(
			table.ProductUsers.ProductID.EQ(postgres.String(productID)),
		),
	)

	result, err := stmt.ExecContext(ctx, transactor.Executor(ctx, r.db))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
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

	inserted, err := transactor.QueryOptionalMap(
		ctx, r.db, insertStmt, r.productUserMapper.ToDomain,
	)
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

	updated, err := transactor.QueryMap(
		ctx, r.db, updateStmt, r.productUserMapper.ToDomain,
	).Value()
	if err != nil {
		return user.ProductUser{}, false, err
	}

	return updated, false, nil
}

func (r *productUserRepositoryImpl) DeleteByExternalID(
	ctx context.Context,
	productID string,
	externalID string,
) error {
	stmt := table.ProductUsers.DELETE().WHERE(
		table.ProductUsers.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductUsers.ExternalID.EQ(postgres.String(externalID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productUserRepositoryImpl) SearchByProductID(
	ctx context.Context,
	productID string,
	request search.Request[user.SearchProductUserFilter, user.SortFieldProductUser],
) (search.Result[user.ProductUser], error) {
	whereStmt := table.ProductUsers.ProductID.EQ(postgres.String(productID))

	if request.Filter != nil { //nolint:nestif // standard filter mapping pattern
		if len(request.Filter.IDs) > 0 {
			expressions := jetx.ToStringExpressions(request.Filter.IDs)
			whereStmt = whereStmt.AND(table.ProductUsers.ID.IN(expressions...))
		}

		if len(request.Filter.Emails) > 0 {
			expressions := jetx.ToStringExpressions(request.Filter.Emails)
			whereStmt = whereStmt.AND(table.ProductUsers.Email.IN(expressions...))
		}

		if len(request.Filter.Names) > 0 {
			expressions := jetx.ToStringExpressions(request.Filter.Names)
			whereStmt = whereStmt.AND(table.ProductUsers.Name.IN(expressions...))
		}

		if len(request.Filter.Statuses) > 0 {
			expressions := jetx.ToStringExpressions(request.Filter.Statuses)
			whereStmt = whereStmt.AND(table.ProductUsers.Status.IN(expressions...))
		}

		if len(request.Filter.ExternalIDs) > 0 {
			expressions := jetx.ToStringExpressions(request.Filter.ExternalIDs)
			whereStmt = whereStmt.AND(table.ProductUsers.ExternalID.IN(expressions...))
		}
	}

	if request.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			table.ProductUsers.Name.LIKE(postgres.String("%" + *request.FullTextSearch + "%")).
				OR(table.ProductUsers.Email.LIKE(postgres.String("%" + *request.FullTextSearch + "%"))),
		)
	}

	order, err := transactor.StrictKeys(
		request.Sort,
		map[user.SortFieldProductUser]transactor.Key[model.ProductUsers]{
			user.SortFieldProductUserCreatedAt: transactor.TimeKey(
				table.ProductUsers.CreatedAt, func(row model.ProductUsers) time.Time { return row.CreatedAt },
			),
			user.SortFieldProductUserUpdatedAt: transactor.TimeKey(
				table.ProductUsers.UpdatedAt, func(row model.ProductUsers) time.Time { return row.UpdatedAt },
			),
			user.SortFieldProductUserEmail: transactor.StringKey(
				table.ProductUsers.Email, func(row model.ProductUsers) string { return row.Email },
			),
			user.SortFieldProductUserName: transactor.NullableStringKey(
				table.ProductUsers.Name, func(row model.ProductUsers) *string { return row.Name },
			),
			user.SortFieldProductUserStatus: transactor.StringKey(
				table.ProductUsers.Status, func(row model.ProductUsers) string { return row.Status },
			),
		},
		transactor.StringKey(table.ProductUsers.ID, func(row model.ProductUsers) string { return row.ID }),
	)
	if err != nil {
		return search.Result[user.ProductUser]{}, err
	}

	return transactor.Page(r.db, r.productUserMapper.ToDomain, table.ProductUsers.AllColumns).
		From(table.ProductUsers).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, request.Pagination).
		Value()
}
