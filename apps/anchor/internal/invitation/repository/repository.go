package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organizationinvitation"
)

var _ Repository = (*repositoryImpl)(nil)

// Repository persists organization invitations. Every method is scoped by
// product, so a caller cannot reach another Product's invitation by guessing a
// KSUID.
//
// The status of an invitation is never stored. Every read derives it from
// accepted_at and expires_at at the moment of the read.
type Repository interface {
	// LockOrganization takes a row lock on the organization that lasts until the
	// surrounding transaction ends, and reports whether the organization exists
	// in the product. Two writers on the same organization run one after the
	// other, which is what makes "at most one pending invitation per email"
	// safe under concurrent creates. Call it inside a transaction.
	LockOrganization(ctx context.Context, productID, organizationID string) (bool, error)
	Create(ctx context.Context, invitation organizationinvitation.Invitation) (organizationinvitation.Invitation, error)
	FindByID(
		ctx context.Context, productID, organizationID, invitationID string,
	) (functional.Option[organizationinvitation.Invitation], error)
	// FindPendingByEmail returns a pending invitation for the email address in
	// the organization, compared without regard to letter case.
	FindPendingByEmail(
		ctx context.Context, productID, organizationID, email string,
	) (functional.Option[organizationinvitation.Invitation], error)
	// EmailBelongsToMember reports whether a member of the organization has the
	// email address, compared without regard to letter case.
	EmailBelongsToMember(ctx context.Context, productID, organizationID, email string) (bool, error)
	// Search lists the invitations of one organization, or of every
	// organization of the product when organizationID is absent.
	Search(
		ctx context.Context,
		productID string,
		organizationID functional.Option[string],
		req search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField],
	) (search.Result[organizationinvitation.Invitation], error)
	UpdateRoleAndExpiry(
		ctx context.Context, invitation organizationinvitation.Invitation,
	) (organizationinvitation.Invitation, error)
	MarkAccepted(
		ctx context.Context, productID, organizationID, invitationID string, acceptedAt time.Time,
	) (organizationinvitation.Invitation, error)
	Delete(ctx context.Context, productID, organizationID, invitationID string) error
	// ExistsPendingForRole reports whether a pending invitation in the product
	// names the role.
	ExistsPendingForRole(ctx context.Context, productID, roleID string) (bool, error)
	// DeleteNonPendingForRole deletes the accepted and expired invitations in
	// the product that name the role, and returns them. A pending invitation
	// is never touched. Call it inside a transaction.
	DeleteNonPendingForRole(
		ctx context.Context, productID, roleID string,
	) ([]organizationinvitation.Invitation, error)
}

type repositoryImpl struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewRepository(db *sql.DB, logger zerolog.Logger) Repository {
	return &repositoryImpl{
		db:     db,
		logger: logger.With().Str("component", "organization_invitation_repository").Logger(),
	}
}

func invitationScope(productID, organizationID string) postgres.BoolExpression {
	return table.OrganizationInvitations.ProductID.EQ(postgres.String(productID)).
		AND(table.OrganizationInvitations.OrganizationID.EQ(postgres.String(organizationID)))
}

func invitationByID(productID, organizationID, invitationID string) postgres.BoolExpression {
	return invitationScope(productID, organizationID).
		AND(table.OrganizationInvitations.ID.EQ(postgres.String(invitationID)))
}

func toDomain(now time.Time) func(model.OrganizationInvitations) organizationinvitation.Invitation {
	return func(entity model.OrganizationInvitations) organizationinvitation.Invitation {
		return organizationinvitation.Invitation{
			ID:             entity.ID,
			ProductID:      entity.ProductID,
			OrganizationID: entity.OrganizationID,
			Email:          entity.Email,
			RoleID:         entity.ProductRoleID,
			Status:         organizationinvitation.DeriveStatus(entity.AcceptedAt, entity.ExpiresAt, now),
			ExpiresAt:      entity.ExpiresAt,
			AcceptedAt:     entity.AcceptedAt,
			CreatedAt:      entity.CreatedAt,
			UpdatedAt:      entity.UpdatedAt,
		}
	}
}

func pendingAt(now time.Time) postgres.BoolExpression {
	return table.OrganizationInvitations.AcceptedAt.IS_NULL().
		AND(table.OrganizationInvitations.ExpiresAt.GT(postgres.TimestampzT(now)))
}

func expiredAt(now time.Time) postgres.BoolExpression {
	return table.OrganizationInvitations.AcceptedAt.IS_NULL().
		AND(table.OrganizationInvitations.ExpiresAt.LT_EQ(postgres.TimestampzT(now)))
}

func acceptedPredicate() postgres.BoolExpression {
	return table.OrganizationInvitations.AcceptedAt.IS_NOT_NULL()
}

func statusPredicate(status organizationinvitation.Status, now time.Time) postgres.BoolExpression {
	switch status {
	case organizationinvitation.StatusPending:
		return pendingAt(now)
	case organizationinvitation.StatusExpired:
		return expiredAt(now)
	case organizationinvitation.StatusAccepted:
		return acceptedPredicate()
	}
	return postgres.Bool(false)
}

func (r *repositoryImpl) LockOrganization(ctx context.Context, productID, organizationID string) (bool, error) {
	stmt := table.Organizations.SELECT(table.Organizations.AllColumns).
		FROM(table.Organizations).
		WHERE(
			table.Organizations.ID.EQ(postgres.String(organizationID)).
				AND(table.Organizations.ProductID.EQ(postgres.String(productID))),
		).
		FOR(postgres.NO_KEY_UPDATE())
	found, err := transactor.QueryOptional[model.Organizations](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return found.IsPresent(), nil
}

func (r *repositoryImpl) Create(
	ctx context.Context, invitation organizationinvitation.Invitation,
) (organizationinvitation.Invitation, error) {
	entity := model.OrganizationInvitations{
		ID:             invitation.ID,
		ProductID:      invitation.ProductID,
		OrganizationID: invitation.OrganizationID,
		Email:          invitation.Email,
		ProductRoleID:  invitation.RoleID,
		ExpiresAt:      invitation.ExpiresAt,
	}
	stmt := table.OrganizationInvitations.INSERT(
		table.OrganizationInvitations.ID,
		table.OrganizationInvitations.ProductID,
		table.OrganizationInvitations.OrganizationID,
		table.OrganizationInvitations.Email,
		table.OrganizationInvitations.ProductRoleID,
		table.OrganizationInvitations.ExpiresAt,
	).MODEL(entity).RETURNING(table.OrganizationInvitations.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, toDomain(time.Now())).Value()
}

func (r *repositoryImpl) FindByID(
	ctx context.Context, productID, organizationID, invitationID string,
) (functional.Option[organizationinvitation.Invitation], error) {
	stmt := table.OrganizationInvitations.SELECT(table.OrganizationInvitations.AllColumns).
		FROM(table.OrganizationInvitations).
		WHERE(invitationByID(productID, organizationID, invitationID)).
		LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, toDomain(time.Now()))
}

func (r *repositoryImpl) FindPendingByEmail(
	ctx context.Context, productID, organizationID, email string,
) (functional.Option[organizationinvitation.Invitation], error) {
	now := time.Now()
	stmt := table.OrganizationInvitations.SELECT(table.OrganizationInvitations.AllColumns).
		FROM(table.OrganizationInvitations).
		WHERE(
			invitationScope(productID, organizationID).
				AND(postgres.LOWER(table.OrganizationInvitations.Email).EQ(postgres.LOWER(postgres.String(email)))).
				AND(pendingAt(now)),
		).
		LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, toDomain(now))
}

func (r *repositoryImpl) EmailBelongsToMember(
	ctx context.Context, productID, organizationID, email string,
) (bool, error) {
	stmt := table.OrganizationMemberships.SELECT(table.OrganizationMemberships.AllColumns).
		FROM(
			table.OrganizationMemberships.INNER_JOIN(
				table.ProductUsers,
				table.OrganizationMemberships.ProductUserID.EQ(table.ProductUsers.ID),
			),
		).
		WHERE(
			table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)).
				AND(table.ProductUsers.ProductID.EQ(postgres.String(productID))).
				AND(postgres.LOWER(table.ProductUsers.Email).EQ(postgres.LOWER(postgres.String(email)))),
		).
		LIMIT(1)
	found, err := transactor.QueryOptional[model.OrganizationMemberships](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return found.IsPresent(), nil
}

func (r *repositoryImpl) Search(
	ctx context.Context,
	productID string,
	organizationID functional.Option[string],
	req search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField],
) (search.Result[organizationinvitation.Invitation], error) {
	now := time.Now()
	where := table.OrganizationInvitations.ProductID.EQ(postgres.String(productID))
	if organizationID.IsPresent() {
		where = invitationScope(productID, organizationID.Value())
	}

	if req.Filter != nil && len(req.Filter.Statuses) > 0 {
		predicates := functional.Slice(req.Filter.Statuses).Map(
			func(status organizationinvitation.Status) postgres.BoolExpression {
				return statusPredicate(status, now)
			},
		)
		anyStatus := predicates[0]
		for _, predicate := range predicates[1:] {
			anyStatus = anyStatus.OR(predicate)
		}
		where = where.AND(anyStatus)
	}

	if req.Filter != nil && len(req.Filter.Emails) > 0 {
		emails := functional.Slice(req.Filter.Emails).Map(func(email string) postgres.Expression {
			return postgres.LOWER(postgres.String(email))
		})
		where = where.AND(postgres.LOWER(table.OrganizationInvitations.Email).IN(emails...))
	}

	return transactor.Page(
		r.db,
		toDomain(now),
		table.OrganizationInvitations.AllColumns,
	).
		From(table.OrganizationInvitations).
		Where(where).
		OrderBy(transactor.SortColumns(
			req.Sort,
			map[organizationinvitation.SortField]postgres.Column{
				organizationinvitation.SortFieldCreatedAt: table.OrganizationInvitations.CreatedAt,
				organizationinvitation.SortFieldEmail:     table.OrganizationInvitations.Email,
				organizationinvitation.SortFieldExpiresAt: table.OrganizationInvitations.ExpiresAt,
			},
		)...).
		Run(ctx, req.Pagination).
		Value()
}

func (r *repositoryImpl) UpdateRoleAndExpiry(
	ctx context.Context, invitation organizationinvitation.Invitation,
) (organizationinvitation.Invitation, error) {
	stmt := table.OrganizationInvitations.UPDATE(
		table.OrganizationInvitations.ProductRoleID,
		table.OrganizationInvitations.ExpiresAt,
	).SET(
		postgres.String(invitation.RoleID),
		postgres.TimestampzT(invitation.ExpiresAt),
	).WHERE(
		invitationByID(invitation.ProductID, invitation.OrganizationID, invitation.ID),
	).RETURNING(table.OrganizationInvitations.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, toDomain(time.Now())).Value()
}

func (r *repositoryImpl) MarkAccepted(
	ctx context.Context, productID, organizationID, invitationID string, acceptedAt time.Time,
) (organizationinvitation.Invitation, error) {
	stmt := table.OrganizationInvitations.UPDATE(
		table.OrganizationInvitations.AcceptedAt,
	).SET(
		postgres.TimestampzT(acceptedAt),
	).WHERE(
		invitationByID(productID, organizationID, invitationID),
	).RETURNING(table.OrganizationInvitations.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, toDomain(time.Now())).Value()
}

func (r *repositoryImpl) Delete(ctx context.Context, productID, organizationID, invitationID string) error {
	stmt := table.OrganizationInvitations.DELETE().
		WHERE(invitationByID(productID, organizationID, invitationID))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) ExistsPendingForRole(ctx context.Context, productID, roleID string) (bool, error) {
	stmt := table.OrganizationInvitations.SELECT(table.OrganizationInvitations.AllColumns).
		FROM(table.OrganizationInvitations).
		WHERE(
			table.OrganizationInvitations.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(roleID))).
				AND(pendingAt(time.Now())),
		).
		LIMIT(1)
	found, err := transactor.QueryOptional[model.OrganizationInvitations](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return found.IsPresent(), nil
}

func (r *repositoryImpl) DeleteNonPendingForRole(
	ctx context.Context, productID, roleID string,
) ([]organizationinvitation.Invitation, error) {
	now := time.Now()
	stmt := table.OrganizationInvitations.DELETE().
		WHERE(
			table.OrganizationInvitations.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(roleID))).
				AND(postgres.NOT(pendingAt(now))),
		).
		RETURNING(table.OrganizationInvitations.AllColumns)
	return transactor.QueryMapSlice(ctx, r.db, stmt, toDomain(now)).Value()
}
