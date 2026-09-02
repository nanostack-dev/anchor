package clerk

import (
	"context"
	"fmt"

	"anchor/internal/domain/integration"
	"anchor/internal/events"
	"anchor/internal/integration/provider"

	"github.com/rs/zerolog"
)

const clerkExternalIDKey = "external_id"

// executeDeleteUser handles CommandDeleteUser commands by deleting a product
// user via the product user repository.
func (p *Provider) executeDeleteUser(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	data any,
) error {
	deleteData, ok := data.(DeleteUserData)
	if !ok {
		return provider.ErrInvalidCommandData(CommandDeleteUser, "DeleteUserData")
	}

	delErr := p.transactor.InTx(ctx, func(txCtx context.Context) error {
		found, findErr := p.productUserRepo.FindByExternalID(
			txCtx, instance.ProductID, deleteData.ExternalID,
		)
		if findErr != nil {
			return findErr
		}
		if delErr := p.productUserRepo.DeleteByExternalID(
			txCtx, instance.ProductID, deleteData.ExternalID,
		); delErr != nil {
			return delErr
		}
		if found.IsAbsent() {
			return nil
		}
		return p.events.Emit(txCtx, events.Event{
			Type:      events.ProductUserDeleted,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: found.Value().ID},
		})
	})
	if delErr != nil {
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionDeleteUser,
			Severity:              integration.AuditSeverityError,
			Message:               "Failed to delete product user from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey: deleteData.ExternalID,
			}),
		})

		logger.Error().Err(delErr).
			Str(clerkExternalIDKey, deleteData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("failed to delete product user")
		return fmt.Errorf("failed to delete product user: %w", delErr)
	}

	logger.Info().
		Str(clerkExternalIDKey, deleteData.ExternalID).
		Str("product_id", instance.ProductID).
		Msg("product user deleted via integration")

	provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
		IntegrationInstanceID: instance.ID,
		Action:                integration.AuditActionDeleteUser,
		Severity:              integration.AuditSeveritySuccess,
		Message:               "Product user deleted from integration event",
		MetadataJSON: provider.MustMarshalJSON(map[string]any{
			clerkExternalIDKey: deleteData.ExternalID,
		}),
	})

	return nil
}
