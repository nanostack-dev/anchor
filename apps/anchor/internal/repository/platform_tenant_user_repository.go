package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

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
		ctx context.Context, platformUser platform.User,
	) (platform.User, error)
	FindByTenantIDAndUserID(
		ctx context.Context, tenantID string, userID string,
	) (*platform.User, error)
	FindByTenantIDAndID(
		ctx context.Context, tenantID string, userID string,
	) (*platform.User, error)
	FindByTenantIDAndEmail(
		ctx context.Context, tenantID string, email string,
	) (*platform.User, error)
	DeleteByID(
		ctx context.Context, tenantID string, userID string,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser],
	) (search.Result[platform.User], error)
}

type platformTenantUserRepositoryImpl struct {
	db                 *sql.DB
	platformUserMapper *mapper.PlatformUserMapper
	logger             zerolog.Logger
}

func (r *platformTenantUserRepositoryImpl) Create(
	ctx context.Context, platformUser platform.User,
) (platform.User, error) {
	// Convert domain object to database entity using mapper
	entity := r.platformUserMapper.ToEntity(platformUser)

	stmt := table.PlatformUsers.INSERT(
		platformUsersUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformUsers.AllColumns)

	result, err := transactor.QueryMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		},
	).Value()
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
	ctx context.Context, tenantID string, userID string,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.UserID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		},
	).Value()
}

func (r *platformTenantUserRepositoryImpl) FindByTenantIDAndID(
	ctx context.Context, tenantID string, userID string,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.ID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		},
	).Value()
}

func (r *platformTenantUserRepositoryImpl) FindByTenantIDAndEmail(
	ctx context.Context, tenantID string, email string,
) (*platform.User, error) {
	stmt := table.PlatformUsers.SELECT(
		table.PlatformUsers.AllColumns,
	).WHERE(
		table.PlatformUsers.Email.EQ(postgres.String(email)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity model.PlatformUsers) platform.User {
			return r.platformUserMapper.ToDomain(entity)
		},
	).Value()
}

func (r *platformTenantUserRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, userID string,
) error {
	stmt := table.PlatformUsers.DELETE().WHERE(
		table.PlatformUsers.ID.EQ(postgres.String(userID)).AND(
			table.PlatformUsers.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *platformTenantUserRepositoryImpl) SearchByTenantID(
	ctx context.Context, tenantID string,
	input search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser],
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
			roles := make([]string, len(input.Filter.Roles))
			for i, role := range input.Filter.Roles {
				roles[i] = strings.ToLower(string(role))
			}
			expressions := jetx.ToStringExpressions(roles)
			whereStmt = whereStmt.AND(postgres.LOWER(table.PlatformUsers.Role).IN(expressions...))
		}
	}
	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			table.PlatformUsers.Email.LIKE(postgres.String("%" + *input.FullTextSearch + "%")).
				OR(table.PlatformUsers.Name.LIKE(postgres.String("%" + *input.FullTextSearch + "%"))),
		)
	}
	return transactor.Page(r.db, r.platformUserMapper.ToDomain, table.PlatformUsers.AllColumns).
		From(table.PlatformUsers).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Sort,
			map[platform.SortFieldPlatformUser]postgres.Column{
				platform.SortFieldPlatformUserCreatedAt: table.PlatformUsers.CreatedAt,
				platform.SortFieldPlatformUserUpdatedAt: table.PlatformUsers.UpdatedAt,
				platform.SortFieldPlatformUserEmail:     table.PlatformUsers.Email,
				platform.SortFieldPlatformUserRole:      table.PlatformUsers.Role,
			},
		)...).
		Run(ctx, input.Pagination).
		Value()
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
