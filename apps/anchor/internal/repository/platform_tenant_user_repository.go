package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/platform"
	"anchor/internal/mapper"
	"anchor/internal/searchx"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

// platformUserToDomain is the stateless mapper that the generated base calls.
func platformUserToDomain(row model.PlatformUsers) platform.User {
	return (&mapper.PlatformUserMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[PlatformTenantUserRepository, platformTenantUserRepositoryImpl](
		table.PlatformUsers, platformUserToDomain,
	)
}

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
	// FindByPlatformTenantIDAndUserID returns the oldest platform user of the
	// tenant for the user. (platform_tenant_id, user_id) is not unique, so the
	// method orders by created_at and id to pick one row in a fixed way.
	FindByPlatformTenantIDAndUserID(
		ctx context.Context, platformTenantID string, userID string,
	) (functional.Option[platform.User], error)
	FindByPlatformTenantIDAndID(
		ctx context.Context, platformTenantID string, id string,
	) (functional.Option[platform.User], error)
	FindByPlatformTenantIDAndEmail(
		ctx context.Context, platformTenantID string, email string,
	) (functional.Option[platform.User], error)
	DeleteByPlatformTenantIDAndID(
		ctx context.Context, platformTenantID string, id string,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser],
	) (search.Result[platform.User], error)
}

type platformTenantUserRepositoryImpl struct {
	platformUsersBase

	db                 *sql.DB
	platformUserMapper *mapper.PlatformUserMapper
	logger             *slog.Logger
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
		r.logger.ErrorContext(ctx, "Failed to create platform user",
			log.Err(err),
			slog.String("platform_user_id", platformUser.ID),
			slog.String("platform_tenant_id", platformUser.PlatformTenantID),
		)
		return platform.User{}, err
	}

	return result, nil
}

func (r *platformTenantUserRepositoryImpl) FindByPlatformTenantIDAndUserID(
	ctx context.Context, platformTenantID string, userID string,
) (functional.Option[platform.User], error) {
	return r.first(ctx,
		r.where.PlatformTenantID(platformTenantID).AND(r.where.UserID(userID)),
		r.t.CreatedAt.ASC(), r.t.ID.ASC(),
	)
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
			roles := functional.Slice(input.Filter.Roles).Map(func(role platform.TenantRole) string {
				return strings.ToLower(string(role))
			})
			expressions := jetx.ToStringExpressions(roles)
			whereStmt = whereStmt.AND(postgres.LOWER(table.PlatformUsers.Role).IN(expressions...))
		}
	}
	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			searchx.Contains(*input.FullTextSearch, table.PlatformUsers.Email, table.PlatformUsers.Name),
		)
	}
	order, err := transactor.StrictKeys(
		input.Sort,
		map[platform.SortFieldPlatformUser]transactor.Key[model.PlatformUsers]{
			platform.SortFieldPlatformUserCreatedAt: transactor.TimeKey(
				table.PlatformUsers.CreatedAt, func(row model.PlatformUsers) time.Time { return row.CreatedAt },
			),
			platform.SortFieldPlatformUserUpdatedAt: transactor.TimeKey(
				table.PlatformUsers.UpdatedAt, func(row model.PlatformUsers) time.Time { return row.UpdatedAt },
			),
			platform.SortFieldPlatformUserEmail: transactor.StringKey(
				table.PlatformUsers.Email, func(row model.PlatformUsers) string { return row.Email },
			),
			platform.SortFieldPlatformUserRole: transactor.StringKey(
				table.PlatformUsers.Role, func(row model.PlatformUsers) string { return row.Role },
			),
		},
		transactor.StringKey(table.PlatformUsers.ID, func(row model.PlatformUsers) string { return row.ID }),
	)
	if err != nil {
		return search.Result[platform.User]{}, err
	}

	return transactor.Page(r.db, r.platformUserMapper.ToDomain, table.PlatformUsers.AllColumns).
		From(table.PlatformUsers).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, input.Pagination).
		Value()
}

func NewPlatformTenantUserRepository(
	db *sql.DB,
	platformUserMapper *mapper.PlatformUserMapper,
	logger *slog.Logger,
) PlatformTenantUserRepository {
	return &platformTenantUserRepositoryImpl{
		platformUsersBase:  newPlatformUsersBase(db),
		db:                 db,
		platformUserMapper: platformUserMapper,
		logger:             logger.With(slog.String("component", "platform_tenant_user_repository")),
	}
}
