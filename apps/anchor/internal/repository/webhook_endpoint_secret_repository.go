package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/webhook"
	"anchor/internal/mapper"
)

var _ WebhookEndpointSecretRepository = (*webhookEndpointSecretRepositoryImpl)(nil)

func webhookEndpointSecretsUpdatableColumns() postgres.ColumnList {
	return table.WebhookEndpointSecrets.AllColumns.Except(
		table.WebhookEndpointSecrets.CreatedAt, table.WebhookEndpointSecrets.UpdatedAt,
	)
}

type WebhookEndpointSecretRepository interface {
	Create(ctx context.Context, secret webhook.Secret) (webhook.Secret, error)

	// ListByEndpointInternal loads every secret of an endpoint without a
	// product filter.
	//
	// Product-scope bypass: the delivery worker signs for endpoints across all
	// products and reaches secrets by endpoint id only. Callers in tenant-facing
	// code must first resolve the endpoint through the product-scoped endpoint
	// repository, which is what the endpoint service does.
	ListByEndpointInternal(ctx context.Context, endpointID string) ([]webhook.Secret, error)

	// ExpireActiveInternal marks every ACTIVE secret of an endpoint EXPIRING
	// with the supplied expiry, as the first half of a rotation.
	//
	// Product-scope bypass: reached by endpoint id after the caller has already
	// resolved the endpoint under its product scope.
	ExpireActiveInternal(
		ctx context.Context, endpointID string, expiresAt time.Time,
	) error
}

type webhookEndpointSecretRepositoryImpl struct {
	db           *sql.DB
	secretMapper *mapper.WebhookEndpointSecretMapper
	logger       zerolog.Logger
}

func NewWebhookEndpointSecretRepository(
	db *sql.DB, secretMapper *mapper.WebhookEndpointSecretMapper, logger zerolog.Logger,
) WebhookEndpointSecretRepository {
	return &webhookEndpointSecretRepositoryImpl{
		db:           db,
		secretMapper: secretMapper,
		logger: logger.With().
			Str("component", "webhook_endpoint_secret_repository").Logger(),
	}
}

func (r *webhookEndpointSecretRepositoryImpl) Create(
	ctx context.Context, secret webhook.Secret,
) (webhook.Secret, error) {
	entity := r.secretMapper.ToEntity(secret)

	stmt := table.WebhookEndpointSecrets.INSERT(
		webhookEndpointSecretsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.WebhookEndpointSecrets.AllColumns)

	created, err := transactor.Query[model.WebhookEndpointSecrets](ctx, r.db, stmt)
	if err != nil {
		return webhook.Secret{}, err
	}

	return r.secretMapper.ToDomain(created), nil
}

func (r *webhookEndpointSecretRepositoryImpl) ListByEndpointInternal(
	ctx context.Context, endpointID string,
) ([]webhook.Secret, error) {
	stmt := table.WebhookEndpointSecrets.SELECT(
		table.WebhookEndpointSecrets.AllColumns,
	).WHERE(
		table.WebhookEndpointSecrets.EndpointID.EQ(postgres.String(endpointID)),
	).ORDER_BY(table.WebhookEndpointSecrets.CreatedAt.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.secretMapper.ToDomain)
}

func (r *webhookEndpointSecretRepositoryImpl) ExpireActiveInternal(
	ctx context.Context, endpointID string, expiresAt time.Time,
) error {
	entity := model.WebhookEndpointSecrets{
		Status:    string(webhook.SecretStatusExpiring),
		ExpiresAt: &expiresAt,
		UpdatedAt: time.Now(),
	}

	stmt := table.WebhookEndpointSecrets.UPDATE(
		postgres.ColumnList{
			table.WebhookEndpointSecrets.Status,
			table.WebhookEndpointSecrets.ExpiresAt,
			table.WebhookEndpointSecrets.UpdatedAt,
		},
	).MODEL(entity).WHERE(
		table.WebhookEndpointSecrets.EndpointID.EQ(postgres.String(endpointID)).AND(
			table.WebhookEndpointSecrets.Status.EQ(
				postgres.String(string(webhook.SecretStatusActive)),
			),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}
