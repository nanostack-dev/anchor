package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/db/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organization"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

// organizationToDomain is the stateless mapper that the generated base calls.
func organizationToDomain(row model.Organizations) organization.Organization {
	return (&mapper.OrganizationMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[OrganizationRepository, organizationRepositoryImpl](
		table.Organizations, organizationToDomain,
	)

	// POST /v1/products/{product_id}/organizations/search
	nanorepo.Search[model.Organizations]("Organization", table.Organizations,
		nanorepo.Map(organizationToDomain),
		nanorepo.Scope(table.Organizations.ProductID),
		nanorepo.Filter(table.Organizations.ID, nanorepo.In),
		nanorepo.Filter(table.Organizations.Name, nanorepo.In),
		nanorepo.FullText(table.Organizations.Name),
		nanorepo.Sort(table.Organizations.Name, table.Organizations.CreatedAt, table.Organizations.UpdatedAt),
		nanorepo.DefaultSort(table.Organizations.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(10),
	)
}

func organizationsUpdatableColumns() postgres.ColumnList {
	return table.Organizations.AllColumns.Except(
		table.Organizations.CreatedAt, table.Organizations.UpdatedAt,
	)
}

type OrganizationRepository interface {
	FindByProductIDAndID(
		ctx context.Context, productID string, id string,
	) (functional.Option[organization.Organization], error)
	Create(ctx context.Context, org organization.Organization) (
		organization.Organization, error,
	)
	Update(
		ctx context.Context, productID string, organization organization.Organization,
	) (organization.Organization, error)
	DeleteByProductIDAndID(ctx context.Context, productID string, id string) error
	SearchByProductID(ctx context.Context, q OrganizationSearch) (search.Result[organization.Organization], error)
}

type organizationRepositoryImpl struct {
	organizationsBase

	db                 *sql.DB
	organizationMapper *mapper.OrganizationMapper
	logger             *slog.Logger
}

func NewOrganizationRepository(
	db *sql.DB,
	organizationMapper *mapper.OrganizationMapper,
	logger *slog.Logger,
) OrganizationRepository {
	return &organizationRepositoryImpl{
		organizationsBase:  newOrganizationsBase(db),
		db:                 db,
		organizationMapper: organizationMapper,
		logger:             logger.With(slog.String("component", "organization_repository")),
	}
}

func (r *organizationRepositoryImpl) Create(
	ctx context.Context, org organization.Organization,
) (organization.Organization, error) {
	entity := r.organizationMapper.ToEntity(org)

	stmt := table.Organizations.INSERT(
		organizationsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Organizations.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.organizationMapper.ToDomain,
	).Value()
}

func (r *organizationRepositoryImpl) Update(
	ctx context.Context, productID string, currentOrg organization.Organization,
) (organization.Organization, error) {
	currentOrg.UpdatedAt = time.Now()
	entityToUpdate := r.organizationMapper.ToEntity(currentOrg)

	updateStmt := table.Organizations.UPDATE(
		organizationsUpdatableColumns().Except(
			table.Organizations.ID,
			table.Organizations.ProductID,
		),
	).MODEL(
		entityToUpdate,
	).WHERE(
		table.Organizations.ID.EQ(postgres.String(currentOrg.ID)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	).RETURNING(table.Organizations.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, updateStmt, r.organizationMapper.ToDomain,
	).Value()
}
