package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organizationinvitation"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
)

// The base returns model rows: the repository maps them with toDomain.
func _() {
	nanorepo.Implement[Repository, repositoryImpl](
		table.OrganizationInvitations, nanorepo.NoMapper[model.OrganizationInvitations],
	)

	// POST /v1/products/{product_id}/organizations/{organization_id}/invitations/search.
	// full_text_search is accepted and ignored: an invitation has no text column
	// that the contract names. The status of an invitation is never stored: the
	// Status filter derives it from accepted_at and expires_at at the moment of
	// the read.
	nanorepo.Search[model.OrganizationInvitations]("OrganizationInvitation", table.OrganizationInvitations,
		nanorepo.Map(invitationToDomain),
		nanorepo.Scope(table.OrganizationInvitations.ProductID),
		nanorepo.Scope(table.OrganizationInvitations.OrganizationID),
		nanorepo.FilterExpr("Status", invitationStatus, nanorepo.In),
		nanorepo.Filter(table.OrganizationInvitations.Email, nanorepo.InFold),
		nanorepo.Sort(
			table.OrganizationInvitations.CreatedAt, table.OrganizationInvitations.Email,
			table.OrganizationInvitations.ExpiresAt,
		),
		nanorepo.DefaultSort(table.OrganizationInvitations.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(10),
	)

	// POST /v1/products/{product_id}/invitations/search: the invitations of every
	// organization of the product.
	nanorepo.Search[model.OrganizationInvitations]("ProductInvitation", table.OrganizationInvitations,
		nanorepo.Map(invitationToDomain),
		nanorepo.Scope(table.OrganizationInvitations.ProductID),
		nanorepo.Unscoped(table.OrganizationInvitations.OrganizationID,
			"the product-wide search reads the invitations of every organization of the product"),
		nanorepo.FilterExpr("Status", invitationStatus, nanorepo.In),
		nanorepo.Filter(table.OrganizationInvitations.Email, nanorepo.InFold),
		nanorepo.Sort(
			table.OrganizationInvitations.CreatedAt, table.OrganizationInvitations.Email,
			table.OrganizationInvitations.ExpiresAt,
		),
		nanorepo.DefaultSort(table.OrganizationInvitations.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(10),
	)
}

// The values of the Status filter: an invitation is accepted once it has an
// acceptance, expired once its expiry has passed without one, and pending
// otherwise. It is organizationinvitation.DeriveStatus, evaluated by the
// database at the moment of the read.
const (
	statusPending  = string(organizationinvitation.StatusPending)
	statusAccepted = string(organizationinvitation.StatusAccepted)
	statusExpired  = string(organizationinvitation.StatusExpired)
)

func invitationStatus() postgres.StringExpression {
	return postgres.StringExp(
		postgres.CASE().
			WHEN(table.OrganizationInvitations.AcceptedAt.IS_NOT_NULL()).THEN(postgres.String(statusAccepted)).
			WHEN(table.OrganizationInvitations.ExpiresAt.LT_EQ(postgres.NOW())).THEN(postgres.String(statusExpired)).
			ELSE(postgres.String(statusPending)),
	)
}

// invitationToDomain is the mapper of a search row: it derives the status at the
// moment of the read.
func invitationToDomain(entity model.OrganizationInvitations) organizationinvitation.Invitation {
	return toDomain(time.Now())(entity)
}

// ErrRoleGone is returned by Create and UpdateRoleAndExpiry when the role was
// deleted after the service found it: a role delete holds the role's row FOR
// UPDATE, so the write waits and then trips the foreign key.
var ErrRoleGone = errors.New("invitation role no longer exists")

const roleForeignKey = "organization_invitations_product_role_id_fkey"

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
	// SearchInOrganization lists the invitations of one organization.
	SearchInOrganization(
		ctx context.Context, q OrganizationInvitationSearch,
	) (search.Result[organizationinvitation.Invitation], error)
	// SearchInProduct lists the invitations of every organization of the product.
	SearchInProduct(
		ctx context.Context, q ProductInvitationSearch,
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
	organizationInvitationsBase

	db     *sql.DB
	logger *slog.Logger
}

func NewRepository(db *sql.DB, logger *slog.Logger) Repository {
	return &repositoryImpl{
		organizationInvitationsBase: newOrganizationInvitationsBase(db),
		db:                          db,
		logger:                      logger.With(slog.String("component", "organization_invitation_repository")),
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
	return transactor.QueryMap(ctx, r.db, stmt, toDomain(time.Now())).
		OnForeignKey(ErrRoleGone, roleForeignKey).
		Value()
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

func (r *repositoryImpl) UpdateRoleAndExpiry(
	ctx context.Context, invitation organizationinvitation.Invitation,
) (organizationinvitation.Invitation, error) {
	stmt := table.OrganizationInvitations.UPDATE(
		table.OrganizationInvitations.ProductRoleID,
		table.OrganizationInvitations.ExpiresAt,
	).MODEL(model.OrganizationInvitations{
		ProductRoleID: invitation.RoleID,
		ExpiresAt:     invitation.ExpiresAt,
	}).WHERE(
		invitationByID(invitation.ProductID, invitation.OrganizationID, invitation.ID),
	).RETURNING(table.OrganizationInvitations.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, toDomain(time.Now())).
		OnForeignKey(ErrRoleGone, roleForeignKey).
		Value()
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
