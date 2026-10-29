package service

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"

	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
)

// Workspaces keeps each built-in workspace registered as a resource of its
// product's workspace type, with the workspace's ID and name, under its
// Organization (ADR-0022 A6, phase 2). Every resource it writes is marked as
// a workspace's mirror: a marked resource counts only while its workspace
// exists. While the built-in Workspace lives, its routes write the resource
// in their own transaction; the resource follows the last write of either
// side.
type Workspaces interface {
	// DeclareType makes sure the product has a workspace type that may sit
	// under the Organization, creating it when absent. It answers false, and
	// logs why once per product, when the product declared the slug with
	// other parents or can't declare another type. It never runs inside a
	// transaction: a refused insert would abort it.
	DeclareType(ctx context.Context, productID string) (bool, error)
	// Put registers or renames the workspace's resource and marks it. A
	// product whose type refuses it keeps its workspace unregistered, and the
	// refusal is logged once per product.
	Put(txCtx context.Context, productID, organizationID, workspaceID, name string) error
	// Delete removes the workspace's resource, with its registered
	// descendants and every role assignment on them.
	Delete(txCtx context.Context, productID, organizationID, workspaceID string) error
}

type workspaces struct {
	resources     Service
	repo          repository.Repository
	resourceTypes rtservice.Service
	logger        zerolog.Logger
	// logged holds the product and code pairs already logged at their level;
	// later ones log at debug, so a product at its type limit logs once.
	logged sync.Map
}

func NewWorkspaces(
	resources Service, repo repository.Repository, resourceTypes rtservice.Service, logger zerolog.Logger,
) Workspaces {
	return &workspaces{
		resources:     resources,
		repo:          repo,
		resourceTypes: resourceTypes,
		logger:        logger.With().Str("component", "workspace_resources").Logger(),
	}
}

const (
	workspaceTypeDescription = "Registers the organization's workspaces as resources."
	codeIncompatibleType     = "WORKSPACE_TYPE_NOT_UNDER_ORGANIZATION"
)

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
		w.once(zerolog.ErrorLevel, productID, codeOf(refused)).Str("product_id", productID).
			Str("code", codeOf(refused)).
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
		w.once(zerolog.ErrorLevel, productID, codeIncompatibleType).Str("product_id", productID).
			Strs("parent_types", rt.ParentTypes).
			Msg("the product's workspace resource type can't sit under the organization; " +
				"its workspaces stay unregistered")
		return false, true, nil
	}
	return true, true, nil
}

func (w *workspaces) Put(txCtx context.Context, productID, organizationID, workspaceID, name string) error {
	written, err := w.resources.Put(txCtx, resource.PutInput{
		ProductID:      productID,
		OrganizationID: organizationID,
		Type:           resourcetype.WorkspaceSlug,
		ID:             workspaceID,
		Name:           name,
		Parent:         resource.OrganizationRef(organizationID),
	})
	if refused, ok := refusal(err); ok {
		w.once(zerolog.WarnLevel, productID, codeOf(refused)).Str("product_id", productID).
			Str("organization_id", organizationID).Str("workspace_id", workspaceID).Str("code", codeOf(refused)).
			Msg("workspace left unregistered as a resource")
		return nil
	}
	if err != nil {
		return err
	}
	return w.repo.MarkWorkspace(txCtx, written.RowID)
}

func (w *workspaces) Delete(txCtx context.Context, productID, organizationID, workspaceID string) error {
	err := w.resources.Delete(txCtx, productID, organizationID, resourcetype.WorkspaceSlug, workspaceID, true)
	if _, ok := refusal(err); ok {
		// Nothing registered it: no type, or no resource.
		return nil
	}
	return err
}

// once logs at the level the first time a product meets the code in this
// process, and at debug after that.
func (w *workspaces) once(level zerolog.Level, productID, code string) *zerolog.Event {
	if _, seen := w.logged.LoadOrStore(productID+"/"+code, struct{}{}); seen {
		return w.logger.Debug()
	}
	return w.logger.WithLevel(level)
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
