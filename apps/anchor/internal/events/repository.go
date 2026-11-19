package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// endpointRepository keeps the model rows: endpointFromModel returns an error,
// and a nanorepo mapper cannot. DeleteByTenantAndProduct is
// derived; the other methods are hand-written. DeleteGoneEndpointInternal
// deletes across tenants, and such a delete never derives.
func _() {
	nanorepo.Implement[EndpointRepository, endpointRepository](
		table.ProductEventEndpointConfigs, nanorepo.NoMapper[model.ProductEventEndpointConfigs],
	)
}

type EndpointRepository interface {
	FindByProductIDInternal(ctx context.Context, productID string) (functional.Option[Endpoint], error)
	Upsert(ctx context.Context, endpoint Endpoint) error
	DeleteByTenantAndProduct(ctx context.Context, platformTenantID, productID string) error
	// DeleteGoneEndpointInternal removes the product's endpoint from the
	// background worker only, when it still has endpointURL, at least
	// minFailedCalls consecutive failed calls, and a streak that began at least
	// minFailingFor ago. It reports whether it removed it.
	DeleteGoneEndpointInternal(
		ctx context.Context, productID, endpointURL string, minFailedCalls int32, minFailingFor time.Duration,
	) (bool, error)
	// RecordDeliveryResultInternal updates delivery state from the background worker only.
	RecordDeliveryResultInternal(ctx context.Context, productID, endpointURL string, succeeded bool) error
}

type endpointRepository struct {
	productEventEndpointConfigsBase

	db     *sql.DB
	logger *slog.Logger
}

func NewEndpointRepository(db *sql.DB, logger *slog.Logger) EndpointRepository {
	return &endpointRepository{
		productEventEndpointConfigsBase: newProductEventEndpointConfigsBase(db),
		db:                              db,
		logger:                          logger.With(slog.String("component", "event_endpoint_repository")),
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
		FailingSince:           optionalTime(endpoint.FailingSince),
		LastFailedAt:           optionalTime(endpoint.LastFailedAt),
	}
	stmt := table.ProductEventEndpointConfigs.INSERT(
		table.ProductEventEndpointConfigs.ProductID,
		table.ProductEventEndpointConfigs.PlatformTenantID,
		table.ProductEventEndpointConfigs.EndpointURL,
		table.ProductEventEndpointConfigs.SigningSecret,
		table.ProductEventEndpointConfigs.EventsJSON,
		table.ProductEventEndpointConfigs.DeliveryStatus,
		table.ProductEventEndpointConfigs.ConsecutiveFailedCalls,
		table.ProductEventEndpointConfigs.FailingSince,
		table.ProductEventEndpointConfigs.LastFailedAt,
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
				table.ProductEventEndpointConfigs.FailingSince.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.FailingSince,
				),
				table.ProductEventEndpointConfigs.LastFailedAt.SET(
					table.ProductEventEndpointConfigs.EXCLUDED.LastFailedAt,
				),
			),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

// DeleteGoneEndpointInternal checks the URL, the count and the age of the
// streak in the delete itself, against the database clock: a save of a new
// URL, or a success that resets the streak, between the worker's call and this
// delete keeps the endpoint. A streak with no start (NULL) never matches.
func (r *endpointRepository) DeleteGoneEndpointInternal(
	ctx context.Context, productID, endpointURL string, minFailedCalls int32, minFailingFor time.Duration,
) (bool, error) {
	stmt := table.ProductEventEndpointConfigs.DELETE().WHERE(
		table.ProductEventEndpointConfigs.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductEventEndpointConfigs.EndpointURL.EQ(postgres.String(endpointURL))).
			AND(table.ProductEventEndpointConfigs.ConsecutiveFailedCalls.GT_EQ(postgres.Int32(minFailedCalls))).
			AND(table.ProductEventEndpointConfigs.FailingSince.LT_EQ(
				postgres.NOW().SUB(postgres.INTERVALd(minFailingFor)),
			)),
	)
	deleted, err := transactor.ExecAffected(ctx, r.db, stmt).Value()
	if err != nil {
		return false, err
	}
	return deleted > 0, nil
}

func (r *endpointRepository) RecordDeliveryResultInternal(
	ctx context.Context, productID, endpointURL string, succeeded bool,
) error {
	query := `UPDATE product_event_endpoint_configs
		SET delivery_status = 'failed', consecutive_failed_calls = consecutive_failed_calls + 1,
			failing_since = COALESCE(failing_since, NOW()), last_failed_at = NOW()
		WHERE product_id = $1 AND endpoint_url = $2`
	if succeeded {
		query = `UPDATE product_event_endpoint_configs
			SET delivery_status = 'succeeded', consecutive_failed_calls = 0,
				failing_since = NULL, last_failed_at = NULL
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
		FailingSince:           timeOrZero(row.FailingSince),
		LastFailedAt:           timeOrZero(row.LastFailedAt),
	}, nil
}

// optionalTime stores the zero time as NULL.
func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// timeOrZero reads NULL as the zero time.
func timeOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
