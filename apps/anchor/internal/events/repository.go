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
	DeliveryStatus(ctx context.Context, tenantID, productID string) (DeliveryStatus, error)
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
		ProductID:        endpoint.ProductID,
		PlatformTenantID: endpoint.PlatformTenantID,
		EndpointURL:      endpoint.URL,
		SigningSecret:    endpoint.SigningSecretEncrypted,
		EventsJSON:       string(eventsJSON),
	}
	stmt := table.ProductEventEndpointConfigs.INSERT(
		table.ProductEventEndpointConfigs.ProductID,
		table.ProductEventEndpointConfigs.PlatformTenantID,
		table.ProductEventEndpointConfigs.EndpointURL,
		table.ProductEventEndpointConfigs.SigningSecret,
		table.ProductEventEndpointConfigs.EventsJSON,
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

func (r *endpointRepository) DeliveryStatus(
	ctx context.Context, tenantID, productID string,
) (DeliveryStatus, error) {
	// ponytail: this reads pgqueue's existing rows. Add a dedicated status table if
	// queue volume makes the product page's status query slow.
	const productJobs = `
		FROM pgqueue_jobs AS jobs
		JOIN products ON products.id = $2 AND products.platform_tenant_id = $1
		WHERE jobs.queue_name = 'product-events'
		  AND convert_from(jobs.payload, 'UTF8')::jsonb ->> 'product_id' = $2`
	var status DeliveryStatus
	err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE jobs.status = 'failed'),
		       count(*) FILTER (WHERE (jobs.status = 'pending' AND jobs.attempts > 0)
		                          OR (jobs.status = 'processing' AND jobs.attempts > 1))
		`+productJobs, tenantID, productID).Scan(&status.FailedCount, &status.RetryingCount)
	if err != nil {
		return DeliveryStatus{}, fmt.Errorf("read event delivery status: %w", err)
	}

	var failure DeliveryFailure
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(convert_from(jobs.payload, 'UTF8')::jsonb ->> 'type',
		                convert_from(jobs.payload, 'UTF8')::jsonb -> 'body' ->> 'type',
		                'unknown'),
		       jobs.attempts, COALESCE(jobs.last_error, ''), jobs.updated_at
		`+productJobs+`
		  AND jobs.status = 'failed'
		ORDER BY jobs.updated_at DESC, jobs.id DESC LIMIT 1`, tenantID, productID).
		Scan(&failure.EventType, &failure.Attempts, &failure.Error, &failure.FailedAt)
	if err != nil && err != sql.ErrNoRows {
		return DeliveryStatus{}, fmt.Errorf("read last event delivery failure: %w", err)
	}
	if err == nil {
		status.LastFailure = &failure
	}
	return status, nil
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
	}, nil
}
