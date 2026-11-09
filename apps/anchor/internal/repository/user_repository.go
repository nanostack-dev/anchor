package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/auth"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"

	"anchor/internal/mapper"
)

// Remove global variable, use a function instead.
func usersUpdatableColumns() postgres.ColumnList {
	return table.Users.AllColumns.Except(
		table.Users.CreatedAt, table.Users.UpdatedAt,
	)
}

// userToDomain is the stateless mapper that the generated base calls.
func userToDomain(row model.Users) auth.User {
	return (&mapper.UserMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[UserRepository, userRepositoryImpl](
		table.Users, userToDomain,
	)
}

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (functional.Option[auth.User], error)
	Create(
		ctx context.Context,
		user auth.User,
	) (auth.User, error)
}

type userRepositoryImpl struct {
	usersBase

	db               *sql.DB
	userMapper       *mapper.UserMapper
	tenantMapper     *mapper.TenantMapper
	invitationMapper *mapper.InvitationMapper
	logger           *slog.Logger
}

const userEmailUniqueConstraint = "users_email_key"

// ErrUserEmailExists is returned by Create when a user already has that email.
// Two invitations for one email from different tenants do not serialize on an
// invitation row, so the unique constraint settles that race.
var ErrUserEmailExists = errors.New("user email already exists")

func (u *userRepositoryImpl) Create(
	ctx context.Context, user auth.User,
) (auth.User, error) {
	stmt := table.Users.INSERT(
		usersUpdatableColumns(),
	).MODEL(u.userMapper.ToEntity(user)).RETURNING(table.Users.AllColumns)

	return transactor.QueryMap(
		ctx, u.db, stmt, u.userMapper.ToDomain,
	).OnUnique(ErrUserEmailExists, userEmailUniqueConstraint).Value()
}

func NewUserRepository(
	db *sql.DB,
	userMapper *mapper.UserMapper,
	tenantMapper *mapper.TenantMapper,
	invitationMapper *mapper.InvitationMapper,
	logger *slog.Logger,
) UserRepository {
	return &userRepositoryImpl{
		usersBase:        newUsersBase(db),
		db:               db,
		userMapper:       userMapper,
		tenantMapper:     tenantMapper,
		invitationMapper: invitationMapper,
		logger:           logger.With(slog.String("component", "user_repository")),
	}
}
