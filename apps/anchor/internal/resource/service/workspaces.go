package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"

	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
	rtservice "anchor/internal/resourcetype/service"
)

// Workspaces keeps each built-in workspace registered as a resource of its
// product's workspace type, with the workspace's ID and name, under its
// Organization (ADR-0022 A6, phase 2). While the built-in Workspace lives,
// its routes write the reference in their own transaction; the reference
// follows the last write of either side.
type Workspaces interface {
	// DeclareType makes sure the product has a workspace type that may sit
	// under the Organization, creating it when absent. It answers false, and
	// logs why, when the product declared the slug with other parents or can't
	// declare another type.
	DeclareType(ctx context.Context, productID string) (bool, error)
	// Put registers or renames the workspace's reference. A product whose
	// type refuses it keeps its workspace unregistered, and the refusal is
	// logged.
	Put(txCtx context.Context, productID, organizationID, workspaceID, name string) error
	// Delete removes the workspace's reference, with its registered
	// descendants and every role assignment on them.
	Delete(txCtx context.Context, productID, organizationID, workspaceID string) error
}

type workspaces struct {
	resources     Service
	resourceTypes rtservice.Service
	logger        zerolog.Logger
}

func NewWorkspaces(resources Service, resourceTypes rtservice.Service, logger zerolog.Logger) Workspaces {
	return &workspaces{
		resources:     resources,
		resourceTypes: resourceTypes,
		logger:        logger.With().Str("component", "workspace_resources").Logger(),
	}
}

const workspaceTypeDescription = "Registers the organization's workspaces as resources."

func (w *workspaces) DeclareType(ctx context.Context, productID string) (bool, error) {
	if productID == "" {
		return false, errors.New("declare workspace type: no product")
	}
	declared, found, err := w.declared(ctx, productID)
	if err != nil || found {
		return declared, err
	}
	description := workspaceTypeDescription
	_, err = w.resourceTypes.Create(ctx, resourcetype.CreateInput{
		ProductID:   productID,
		Slug:        resourcetype.WorkspaceSlug,
		Name:        resourcetype.WorkspaceName,
		Description: &description,
		ParentTypes: []string{resourcetype.OrganizationSlug},
	})
	refused, isFault := fault.As(err)
	switch {
	case err == nil:
		return true, nil
	case isFault && codeOf(refused) == resourcetype.CodeSlugDuplicate:
		// Another write declared it first.
		declared, _, err = w.declared(ctx, productID)
		return declared, err
	case isFault && refused.HTTPStatus() < http.StatusInternalServerError:
		w.logger.Error().Str("product_id", productID).Str("code", codeOf(refused)).
			Msg("the product can't declare the workspace resource type; its workspaces stay unregistered")
		return false, nil
	default:
		return false, err
	}
}

// declared reports whether the product's tree holds the workspace type, and
// whether that type may sit under the Organization.
func (w *workspaces) declared(ctx context.Context, productID string) (bool, bool, error) {
	tree, err := w.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return false, false, err
	}
	rt, found := tree.BySlug(resourcetype.WorkspaceSlug).Get()
	if !found {
		return false, false, nil
	}
	if !rt.SitsUnderOrganization() {
		w.logger.Error().Str("product_id", productID).Strs("parent_types", rt.ParentTypes).
			Msg("the product's workspace resource type can't sit under the organization; " +
				"its workspaces stay unregistered")
		return false, true, nil
	}
	return true, true, nil
}

func (w *workspaces) Put(txCtx context.Context, productID, organizationID, workspaceID, name string) error {
	_, err := w.resources.Put(txCtx, resource.PutInput{
		ProductID:      productID,
		OrganizationID: organizationID,
		Type:           resourcetype.WorkspaceSlug,
		ID:             workspaceID,
		Name:           name,
		Parent:         resource.OrganizationRef(organizationID),
	})
	if refused, ok := refusal(err); ok {
		w.logger.Warn().Str("product_id", productID).Str("organization_id", organizationID).
			Str("workspace_id", workspaceID).Str("code", codeOf(refused)).
			Msg("workspace left unregistered as a resource")
		return nil
	}
	return err
}

func (w *workspaces) Delete(txCtx context.Context, productID, organizationID, workspaceID string) error {
	err := w.resources.Delete(txCtx, productID, organizationID, resourcetype.WorkspaceSlug, workspaceID, true)
	if _, ok := refusal(err); ok {
		// Nothing registered it: no type, or no reference.
		return nil
	}
	return err
}

// refusal reports a client error: a rule refused the write before it changed
// anything, so the workspace's own write goes on.
func refusal(err error) (*fault.Error, bool) {
	refused, ok := fault.As(err)
	if !ok || refused.HTTPStatus() >= http.StatusInternalServerError {
		return nil, false
	}
	return refused, true
}

func codeOf(refused *fault.Error) string {
	if len(refused.Details) == 0 {
		return ""
	}
	return refused.Details[0].Code
}
