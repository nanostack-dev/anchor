package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
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

	entities := functional.Slice(changes).Map(r.mapper.ToEntity)

	stmt := table.OrganizationLicenseChanges.
		INSERT(table.OrganizationLicenseChanges.AllColumns).
		MODELS(entities)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *organizationLicenseChangeRepositoryImpl) Search(
	ctx context.Context, in license.SearchLicenseChangesInput,
) (search.Result[license.OrganizationLicenseChange], error) {
	changes := table.OrganizationLicenseChanges
	where := organizationLicenseChangeScope(in.TenantID, in.ProductID, in.OrganizationID)

	if filter := in.Request.Filter; filter != nil {
		if len(filter.Types) > 0 {
			types := functional.Slice(filter.Types).Map(func(changeType license.ChangeType) string {
				return string(changeType)
			})
			where = where.AND(changes.ChangeType.IN(jetx.ToStringExpressions(types)...))
		}
		if filter.ChangedAfter != nil {
			where = where.AND(changes.ChangedAt.GT_EQ(postgres.TimestampzT(*filter.ChangedAfter)))
		}
		if filter.ChangedBefore != nil {
			where = where.AND(changes.ChangedAt.LT(postgres.TimestampzT(*filter.ChangedBefore)))
		}
	}

	// The identifier breaks the tie because the entries of one adjustment share
	// a single changed_at, and a page boundary landing inside one of them must
	// not reorder or repeat a row.
	order, err := transactor.StrictKeys(
		in.Request.Sort,
		map[license.SortFieldChange]transactor.Key[model.OrganizationLicenseChanges]{
			license.SortFieldChangeChangedAt: transactor.TimeKey(
				changes.ChangedAt, func(row model.OrganizationLicenseChanges) time.Time { return row.ChangedAt },
			),
		},
		transactor.StringKey(changes.ID, func(row model.OrganizationLicenseChanges) string { return row.ID }),
	)
	if err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}

	return transactor.Page(r.db, r.mapper.ToDomain, changes.AllColumns).
		From(changes).
		Where(where).
		Keyset(order).
		Run(ctx, in.Request.Pagination).
		Value()
}
