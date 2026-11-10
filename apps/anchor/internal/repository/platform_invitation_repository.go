package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/invitation"
	"anchor/internal/mapper"
)

// invitationToDomain is the stateless mapper that the generated base calls.
func invitationToDomain(row model.PlatformInvitations) invitation.PlatformInvitation {
	return (&mapper.InvitationMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[InvitationRepository, invitationRepositoryImpl](
		table.PlatformInvitations, invitationToDomain,
	)

	// POST /v1/platform-invitations/search, and GET of one invitation.
	nanorepo.Search[model.PlatformInvitations]("PlatformInvitation", table.PlatformInvitations,
		nanorepo.Map(invitationToDomain),
		nanorepo.Scope(table.PlatformInvitations.PlatformTenantID),
		nanorepo.Filter(table.PlatformInvitations.Email, nanorepo.In),
		nanorepo.Filter(table.PlatformInvitations.ID, nanorepo.In),
		nanorepo.FullText(table.PlatformInvitations.Email),
		nanorepo.Sort(
			table.PlatformInvitations.CreatedAt, table.PlatformInvitations.UpdatedAt, table.PlatformInvitations.Email,
		),
		nanorepo.DefaultSort(table.PlatformInvitations.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

// Remove global variable, use a function instead.
func platformInvitationsUpdatableColumns() postgres.ColumnList {
	return table.PlatformInvitations.AllColumns.Except(
		table.PlatformInvitations.CreatedAt, table.PlatformInvitations.UpdatedAt,
	)
}

type InvitationRepository interface {
	Create(
		ctx context.Context, inv invitation.PlatformInvitation,
	) (invitation.PlatformInvitation, error)
	FindByTenantAndEmail(
		ctx context.Context, platformTenantID string, email string,
	) (functional.Option[invitation.PlatformInvitation], error)
	// FindByCodeAndEmailInternal reads across tenants: a user accepts an
	// invitation before anchor knows the tenant.
	FindByCodeAndEmailInternal(
		ctx context.Context, code string, email string,
	) (functional.Option[invitation.PlatformInvitation], error)
	DeleteByTenantAndID(
		ctx context.Context, tenantID string, id string,
	) error
	SearchByTenantID(
		ctx context.Context,
		q PlatformInvitationSearch,
	) (search.Result[invitation.PlatformInvitation], error)
}

type invitationRepositoryImpl struct {
	platformInvitationsBase

	db     *sql.DB
	mapper *mapper.InvitationMapper // Use concrete type
	logger *slog.Logger
}

func NewPlatformInvitationRepository(
	db *sql.DB,
	mapper *mapper.InvitationMapper,
	logger *slog.Logger,
) InvitationRepository {
	return &invitationRepositoryImpl{
		platformInvitationsBase: newPlatformInvitationsBase(db),
		db:                      db,
		mapper:                  mapper,
		logger:                  logger.With(slog.String("component", "invitation_repository")),
	}
}

// invitationTenantEmailUniqueIndex is the UNIQUE (platform_tenant_id, email)
// index from 000001_init — the only uniqueness guard on platform_invitations
// besides the primary key.
const invitationTenantEmailUniqueIndex = "idx_platform_tenant_email"

// ErrInvitationExists is returned by Create when the tenant already has an
// invitation for the address.
//
// The caller's pre-check cannot be race-free: a concurrent inserter's row stays
// invisible until it commits, so both callers pass their check and the loser
// trips the unique index. The index is the only race-free arbiter, so its
// violation is translated here rather than escaping as an unexpected error.
var ErrInvitationExists = errors.New("invitation already exists for tenant and email")

func (r *invitationRepositoryImpl) Create(
	ctx context.Context, inv invitation.PlatformInvitation,
) (invitation.PlatformInvitation, error) {
	entity := r.mapper.ToEntity(inv)
	stmt := table.PlatformInvitations.INSERT(
		platformInvitationsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformInvitations.AllColumns)

	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).
		OnUnique(ErrInvitationExists, invitationTenantEmailUniqueIndex).
		Value()
}
