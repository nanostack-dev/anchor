package repository

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ OrganizationLicenseChangeRepository = (*organizationLicenseChangeRepositoryImpl)(nil)

type organizationLicenseChangeRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.OrganizationLicenseChangeMapper
	logger zerolog.Logger
}

func NewOrganizationLicenseChangeRepository(
	db *sql.DB, m *mapper.OrganizationLicenseChangeMapper, logger zerolog.Logger,
) OrganizationLicenseChangeRepository {
	return &organizationLicenseChangeRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "organization_license_change_repository").Logger(),
	}
}

// organizationLicenseChangeScope is the tenant, product and Organization
// predicate every read in this file carries. Written once so a new query
// cannot be added with half the scope.
func organizationLicenseChangeScope(
	tenantID, productID, organizationID string,
) postgres.BoolExpression {
	return table.OrganizationLicenseChanges.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.OrganizationLicenseChanges.ProductID.EQ(postgres.String(productID))).
		AND(table.OrganizationLicenseChanges.OrganizationID.EQ(postgres.String(organizationID)))
}

func (r *organizationLicenseChangeRepositoryImpl) Append(
	ctx context.Context, changes []license.OrganizationLicenseChange,
) error {
	if len(changes) == 0 {
		return nil
	}

	entities := make([]model.OrganizationLicenseChanges, 0, len(changes))
	for _, change := range changes {
		entities = append(entities, r.mapper.ToEntity(change))
	}

	stmt := table.OrganizationLicenseChanges.
		INSERT(table.OrganizationLicenseChanges.AllColumns).
		MODELS(entities)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *organizationLicenseChangeRepositoryImpl) ListByOrganization(
	ctx context.Context, in license.ListLicenseChangesInput,
) (search.Result[license.OrganizationLicenseChange], error) {
	changes := table.OrganizationLicenseChanges
	return transactor.Page(r.db, r.mapper.ToDomain, changes.AllColumns).
		From(changes).
		Where(organizationLicenseChangeScope(in.TenantID, in.ProductID, in.OrganizationID)).
		// The identifier breaks the tie because the entries of one adjustment
		// share a single changed_at, and a page boundary landing inside one of
		// them must not reorder or repeat a row.
		OrderBy(changes.ChangedAt.DESC(), changes.ID.DESC()).
		Run(ctx, in.Pagination).
		Value()
}
