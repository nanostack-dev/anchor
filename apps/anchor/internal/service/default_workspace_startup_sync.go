package service

import (
	"context"
	"errors"
	"fmt"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/workspace"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const defaultWorkspaceStartupPageSize = 200

type DefaultWorkspaceStartupSync struct {
	organizations repository.OrganizationRepository
	workspaces    WorkspaceService
}

func NewDefaultWorkspaceStartupSync(
	organizations repository.OrganizationRepository,
	workspaces WorkspaceService,
) *DefaultWorkspaceStartupSync {
	return &DefaultWorkspaceStartupSync{organizations: organizations, workspaces: workspaces}
}

// Run is the startup-only sweep across all products. Each organization's write commits independently.
func (s *DefaultWorkspaceStartupSync) Run(ctx context.Context) error {
	var afterID string
	for {
		page, err := s.organizations.ListWithoutDefaultWorkspaceInternal(ctx, afterID, defaultWorkspaceStartupPageSize)
		if err != nil {
			return fmt.Errorf("list organizations without default workspaces: %w", err)
		}
		for _, org := range page {
			if _, ensureErr := s.workspaces.EnsureDefault(
				ctx,
				workspace.EnsureDefaultWorkspaceInput{ProductID: org.ProductID, OrganizationID: org.ID},
			); ensureErr != nil &&
				!errors.Is(ensureErr, organization.ErrOrganizationNotFound) {
				return fmt.Errorf("ensure default workspace: %w", ensureErr)
			}
		}
		if len(page) < defaultWorkspaceStartupPageSize {
			return nil
		}
		afterID = page[len(page)-1].ID
	}
}

type defaultWorkspaceStartupSyncParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Sync      *DefaultWorkspaceStartupSync
	Logger    zerolog.Logger
}

func RegisterDefaultWorkspaceStartupSync(p defaultWorkspaceStartupSyncParams) {
	p.Lifecycle.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		if err := p.Sync.Run(ctx); err != nil {
			p.Logger.Error().Err(err).Msg("default workspace startup sweep failed")
			return err
		}
		p.Logger.Info().Msg("default workspace startup sweep complete")
		return nil
	}})
}
