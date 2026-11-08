package clerk

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/domain/integration"
	"anchor/internal/domain/product/user"
	"anchor/internal/events"
	"anchor/internal/integration/provider"
)

func (p *Provider) executeUpsertUser(
	ctx context.Context,
	logger *slog.Logger,
	instance *integration.Instance,
	data any,
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

	var upserted user.ProductUser
	var created bool
	var unchanged bool
	upsertErr := p.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := p.productUserRepo.FindByExternalID(
			txCtx, instance.ProductID, upsertData.ExternalID,
		)
		if findErr != nil {
			return findErr
		}
		if existing.IsPresent() && !clerkUpsertWouldChange(existing.Value(), upsertData) {
			unchanged = true
			return nil
		}
		var err error
		upserted, created, err = p.productUserRepo.UpsertByExternalID(txCtx, productUser)
		if err != nil {
			return err
		}
		eventType := events.ProductUserCreated
		clerkType := events.ClerkUserCreated
		if !created {
			eventType = events.ProductUserUpdated
			clerkType = events.ClerkUserUpdated
		}
		if emitErr := p.events.Emit(txCtx, events.Event{
			Type:      eventType,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: upserted.ID},
		}); emitErr != nil {
			return emitErr
		}
		return p.events.Emit(txCtx, events.Event{
			Type:      clerkType,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: upserted.ID},
		})
	})
	if upsertErr != nil {
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityError,
			Message:               "Failed to upsert product user from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		})

		logger.ErrorContext(ctx, "failed to upsert product user",
			log.Err(upsertErr),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		return fmt.Errorf("failed to upsert product user: %w", upsertErr)
	}

	if unchanged {
		logger.DebugContext(ctx, "product user unchanged, skipping event",
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		return nil
	}

	if created {
		logger.InfoContext(ctx, "product user created via integration",
			slog.String("user_id", upserted.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)

		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Product user created from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				"user_id":          upserted.ID,
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		})
	} else {
		logger.DebugContext(ctx, "product user already exists, skipping audit log",
			slog.String("user_id", upserted.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
	}

	return nil
}

func clerkUpsertWouldChange(existing user.ProductUser, data UpsertUserData) bool {
	return existing.Email != data.Email ||
		existing.Name != data.Name ||
		existing.Status != user.ProductUserStatusActive
}
