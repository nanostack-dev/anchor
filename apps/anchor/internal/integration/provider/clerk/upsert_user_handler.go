package clerk

import (
	"context"
	"fmt"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/domain/product/user"
	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/rs/zerolog"
)

// executeUpsertUser handles CommandUpsertUser commands by upserting a product
// user via the product user repository. It only logs and emits an audit event
// when the user is newly created; updates to existing users are intentionally
// silent to avoid log/audit noise on every Clerk sync event.
func (p *Provider) executeUpsertUser(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	data any,
	txOpts *jetx.DBOptions,
) error {
	upsertData, ok := data.(UpsertUserData)
	if !ok {
		return provider.ErrInvalidCommandData(CommandUpsertUser, "UpsertUserData")
	}

	productUser := user.ProductUser{
		ProductID:  instance.ProductID,
		Email:      upsertData.Email,
		Name:       upsertData.Name,
		ExternalID: &upsertData.ExternalID,
		Status:     user.ProductUserStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	productUser.GenerateID()

	upserted, created, upsertErr := p.productUserRepo.UpsertByExternalID(ctx, productUser, txOpts)
	if upsertErr != nil {
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityError,
			Message:               "Failed to upsert product user from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		}, txOpts)

		logger.Error().Err(upsertErr).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("failed to upsert product user")
		return fmt.Errorf("failed to upsert product user: %w", upsertErr)
	}

	if created {
		logger.Info().
			Str("user_id", upserted.ID).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("product user created via integration")

		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Product user created from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				"user_id":          upserted.ID,
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		}, txOpts)
	} else {
		logger.Debug().
			Str("user_id", upserted.ID).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("product user already exists, skipping audit log")
	}

	return nil
}
