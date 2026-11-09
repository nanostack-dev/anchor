package repository

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/platform"
	"anchor/internal/mapper"

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

	// POST /v1/platform-users/search
	nanorepo.Search[model.PlatformUsers]("PlatformUser", table.PlatformUsers,
		nanorepo.Map(platformUserToDomain),
		nanorepo.Scope(table.PlatformUsers.PlatformTenantID),
		nanorepo.Filter(table.PlatformUsers.ID, nanorepo.In),
		nanorepo.Filter(table.PlatformUsers.Email, nanorepo.In),
		nanorepo.Filter(table.PlatformUsers.Role, nanorepo.InFold),
		nanorepo.FullText(table.PlatformUsers.Email, table.PlatformUsers.Name),
		nanorepo.Sort(
			table.PlatformUsers.CreatedAt, table.PlatformUsers.UpdatedAt,
			table.PlatformUsers.Email, table.PlatformUsers.Role,
		),
		nanorepo.DefaultSort(table.PlatformUsers.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
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
	SearchByTenantID(ctx context.Context, q PlatformUserSearch) (search.Result[platform.User], error)
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
