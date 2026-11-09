package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/invitation"
	"anchor/internal/mapper"
	"anchor/internal/searchx"
)

// invitationToDomain is the stateless mapper that the generated base calls.
func invitationToDomain(row model.PlatformInvitations) invitation.PlatformInvitation {
	return (&mapper.InvitationMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[InvitationRepository, invitationRepositoryImpl](
		table.PlatformInvitations, invitationToDomain,
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
	FindByPlatformTenantIDAndEmail(
		ctx context.Context, platformTenantID string, email string,
	) (functional.Option[invitation.PlatformInvitation], error)
	// FindByCodeAndEmailInternal reads across tenants: a user accepts an
	// invitation before anchor knows the tenant.
	FindByCodeAndEmailInternal(
		ctx context.Context, code string, email string,
	) (functional.Option[invitation.PlatformInvitation], error)
	DeleteByTenantIDAndID(
		ctx context.Context, tenantID string, code string,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation],
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

func (r *invitationRepositoryImpl) SearchByTenantID(
	ctx context.Context, tenantID string,
	input search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation],
) (search.Result[invitation.PlatformInvitation], error) {
	whereStmt := table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID))

	if input.Filter != nil {
		if len(input.Filter.Emails) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.Emails)
			whereStmt = whereStmt.AND(table.PlatformInvitations.Email.IN(expressions...))
		}

		if len(input.Filter.IDs) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.IDs)
			whereStmt = whereStmt.AND(table.PlatformInvitations.ID.IN(expressions...))
		}

		if input.Filter.Code != nil {
			whereStmt = whereStmt.AND(
				table.PlatformInvitations.Code.EQ(postgres.String(*input.Filter.Code)),
			)
		}
	}
	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			searchx.Contains(*input.FullTextSearch, table.PlatformInvitations.Email),
		)
	}
	order, err := transactor.StrictKeys(
		input.Sort,
		map[invitation.SortFieldPlatformInvitation]transactor.Key[model.PlatformInvitations]{
			invitation.SortFieldPlatformInvitationCreatedAt: transactor.TimeKey(
				table.PlatformInvitations.CreatedAt,
				func(row model.PlatformInvitations) time.Time { return row.CreatedAt },
			),
			invitation.SortFieldPlatformInvitationUpdatedAt: transactor.TimeKey(
				table.PlatformInvitations.UpdatedAt,
				func(row model.PlatformInvitations) time.Time { return row.UpdatedAt },
			),
			invitation.SortFieldPlatformInvitationEmail: transactor.StringKey(
				table.PlatformInvitations.Email,
				func(row model.PlatformInvitations) string { return row.Email },
			),
		},
		transactor.StringKey(
			table.PlatformInvitations.ID,
			func(row model.PlatformInvitations) string { return row.ID },
		),
	)
	if err != nil {
		return search.Result[invitation.PlatformInvitation]{}, err
	}

	return transactor.Page(r.db, r.mapper.ToDomain, table.PlatformInvitations.AllColumns).
		From(table.PlatformInvitations).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, input.Pagination).
		Value()
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

func (r *invitationRepositoryImpl) DeleteByTenantIDAndID(
	ctx context.Context, tenantID string, invitationID string,
) error {
	stmt := table.PlatformInvitations.DELETE().
		WHERE(
			table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.PlatformInvitations.ID.EQ(postgres.String(invitationID))),
		)

	return transactor.Exec(ctx, r.db, stmt).Err()
}
