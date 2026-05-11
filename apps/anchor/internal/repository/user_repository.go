package repository

import (
	"context"
	"database/sql"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/auth"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"

	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/mapper"
)

// Remove global variable, use a function instead.
func usersUpdatableColumns() postgres.ColumnList {
	return table.Users.AllColumns.Except(
		table.Users.CreatedAt, table.Users.UpdatedAt,
	)
}

var _ UserRepository = (*userRepositoryImpl)(nil)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string, options *toolkit.DBOptions) (*auth.User, error)
	Count(ctx context.Context, options *toolkit.DBOptions) (int64, error)
	Create(
		ctx context.Context,
		user auth.User, options *toolkit.DBOptions,
	) (auth.User, error)
}

type userRepositoryImpl struct {
	db               *sql.DB
	userMapper       *mapper.UserMapper
	tenantMapper     *mapper.TenantMapper
	invitationMapper *mapper.InvitationMapper
	logger           zerolog.Logger
}

func (u *userRepositoryImpl) FindByEmail(
	ctx context.Context, email string, options *toolkit.DBOptions,
) (*auth.User, error) {
	stmt := table.Users.SELECT(
		table.Users.AllColumns,
	).FROM(
		table.Users,
	).WHERE(
		table.Users.Email.EQ(postgres.String(email)),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.Users, auth.User](
		ctx, u.db, stmt,

		u.userMapper.ToDomain, options,
	)
}

func (u *userRepositoryImpl) Count(ctx context.Context, options *toolkit.DBOptions) (
	int64, error,
) {
	return toolkit.QueryCount(ctx, u.db, table.Users, options)
}

func (u *userRepositoryImpl) Create(
	ctx context.Context, user auth.User, options *toolkit.DBOptions,
) (auth.User, error) {
	stmt := table.Users.INSERT(
		usersUpdatableColumns(),
	).MODEL(u.userMapper.ToEntity(user)).RETURNING(table.Users.AllColumns)

	return toolkit.QueryMap(
		ctx, u.db, stmt, u.userMapper.ToDomain, options,
	)
}

func NewUserRepository(
	db *sql.DB,
	userMapper *mapper.UserMapper,
	tenantMapper *mapper.TenantMapper,
	invitationMapper *mapper.InvitationMapper,
	logger zerolog.Logger,
) UserRepository {
	return &userRepositoryImpl{
		db:               db,
		userMapper:       userMapper,
		tenantMapper:     tenantMapper,
		invitationMapper: invitationMapper,
		logger: logger.With().Str(
			"component", "user_repository",
		).Logger(),
	}
}
