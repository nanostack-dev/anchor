package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/auth"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"

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
	FindByEmail(ctx context.Context, email string) (*auth.User, error)
	Count(ctx context.Context) (int64, error)
	Create(
		ctx context.Context,
		user auth.User,
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
	ctx context.Context, email string,
) (*auth.User, error) {
	stmt := table.Users.SELECT(
		table.Users.AllColumns,
	).FROM(
		table.Users,
	).WHERE(
		table.Users.Email.EQ(postgres.String(email)),
	).LIMIT(1)

	result := transactor.QueryOptionalMap(
		ctx, u.db, stmt,

		u.userMapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
}

func (u *userRepositoryImpl) Count(ctx context.Context) (
	int64, error,
) {
	return transactor.QueryCount(ctx, u.db, table.Users.SELECT(postgres.COUNT(postgres.STAR))).Value()
}

func (u *userRepositoryImpl) Create(
	ctx context.Context, user auth.User,
) (auth.User, error) {
	stmt := table.Users.INSERT(
		usersUpdatableColumns(),
	).MODEL(u.userMapper.ToEntity(user)).RETURNING(table.Users.AllColumns)

	return transactor.QueryMap(
		ctx, u.db, stmt, u.userMapper.ToDomain,
	).Value()
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
