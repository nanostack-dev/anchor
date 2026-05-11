package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/shared/toolkit/search"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"

	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/db/gen/anchor/public/model"
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
		ctx context.Context, inv invitation.PlatformInvitation, options *toolkit.DBOptions,
	) (invitation.PlatformInvitation, error)
	FindByTenantIDAndEmail(
		ctx context.Context, tenantID string, email string, options *toolkit.DBOptions,
	) (*invitation.PlatformInvitation, error)
	FindByCodeAndEmail(
		ctx context.Context, code string, email string, options *toolkit.DBOptions,
	) (*invitation.PlatformInvitation, error)
	DeleteByTenantIDAndID(
		ctx context.Context, tenantID string, code string, options *toolkit.DBOptions,
	) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation],
		options *toolkit.DBOptions,
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
	options *toolkit.DBOptions,
) (search.Result[invitation.PlatformInvitation], error) {
	whereStmt := table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID))

	if input.Filter != nil {
		if len(input.Filter.Emails) > 0 {
			expressions := search.ToStringExpressions(input.Filter.Emails)
			whereStmt = whereStmt.AND(table.PlatformInvitations.Email.IN(expressions...))
		}

		if len(input.Filter.IDs) > 0 {
			expressions := search.ToStringExpressions(input.Filter.IDs)
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
	query := table.PlatformInvitations.SELECT(
		table.PlatformInvitations.AllColumns,
	).WHERE(whereStmt)

	resultCount, err := toolkit.QueryCountWithBoolExpression(
		ctx, r.db, table.PlatformInvitations, whereStmt, options,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"tenantID", tenantID,
		).Msg("failed to count platform invitations")
		return search.Result[invitation.PlatformInvitation]{}, err
	}

	if input.Sort != nil {
		if len(input.Sort) > 0 {
			for _, sort := range input.Sort {
				switch sort.Field {
				case invitation.SortFieldPlatformInvitationCreatedAt:
					fieldToOrderBy := table.PlatformInvitations.CreatedAt
					query = query.ORDER_BY(
						search.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case invitation.SortFieldPlatformInvitationUpdatedAt:
					fieldToOrderBy := table.PlatformInvitations.UpdatedAt
					query = query.ORDER_BY(
						search.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case invitation.SortFieldPlatformInvitationEmail:
					fieldToOrderBy := table.PlatformInvitations.Email
					query = query.ORDER_BY(
						search.OrderBy(fieldToOrderBy, sort.Direction),
					)
				}
			}
		}
	}
	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))
	slice, err := toolkit.QueryMapSlice(ctx, r.db, query, r.mapper.ToDomain, options)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"tenantID", tenantID,
		).Msg("failed to search platform invitations")
		return search.Result[invitation.PlatformInvitation]{}, err
	}
	return search.Result[invitation.PlatformInvitation]{
		Items: slice,
		Total: resultCount,
		Count: len(slice),
	}, nil
}

func (r *invitationRepositoryImpl) Create(
	ctx context.Context, inv invitation.PlatformInvitation, options *toolkit.DBOptions,
) (invitation.PlatformInvitation, error) {
	entity := r.mapper.ToEntity(inv)
	stmt := table.PlatformInvitations.INSERT(
		platformInvitationsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformInvitations.AllColumns)

	return toolkit.QueryMap[model.PlatformInvitations, invitation.PlatformInvitation](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *invitationRepositoryImpl) FindByCodeAndEmail(
	ctx context.Context, code string, email string, options *toolkit.DBOptions,
) (*invitation.PlatformInvitation, error) {
	stmt := table.PlatformInvitations.SELECT(
		table.PlatformInvitations.AllColumns,
	).WHERE(
		table.PlatformInvitations.Code.EQ(postgres.String(code)).
			AND(table.PlatformInvitations.Email.EQ(postgres.String(email))),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.PlatformInvitations, invitation.PlatformInvitation](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *invitationRepositoryImpl) FindByTenantIDAndEmail(
	ctx context.Context, tenantID string, email string, options *toolkit.DBOptions,
) (*invitation.PlatformInvitation, error) {
	stmt := table.PlatformInvitations.SELECT(
		table.PlatformInvitations.AllColumns,
	).WHERE(
		table.PlatformInvitations.Email.EQ(postgres.String(email)).
			AND(table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID))),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.PlatformInvitations, invitation.PlatformInvitation](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *invitationRepositoryImpl) DeleteByTenantIDAndID(
	ctx context.Context, tenantID string, invitationID string, options *toolkit.DBOptions,
) error {
	stmt := table.PlatformInvitations.DELETE().
		WHERE(
			table.PlatformInvitations.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.PlatformInvitations.ID.EQ(postgres.String(invitationID))),
		)

	return toolkit.Exec(ctx, r.db, stmt, options)
}
