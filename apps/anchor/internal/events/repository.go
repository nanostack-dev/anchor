package events

import (
	"context"
	"database/sql"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"
)

type EndpointRepository interface {
	FindByProductIDInternal(ctx context.Context, productID string) (functional.Option[Endpoint], error)
	Upsert(ctx context.Context, endpoint Endpoint) error
	Delete(ctx context.Context, tenantID, productID string) error
	DeleteByProductIDInternal(ctx context.Context, productID string) error
}

type endpointRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewEndpointRepository(db *sql.DB, logger zerolog.Logger) EndpointRepository {
	return &endpointRepository{
		db:     db,
		logger: logger.With().Str("component", "event_endpoint_repository").Logger(),
	}
}

func (r *endpointRepository) FindByProductIDInternal(
	ctx context.Context, productID string,
) (functional.Option[Endpoint], error) {
	stmt := table.ProductEventEndpointConfigs.SELECT(
		table.ProductEventEndpointConfigs.AllColumns,
	).WHERE(
		table.ProductEventEndpointConfigs.ProductID.EQ(postgres.String(productID)),
	).LIMIT(1)

	row, err := transactor.QueryOptional[model.ProductEventEndpointConfigs](ctx, r.db, stmt)
	if err != nil {
		return functional.None[Endpoint](), err
	}
	return row.Map(endpointFromModel), nil
}

func (r *endpointRepository) Upsert(ctx context.Context, endpoint Endpoint) error {
	entity := model.ProductEventEndpointConfigs{
		ProductID:        endpoint.ProductID,
		PlatformTenantID: endpoint.PlatformTenantID,
		EndpointURL:      endpoint.URL,
		SigningSecret:    endpoint.SigningSecretEncrypted,
	}
	stmt := table.ProductEventEndpointConfigs.INSERT(
		table.ProductEventEndpointConfigs.ProductID,
		table.ProductEventEndpointConfigs.PlatformTenantID,
		table.ProductEventEndpointConfigs.EndpointURL,
		table.ProductEventEndpointConfigs.SigningSecret,
	).MODEL(entity).
		ON_CONFLICT(table.ProductEventEndpointConfigs.ProductID).
		DO_UPDATE(
			postgres.SET(
				table.ProductEventEndpointConfigs.EndpointURL.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.EndpointURL,
				),
				table.ProductEventEndpointConfigs.SigningSecret.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.SigningSecret,
				),
				table.ProductEventEndpointConfigs.PlatformTenantID.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.PlatformTenantID,
				),
			),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *endpointRepository) Delete(ctx context.Context, tenantID, productID string) error {
	stmt := table.ProductEventEndpointConfigs.DELETE().WHERE(
		table.ProductEventEndpointConfigs.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductEventEndpointConfigs.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *endpointRepository) DeleteByProductIDInternal(ctx context.Context, productID string) error {
	stmt := table.ProductEventEndpointConfigs.DELETE().WHERE(
		table.ProductEventEndpointConfigs.ProductID.EQ(postgres.String(productID)),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func endpointFromModel(row model.ProductEventEndpointConfigs) Endpoint {
	return Endpoint{
		ProductID:              row.ProductID,
		PlatformTenantID:       row.PlatformTenantID,
		URL:                    row.EndpointURL,
		SigningSecretEncrypted: row.SigningSecret,
	}
}
