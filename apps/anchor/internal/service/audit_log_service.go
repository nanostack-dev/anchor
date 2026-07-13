package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/audit"
	"anchor/internal/repository"
	"anchor/internal/security"

	"github.com/rs/zerolog"
)

// AuditLogService records and queries the general audit log.
// Design doc: docs/audit-logs.md.
type AuditLogService interface {
	// Record writes an audit log entry. It resolves the actor and tenant from
	// the request context when not already set on the entry, and never returns
	// an error: a failed audit write is logged but must not fail the mutation
	// that triggered it. Call it after the mutation succeeds, passing the
	// transaction context when inside one so the entry commits atomically.
	Record(ctx context.Context, entry audit.Log)

	Search(ctx context.Context, input audit.SearchInput) (search.Result[audit.Log], error)
}

type auditLogService struct {
	auditLogRepo     repository.AuditLogRepository
	platformUserRepo repository.PlatformTenantUserRepository
	logger           zerolog.Logger
}

func NewAuditLogService(
	auditLogRepo repository.AuditLogRepository,
	platformUserRepo repository.PlatformTenantUserRepository,
	logger zerolog.Logger,
) AuditLogService {
	return &auditLogService{
		auditLogRepo:     auditLogRepo,
		platformUserRepo: platformUserRepo,
		logger:           logger.With().Str("component", "audit_log_service").Logger(),
	}
}

func (s *auditLogService) Record(ctx context.Context, entry audit.Log) {
	logger := s.logger.With().Str("operation", "Record").Logger()

	if entry.ID == "" {
		entry.GenerateID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.Outcome == "" {
		entry.Outcome = audit.OutcomeSuccess
	}
	if entry.PlatformTenantID == "" {
		if tenantID, err := security.GetTenantID(ctx); err == nil {
			entry.PlatformTenantID = tenantID
		}
	}
	s.resolveActor(ctx, &entry)

	if entry.ProductID == "" || entry.Action == "" || entry.TargetType == "" ||
		entry.PlatformTenantID == "" {
		logger.Error().
			Str("action", string(entry.Action)).
			Str("product_id", entry.ProductID).
			Str("target_type", entry.TargetType).
			Msg("audit log entry missing required fields; entry dropped")
		return
	}

	if _, err := s.auditLogRepo.Create(ctx, entry); err != nil {
		logger.Error().Err(err).
			Str("action", string(entry.Action)).
			Str("product_id", entry.ProductID).
			Msg("failed to create audit log entry")
	}
}

// resolveActor fills the actor fields from the request context when unset.
// Falls back to SYSTEM for flows without an authenticated principal.
func (s *auditLogService) resolveActor(ctx context.Context, entry *audit.Log) {
	if entry.ActorType != "" {
		return
	}

	actor, ok := security.GetActor(ctx)
	if !ok {
		entry.ActorType = audit.ActorTypeSystem
		return
	}

	entry.ActorType = audit.ActorType(actor.Type)
	if actor.ID != "" {
		actorID := actor.ID
		entry.ActorID = &actorID
	}
	if actor.Name != "" {
		actorName := actor.Name
		entry.ActorName = &actorName
	}

	// Platform user names are not carried in the JWT; snapshot best-effort.
	if entry.ActorName == nil && actor.Type == security.ActorTypePlatformUser &&
		entry.PlatformTenantID != "" && actor.ID != "" {
		platformUser, err := s.platformUserRepo.FindByTenantIDAndUserID(
			ctx, entry.PlatformTenantID, actor.ID,
		)
		if err != nil || platformUser == nil {
			return
		}
		name := platformUser.Name
		if name == "" {
			name = platformUser.Email
		}
		if name != "" {
			entry.ActorName = &name
		}
	}
}

func (s *auditLogService) Search(
	ctx context.Context, input audit.SearchInput,
) (search.Result[audit.Log], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[audit.Log]{}, err
	}

	result, err := s.auditLogRepo.Search(ctx, input.TenantID, input.ProductID, input.Request)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search audit logs")
		return search.Result[audit.Log]{}, err
	}

	return result, nil
}
