package repository

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
)

// The base returns model rows: the repository maps them with the change mapper.
func _() {
	nanorepo.Implement[OrganizationLicenseChangeRepository, organizationLicenseChangeRepositoryImpl](
		table.OrganizationLicenseChanges, nanorepo.NoMapper[model.OrganizationLicenseChanges],
	)

	// POST /v1/products/{product_id}/organizations/{organization_id}/license/history/search,
	// and the deprecated GET. changed_after is inclusive and changed_before
	// exclusive. The identifier breaks the tie because the entries of one
	// adjustment share a changed_at.
	nanorepo.Search[model.OrganizationLicenseChanges]("OrganizationLicenseChange", table.OrganizationLicenseChanges,
		nanorepo.Map((*mapper.OrganizationLicenseChangeMapper).ToDomain),
		nanorepo.Scope(table.OrganizationLicenseChanges.PlatformTenantID),
		nanorepo.Scope(table.OrganizationLicenseChanges.ProductID),
		nanorepo.Scope(table.OrganizationLicenseChanges.OrganizationID),
		nanorepo.FilterAs[license.ChangeType](table.OrganizationLicenseChanges.ChangeType, nanorepo.In.API("types")),
		nanorepo.Filter(table.OrganizationLicenseChanges.ChangedAt,
			nanorepo.Range("changed_after", "changed_before").HalfOpen()),
		nanorepo.Sort(table.OrganizationLicenseChanges.ChangedAt),
		nanorepo.DefaultSort(table.OrganizationLicenseChanges.ChangedAt, nanorepo.Desc),
		nanorepo.DefaultLimit(10),
	)
}

type organizationLicenseChangeRepositoryImpl struct {
	organizationLicenseChangesBase

	db     *sql.DB
	mapper *mapper.OrganizationLicenseChangeMapper
	logger *slog.Logger
}

func NewOrganizationLicenseChangeRepository(
	db *sql.DB, m *mapper.OrganizationLicenseChangeMapper, logger *slog.Logger,
) OrganizationLicenseChangeRepository {
	return &organizationLicenseChangeRepositoryImpl{
		organizationLicenseChangesBase: newOrganizationLicenseChangesBase(db),
		db:                             db,
		mapper:                         m,
		logger:                         logger.With(slog.String("component", "organization_license_change_repository")),
	}
}

func (r *organizationLicenseChangeRepositoryImpl) Append(
	ctx context.Context, changes []license.OrganizationLicenseChange,
) error {
	if len(changes) == 0 {
		return nil
	}

	entities := functional.Slice(changes).Map(r.mapper.ToEntity)

	stmt := table.OrganizationLicenseChanges.
		INSERT(table.OrganizationLicenseChanges.AllColumns).
		MODELS(entities)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
