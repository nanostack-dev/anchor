package service

import (
	"context"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/workspace"
	"anchor/internal/events"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

const defaultWorkspaceName = "Default"

func (s *workspaceService) createDefault(
	ctx context.Context,
	input workspace.CreateWorkspaceInput,
) (workspace.Workspace, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Workspace{}, err
	}
	productID, organizationID := input.ProductID, input.OrganizationID
	var result workspace.Workspace
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.lockOrganization(txCtx, productID, organizationID); lockErr != nil {
			return lockErr
		}
		found, findErr := s.workspaceRepo.FindDefault(txCtx, productID, organizationID)
		if findErr != nil {
			return findErr
		}
		if found.IsPresent() {
			result = found.Value()
			return nil
		}
		byName, nameErr := s.workspaceRepo.FindByOrganizationIDAndName(
			txCtx,
			productID,
			organizationID,
			input.Name,
		)
		if nameErr != nil {
			return nameErr
		}
		if byName.IsPresent() {
			current := byName.Value()
			var updateErr error
			result, updateErr = s.workspaceRepo.DesignateDefault(txCtx, productID, organizationID, current.ID)
			if updateErr != nil {
				return updateErr
			}
			return s.emitWorkspace(txCtx, events.WorkspaceUpdated, productID, organizationID, result.ID)
		}
		created := workspace.Workspace{
			OrganizationID: organizationID,
			Name:           input.Name,
			Description:    input.Description,
			IsDefault:      true,
		}
		created.GenerateID()
		var createErr error
		result, createErr = s.workspaceRepo.Create(txCtx, created)
		if createErr != nil {
			return createErr
		}
		return s.emitWorkspace(txCtx, events.WorkspaceCreated, productID, organizationID, result.ID)
	})
	return result, err
}

func (s *workspaceService) lockOrganization(ctx context.Context, productID, organizationID string) error {
	found, err := s.organizationRepo.FindByID(transactor.ForUpdate(ctx), productID, organizationID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
	}
	return nil
}
