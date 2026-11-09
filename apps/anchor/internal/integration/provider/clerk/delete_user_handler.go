package clerk

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/domain/integration"
	"anchor/internal/events"
	"anchor/internal/integration/provider"
)

const clerkExternalIDKey = "external_id"

// executeDeleteUser handles CommandDeleteUser commands by deleting a product
// user via the product user repository.
func (p *Provider) executeDeleteUser(
	ctx context.Context,
	logger *slog.Logger,
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
		if user, present := found.Get(); present {
			// Its role assignments on resources go with its memberships
			// through the foreign keys, each with its event.
			if releaseErr := p.roleAssignments.ReleaseUser(txCtx, instance.ProductID, user.ID); releaseErr != nil {
				return releaseErr
			}
		}
		if delErr := p.productUserRepo.DeleteByProductIDAndExternalID(
			txCtx, instance.ProductID, deleteData.ExternalID,
		); delErr != nil {
			return delErr
		}
		if found.IsAbsent() {
			return nil
		}
		if emitErr := p.events.Emit(txCtx, events.Event{
			Type:      events.ProductUserDeleted,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: found.Value().ID},
		}); emitErr != nil {
			return emitErr
		}
		return p.events.Emit(txCtx, events.Event{
			Type:      events.ClerkUserDeleted,
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

		logger.ErrorContext(ctx, "failed to delete product user",
			log.Err(delErr),
			slog.String(clerkExternalIDKey, deleteData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		return fmt.Errorf("failed to delete product user: %w", delErr)
	}

	logger.InfoContext(ctx, "product user deleted via integration",
		slog.String(clerkExternalIDKey, deleteData.ExternalID),
		slog.String("product_id", instance.ProductID),
	)

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
