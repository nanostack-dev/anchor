package service

import (
	"context"
	"database/sql"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/domain/workspace"
	"anchor/internal/repository"
)

type WorkspaceService interface {
	Find(ctx context.Context, input workspace.FindWorkspaceInput) (*workspace.Workspace, error)
	Create(ctx context.Context, input workspace.CreateWorkspaceInput) (workspace.Workspace, error)
	Update(ctx context.Context, input workspace.UpdateWorkspaceInput) (workspace.Workspace, error)
	Delete(ctx context.Context, input workspace.DeleteWorkspaceInput) error
	Search(
		ctx context.Context,
		input workspace.SearchWorkspacesInput,
	) (search.Result[workspace.Workspace], error)
}

type workspaceService struct {
	workspaceRepo    repository.WorkspaceRepository
	organizationRepo repository.OrganizationRepository
	db               *sql.DB
	logger           zerolog.Logger
}

func NewWorkspaceService(
	db *sql.DB,
	workspaceRepo repository.WorkspaceRepository,
	organizationRepo repository.OrganizationRepository,
	logger zerolog.Logger,
) WorkspaceService {
	return &workspaceService{
		workspaceRepo:    workspaceRepo,
		organizationRepo: organizationRepo,
		db:               db,
		logger: logger.With().Str(
			"component", "workspace_service",
		).Logger(),
	}
}

func (s *workspaceService) Find(
	ctx context.Context,
	input workspace.FindWorkspaceInput,
) (*workspace.Workspace, error) {
	if err := validateStruct(input); err != nil {
		return nil, err
	}

	return s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
		nil,
	)
}

func (s *workspaceService) Create(
	ctx context.Context,
	input workspace.CreateWorkspaceInput,
) (workspace.Workspace, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID); err != nil {
		return workspace.Workspace{}, err
	}

	if err := s.ensureNameAvailable(ctx, input.ProductID, input.OrganizationID, input.Name, ""); err != nil {
		return workspace.Workspace{}, err
	}

	newWorkspace := workspace.Workspace{
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Description:    input.Description,
	}
	newWorkspace.GenerateID()

	created, err := jetx.WithTxReturn(s.db, func(tx *sql.Tx) (workspace.Workspace, error) {
		txOptions := &jetx.DBOptions{Tx: tx}

		existing, findErr := s.workspaceRepo.FindByOrganizationIDAndName(
			ctx,
			input.ProductID,
			input.OrganizationID,
			input.Name,
			txOptions,
		)
		if findErr != nil {
			return workspace.Workspace{}, findErr
		}
		if existing != nil {
			return workspace.Workspace{}, workspace.NewNameExistsError(input.Name, input.OrganizationID)
		}

		return s.workspaceRepo.Create(ctx, newWorkspace, txOptions)
	})
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("name", input.Name).
			Msg("failed to create workspace")
		return workspace.Workspace{}, err
	}

	return created, nil
}

func (s *workspaceService) Update(
	ctx context.Context,
	input workspace.UpdateWorkspaceInput,
) (workspace.Workspace, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}

	currentWorkspace, err := s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
		nil,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("workspace_id", input.WorkspaceID).
			Msg("failed to find workspace for update")
		return workspace.Workspace{}, err
	}
	if currentWorkspace == nil {
		return workspace.Workspace{}, apierror.ErrNotFound
	}

	updated, err := jetx.WithTxReturn(s.db, func(tx *sql.Tx) (workspace.Workspace, error) {
		txOptions := &jetx.DBOptions{Tx: tx}

		updatedWorkspace := *currentWorkspace
		if input.Name != nil {
			existing, findErr := s.workspaceRepo.FindByOrganizationIDAndName(
				ctx,
				input.ProductID,
				input.OrganizationID,
				*input.Name,
				txOptions,
			)
			if findErr != nil {
				return workspace.Workspace{}, findErr
			}
			if existing != nil && existing.ID != input.WorkspaceID {
				return workspace.Workspace{}, workspace.NewNameExistsError(*input.Name, input.OrganizationID)
			}
			updatedWorkspace.Name = *input.Name
		}
		updatedWorkspace.Description = input.Description

		return s.workspaceRepo.Update(
			ctx,
			input.ProductID,
			input.OrganizationID,
			updatedWorkspace,
			txOptions,
		)
	})
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("workspace_id", input.WorkspaceID).
			Msg("failed to update workspace")
		return workspace.Workspace{}, err
	}

	return updated, nil
}

func (s *workspaceService) Delete(
	ctx context.Context,
	input workspace.DeleteWorkspaceInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	currentWorkspace, err := s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
		nil,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("workspace_id", input.WorkspaceID).
			Msg("failed to find workspace for deletion")
		return err
	}
	if currentWorkspace == nil {
		return apierror.ErrNotFound
	}

	return s.workspaceRepo.DeleteByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
		nil,
	)
}

func (s *workspaceService) Search(
	ctx context.Context,
	input workspace.SearchWorkspacesInput,
) (search.Result[workspace.Workspace], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[workspace.Workspace]{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID); err != nil {
		return search.Result[workspace.Workspace]{}, err
	}

	result, err := s.workspaceRepo.SearchByOrganizationID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.Request,
		nil,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Msg("failed to search workspaces")
		return search.Result[workspace.Workspace]{}, err
	}

	return result, nil
}

func (s *workspaceService) ensureOrganizationExists(
	ctx context.Context,
	productID string,
	organizationID string,
) error {
	org, err := s.organizationRepo.FindByID(ctx, productID, organizationID, nil)
	if err != nil {
		return err
	}
	if org == nil {
		return apierror.ErrNotFound
	}

	return nil
}

func (s *workspaceService) ensureNameAvailable(
	ctx context.Context,
	productID string,
	organizationID string,
	name string,
	currentWorkspaceID string,
) error {
	existing, err := s.workspaceRepo.FindByOrganizationIDAndName(
		ctx,
		productID,
		organizationID,
		name,
		nil,
	)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != currentWorkspaceID {
		return workspace.NewNameExistsError(name, organizationID)
	}

	return nil
}
