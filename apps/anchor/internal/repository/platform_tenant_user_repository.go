package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/platform"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

// Remove global variable, use a function instead.
func platformUsersUpdatableColumns() postgres.ColumnList {
	return table.PlatformUsers.AllColumns.Except(
		table.PlatformUsers.CreatedAt, table.PlatformUsers.UpdatedAt,
	)
}

type PlatformTenantUserRepository interface {
	Create(
		ctx context.Context, platformUser platform.User, options *jetx.DBOptions,
	) (platform.User, error)
	FindByTenantIDAndUserID(
		ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
	) (*platform.User, error)
	FindByTenantIDAndID(
		ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
	) (*platform.User, error)
	FindByTenantIDAndEmail(
		ctx context.Context, tenantID string, email string, options *jetx.DBOptions,
	) (*platform.User, error)
	DeleteByID(
		ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser],
		options *jetx.DBOptions,
	) (search.Result[platform.User], error)
}

type platformTenantUserRepositoryImpl struct {
	db                 *sql.DB
	platformUserMapper *mapper.PlatformUserMapper
	logger             zerolog.Logger
}

func (r *platformTenantUserRepositoryImpl) Create(
	ctx context.Context, platformUser platform.User, options *jetx.DBOptions,
) (platform.User, error) {
	// Convert domain object to database entity using mapper
	entity := r.platformUserMapper.ToEntity(platformUser)

	stmt := table.PlatformUsers.INSERT(
		platformUsersUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformUsers.AllColumns)

	result, err := jetx.QueryMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		}, options,
	)
	if err != nil {
		r.logger.Error().Err(err).
			Str("platform_user_id", platformUser.ID).
			Str("platform_tenant_id", platformUser.PlatformTenantID).
			Msg("Failed to create platform user")
		return platform.User{}, err
	}

	return result, nil
}

func (r *platformTenantUserRepositoryImpl) FindByTenantIDAndUserID(
	ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.UserID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return jetx.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		}, options,
	)
}

func (r *platformTenantUserRepositoryImpl) FindByTenantIDAndID(
	ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.ID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return jetx.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		}, options,
	)
}

func (r *platformTenantUserRepositoryImpl) FindByTenantIDAndEmail(
	ctx context.Context, tenantID string, email string, options *jetx.DBOptions,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.Email.EQ(postgres.String(email)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return jetx.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		}, options,
	)
}

func (r *platformTenantUserRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, userID string, options *jetx.DBOptions,
) error {
	stmt := table.PlatformUsers.DELETE().WHERE(
		table.PlatformUsers.ID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)

	return jetx.Exec(ctx, r.db, stmt, options)
}

func (r *platformTenantUserRepositoryImpl) SearchByTenantID(
	ctx context.Context, tenantID string,
	input search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser],
	options *jetx.DBOptions,
) (search.Result[platform.User], error) {
	whereStmt := table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID))

	if input.Filter != nil {
		if len(input.Filter.IDs) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.IDs)
			whereStmt = whereStmt.AND(table.PlatformUsers.ID.IN(expressions...))
		}

		if len(input.Filter.Emails) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.Emails)
			whereStmt = whereStmt.AND(table.PlatformUsers.Email.IN(expressions...))
		}

		if len(input.Filter.Roles) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.Roles)
			whereStmt = whereStmt.AND(table.PlatformUsers.Role.IN(expressions...))
		}
	}
	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			table.PlatformUsers.Email.LIKE(postgres.String("%" + *input.FullTextSearch + "%")).
				OR(table.PlatformUsers.Name.LIKE(postgres.String("%" + *input.FullTextSearch + "%"))),
		)
	}
	query := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(whereStmt)
	total, err := jetx.QueryCountWithBoolExpression(
		ctx, r.db, table.PlatformUsers, whereStmt, options,
	)
	if err != nil {
		return search.Result[platform.User]{}, err
	}
	if len(input.Sort) > 0 {
		for _, sort := range input.Sort {
			switch sort.Field {
			case platform.SortFieldPlatformUserCreatedAt:
				fieldToOrderBy := table.PlatformUsers.CreatedAt
				query = query.ORDER_BY(
					jetx.OrderBy(fieldToOrderBy, sort.Direction),
				)
			case platform.SortFieldPlatformUserUpdatedAt:
				fieldToOrderBy := table.PlatformUsers.UpdatedAt
				query = query.ORDER_BY(
					jetx.OrderBy(fieldToOrderBy, sort.Direction),
				)
			case platform.SortFieldPlatformUserEmail:
				fieldToOrderBy := table.PlatformUsers.Email
				query = query.ORDER_BY(
					jetx.OrderBy(fieldToOrderBy, sort.Direction),
				)
			case platform.SortFieldPlatformUserRole:
				fieldToOrderBy := table.PlatformUsers.Role
				query = query.ORDER_BY(
					jetx.OrderBy(fieldToOrderBy, sort.Direction),
				)
			}
		}
	}

	// Apply pagination
	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	// Execute query
	entities, err := jetx.QueryMapSlice(ctx, r.db, query, r.platformUserMapper.ToDomain, options)
	if err != nil {
		return search.Result[platform.User]{}, err
	}

	return search.Result[platform.User]{
		Items: entities,
		Total: total,
		Count: len(entities),
	}, nil
}

func NewPlatformTenantUserRepository(
	db *sql.DB,
	platformUserMapper *mapper.PlatformUserMapper,
	logger zerolog.Logger,
) PlatformTenantUserRepository {
	return &platformTenantUserRepositoryImpl{
		db:                 db,
		platformUserMapper: platformUserMapper,
		logger: logger.With().Str(
			"component", "platform_tenant_user_repository",
		).Logger(),
	}
}
