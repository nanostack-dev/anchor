package service

import (
	"context"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/workspace"
	"anchor/internal/events"
	"anchor/internal/repository"
	resourceservice "anchor/internal/resource/service"
	"anchor/internal/searchx"
)

type WorkspaceService interface {
	ListByOrganizations(
		ctx context.Context,
		input workspace.ListByOrganizationsInput,
	) (map[string][]workspace.Workspace, error)
	Find(ctx context.Context, input workspace.FindWorkspaceInput) (workspace.Workspace, error)
	Create(ctx context.Context, input workspace.CreateWorkspaceInput) (workspace.Workspace, error)
	Update(ctx context.Context, input workspace.UpdateWorkspaceInput) (workspace.Workspace, error)
	Delete(ctx context.Context, input workspace.DeleteWorkspaceInput) error
	Search(
		ctx context.Context,
		input workspace.SearchWorkspacesInput,
	) (search.Result[workspace.Workspace], error)
	// RegisterResource declares the product's workspace resource type and
	// registers the workspace as a resource, for a workspace created inside a
	// caller's transaction, once that transaction committed. Inside a
	// transaction it does nothing; the next start's backfill registers it.
	RegisterResource(ctx context.Context, productID, organizationID, workspaceID string) error
}

func (s *workspaceService) ListByOrganizations(
	ctx context.Context, input workspace.ListByOrganizationsInput,
) (map[string][]workspace.Workspace, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	return s.workspaceRepo.ListByOrganizations(ctx, input.ProductID, input.OrganizationIDs)
}

// workspaceService writes the built-in workspaces, and mirrors each write to
// the workspace's resource reference in the same transaction (ADR-0022 A6,
// phase 2).
type workspaceService struct {
	workspaceRepo    repository.WorkspaceRepository
	organizationRepo repository.OrganizationRepository
	resources        resourceservice.Workspaces
	transactor       transactor.Transactor
	events           events.Emitter
	logger           zerolog.Logger
}

func NewWorkspaceService(
	workspaceRepo repository.WorkspaceRepository,
	organizationRepo repository.OrganizationRepository,
	resources resourceservice.Workspaces,
	transactor transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) WorkspaceService {
	return &workspaceService{
		workspaceRepo:    workspaceRepo,
		organizationRepo: organizationRepo,
		resources:        resources,
		transactor:       transactor,
		events:           eventEmitter,
		logger: logger.With().Str(
			"component", "workspace_service",
		).Logger(),
	}
}

func (s *workspaceService) emitWorkspace(
	ctx context.Context, eventType events.Type, productID, organizationID, workspaceID string,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data: events.Data{
			events.FieldOrganizationID: organizationID,
			events.FieldWorkspaceID:    workspaceID,
		},
	})
}

func (s *workspaceService) Find(
	ctx context.Context,
	input workspace.FindWorkspaceInput,
) (workspace.Workspace, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}

	found, err := s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
	)
	if err != nil {
		return workspace.Workspace{}, err
	}
	return found.ToResult(workspace.ErrWorkspaceNotFound).Value()
}

func (s *workspaceService) Create(
	ctx context.Context,
	input workspace.CreateWorkspaceInput,
) (workspace.Workspace, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID); err != nil {
		return workspace.Workspace{}, err
	}

	if err := s.ensureNameAvailable(ctx, input.ProductID, input.OrganizationID, input.Name, ""); err != nil {
		return workspace.Workspace{}, err
	}

	if transactor.CurrentTx(ctx) == nil {
		if _, err := s.resources.DeclareType(ctx, input.ProductID); err != nil {
			return workspace.Workspace{}, err
		}
	}

	newWorkspace := workspace.Workspace{
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Description:    input.Description,
	}
	newWorkspace.GenerateID()

	var created workspace.Workspace
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureOrganizationExists(
			transactor.ForUpdate(txCtx),
			input.ProductID,
			input.OrganizationID,
		); err != nil {
			return err
		}

		existing, findErr := s.workspaceRepo.FindByOrganizationIDAndName(
			txCtx,
			input.ProductID,
			input.OrganizationID,
			input.Name,
		)
		if findErr != nil {
			return findErr
		}
		if existing.IsPresent() {
			return workspace.NewNameExistsError(input.Name, input.OrganizationID)
		}

		var createErr error
		created, createErr = s.workspaceRepo.Create(txCtx, newWorkspace)
		if errors.Is(createErr, repository.ErrWorkspaceNameExists) {
			return workspace.NewNameExistsError(input.Name, input.OrganizationID)
		}
		if createErr != nil {
			return createErr
		}
		if err := s.emitWorkspace(
			txCtx, events.WorkspaceCreated, input.ProductID, input.OrganizationID, created.ID,
		); err != nil {
			return err
		}
		return s.resources.Put(txCtx, input.ProductID, input.OrganizationID, created.ID, created.Name)
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

	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}

	foundWorkspace, err := s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("workspace_id", input.WorkspaceID).
			Msg("failed to find workspace for update")
		return workspace.Workspace{}, err
	}
	if foundWorkspace.IsAbsent() {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	currentWorkspace := foundWorkspace.Value()

	var updated workspace.Workspace
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		updatedWorkspace := currentWorkspace
		if input.Name != nil {
			existing, findErr := s.workspaceRepo.FindByOrganizationIDAndName(
				txCtx,
				input.ProductID,
				input.OrganizationID,
				*input.Name,
			)
			if findErr != nil {
				return findErr
			}
			if existing.IsPresent() && existing.Value().ID != input.WorkspaceID {
				return workspace.NewNameExistsError(*input.Name, input.OrganizationID)
			}
			updatedWorkspace.Name = *input.Name
		}
		updatedWorkspace.Description = input.Description

		var updateErr error
		updated, updateErr = s.workspaceRepo.Update(
			txCtx,
			input.ProductID,
			input.OrganizationID,
			updatedWorkspace,
		)
		if errors.Is(updateErr, repository.ErrWorkspaceNameExists) {
			return workspace.NewNameExistsError(updatedWorkspace.Name, input.OrganizationID)
		}
		if updateErr != nil {
			return updateErr
		}
		if emitErr := s.emitWorkspace(
			txCtx, events.WorkspaceUpdated, input.ProductID, input.OrganizationID, updated.ID,
		); emitErr != nil {
			return emitErr
		}
		return s.resources.Put(txCtx, input.ProductID, input.OrganizationID, updated.ID, updated.Name)
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

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	foundWorkspace, err := s.workspaceRepo.FindByID(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.WorkspaceID,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("workspace_id", input.WorkspaceID).
			Msg("failed to find workspace for deletion")
		return err
	}
	if foundWorkspace.IsAbsent() {
		return workspace.ErrWorkspaceNotFound
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if delErr := s.workspaceRepo.DeleteByID(
			txCtx,
			input.ProductID,
			input.OrganizationID,
			input.WorkspaceID,
		); delErr != nil {
			return delErr
		}
		if emitErr := s.emitWorkspace(
			txCtx, events.WorkspaceDeleted, input.ProductID, input.OrganizationID, input.WorkspaceID,
		); emitErr != nil {
			return emitErr
		}
		return s.resources.Delete(txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID)
	})
}

// RegisterResource declares the type outside any transaction: a refused type
// insert would abort it, and the catalog lock would come after the
// Organization's row, against the order of resource writes.
func (s *workspaceService) RegisterResource(ctx context.Context, productID, organizationID, workspaceID string) error {
	if productID == "" || organizationID == "" || workspaceID == "" {
		return fault.ErrBadRequest
	}
	if transactor.CurrentTx(ctx) != nil {
		return nil
	}
	declared, err := s.resources.DeclareType(ctx, productID)
	if err != nil || !declared {
		return err
	}
	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		found, findErr := s.workspaceRepo.FindByID(txCtx, productID, organizationID, workspaceID)
		if findErr != nil {
			return findErr
		}
		ws, exists := found.Get()
		if !exists {
			return nil
		}
		return s.resources.Put(txCtx, productID, organizationID, ws.ID, ws.Name)
	})
}

func (s *workspaceService) Search(
	ctx context.Context,
	input workspace.SearchWorkspacesInput,
) (search.Result[workspace.Workspace], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := input.Validate(); err != nil {
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
	)
	if err != nil {
		if !searchx.IsClientError(err) {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("organization_id", input.OrganizationID).
				Msg("failed to search workspaces")
		}
		return search.Result[workspace.Workspace]{}, err
	}

	return result, nil
}

func (s *workspaceService) ensureOrganizationExists(
	ctx context.Context,
	productID string,
	organizationID string,
) error {
	found, err := s.organizationRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
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
	)
	if err != nil {
		return err
	}
	if existing.IsPresent() && existing.Value().ID != currentWorkspaceID {
		return workspace.NewNameExistsError(name, organizationID)
	}

	return nil
}
