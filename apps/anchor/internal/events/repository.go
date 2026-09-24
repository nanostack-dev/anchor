package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
	// RecordDeliveryResultInternal updates delivery state from the background worker only.
	RecordDeliveryResultInternal(ctx context.Context, productID, endpointURL string, succeeded bool) error
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
	if row.IsAbsent() {
		return functional.None[Endpoint](), nil
	}
	endpoint, err := endpointFromModel(row.Value())
	if err != nil {
		return functional.None[Endpoint](), err
	}
	return functional.Some(endpoint), nil
}

func (r *endpointRepository) Upsert(ctx context.Context, endpoint Endpoint) error {
	if endpoint.Events == nil {
		endpoint.Events = []string{}
	}
	eventsJSON, err := json.Marshal(endpoint.Events)
	if err != nil {
		return fmt.Errorf("encode event subscriptions: %w", err)
	}
	entity := model.ProductEventEndpointConfigs{
		ProductID:              endpoint.ProductID,
		PlatformTenantID:       endpoint.PlatformTenantID,
		EndpointURL:            endpoint.URL,
		SigningSecret:          endpoint.SigningSecretEncrypted,
		EventsJSON:             string(eventsJSON),
		DeliveryStatus:         endpoint.DeliveryStatus,
		ConsecutiveFailedCalls: endpoint.ConsecutiveFailedCalls,
	}
	stmt := table.ProductEventEndpointConfigs.INSERT(
		table.ProductEventEndpointConfigs.ProductID,
		table.ProductEventEndpointConfigs.PlatformTenantID,
		table.ProductEventEndpointConfigs.EndpointURL,
		table.ProductEventEndpointConfigs.SigningSecret,
		table.ProductEventEndpointConfigs.EventsJSON,
		table.ProductEventEndpointConfigs.DeliveryStatus,
		table.ProductEventEndpointConfigs.ConsecutiveFailedCalls,
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
				table.ProductEventEndpointConfigs.EventsJSON.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.EventsJSON,
				),
				table.ProductEventEndpointConfigs.DeliveryStatus.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.DeliveryStatus,
				),
				table.ProductEventEndpointConfigs.ConsecutiveFailedCalls.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.ConsecutiveFailedCalls,
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

func (r *endpointRepository) RecordDeliveryResultInternal(
	ctx context.Context, productID, endpointURL string, succeeded bool,
) error {
	query := `UPDATE product_event_endpoint_configs
		SET delivery_status = 'failed', consecutive_failed_calls = consecutive_failed_calls + 1
		WHERE product_id = $1 AND endpoint_url = $2`
	if succeeded {
		query = `UPDATE product_event_endpoint_configs
			SET delivery_status = 'succeeded', consecutive_failed_calls = 0
			WHERE product_id = $1 AND endpoint_url = $2`
	}
	_, err := r.db.ExecContext(ctx, query, productID, endpointURL)
	return err
}

func endpointFromModel(row model.ProductEventEndpointConfigs) (Endpoint, error) {
	var eventsList []string
	if err := json.Unmarshal([]byte(row.EventsJSON), &eventsList); err != nil {
		return Endpoint{}, fmt.Errorf("decode event subscriptions for product %s: %w", row.ProductID, err)
	}
	if eventsList == nil {
		return Endpoint{}, fmt.Errorf("event subscriptions for product %s must be an array", row.ProductID)
	}
	return Endpoint{
		ProductID:              row.ProductID,
		PlatformTenantID:       row.PlatformTenantID,
		URL:                    row.EndpointURL,
		SigningSecretEncrypted: row.SigningSecret,
		Events:                 eventsList,
		DeliveryStatus:         row.DeliveryStatus,
		ConsecutiveFailedCalls: row.ConsecutiveFailedCalls,
	}, nil
}
