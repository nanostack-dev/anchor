package service

import (
	"context"
	"strings"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/workspace"
	"anchor/internal/events"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

const defaultWorkspaceName = "Default"

func (s *workspaceService) EnsureDefault(
	ctx context.Context,
	productID, organizationID string,
) (workspace.Workspace, error) {
	if strings.TrimSpace(productID) == "" || strings.TrimSpace(organizationID) == "" {
		return workspace.Workspace{}, fault.BadRequest(
			"INVALID_WORKSPACE_ORGANIZATION",
			"Product and organization IDs are required.",
		)
	}
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
			defaultWorkspaceName,
		)
		if nameErr != nil {
			return nameErr
		}
		if byName.IsPresent() {
			current := byName.Value()
			current.IsDefault = true
			var updateErr error
			result, updateErr = s.workspaceRepo.Update(txCtx, productID, organizationID, current)
			if updateErr != nil {
				return updateErr
			}
			return s.emitWorkspace(txCtx, events.WorkspaceUpdated, productID, organizationID, result.ID)
		}
		created := workspace.Workspace{OrganizationID: organizationID, Name: defaultWorkspaceName, IsDefault: true}
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
