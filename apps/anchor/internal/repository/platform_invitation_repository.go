package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/invitation"
	"anchor/internal/mapper"
)

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
	FindByTenantIDAndEmail(
		ctx context.Context, tenantID string, email string,
	) functional.Option[invitation.PlatformInvitation]
	FindByCodeAndEmail(
		ctx context.Context, code string, email string,
	) functional.Option[invitation.PlatformInvitation]
	DeleteByTenantIDAndID(
		ctx context.Context, tenantID string, code string,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation],
	) (search.Result[invitation.PlatformInvitation], error)
}

type invitationRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.InvitationMapper // Use concrete type
	logger zerolog.Logger
}

func NewPlatformInvitationRepository(
	db *sql.DB,
	mapper *mapper.InvitationMapper,
	logger zerolog.Logger,
) InvitationRepository {
	return &invitationRepositoryImpl{
		db:     db,
		mapper: mapper,
		logger: logger.With().Str("component", "invitation_repository").Logger(),
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
			table.PlatformInvitations.Email.LIKE(postgres.String("%" + *input.FullTextSearch + "%")),
		)
	}
	return transactor.Page(r.db, r.mapper.ToDomain, table.PlatformInvitations.AllColumns).
		From(table.PlatformInvitations).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Sort,
			map[invitation.SortFieldPlatformInvitation]postgres.Column{
				invitation.SortFieldPlatformInvitationCreatedAt: table.PlatformInvitations.CreatedAt,
				invitation.SortFieldPlatformInvitationUpdatedAt: table.PlatformInvitations.UpdatedAt,
				invitation.SortFieldPlatformInvitationEmail:     table.PlatformInvitations.Email,
			},
		)...).
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

func (r *invitationRepositoryImpl) FindByCodeAndEmail(
	ctx context.Context, code string, email string,
) functional.Option[invitation.PlatformInvitation] {
	stmt := table.PlatformInvitations.SELECT(
		table.PlatformInvitations.AllColumns,
	).WHERE(
		table.PlatformInvitations.Code.EQ(postgres.String(code)).
			AND(table.PlatformInvitations.Email.EQ(postgres.String(email))),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *invitationRepositoryImpl) FindByTenantIDAndEmail(
	ctx context.Context, tenantID string, email string,
) functional.Option[invitation.PlatformInvitation] {
	stmt := table.PlatformInvitations.SELECT(
		table.PlatformInvitations.AllColumns,
	).WHERE(
		table.PlatformInvitations.Email.EQ(postgres.String(email)).
			AND(table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID))),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
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
