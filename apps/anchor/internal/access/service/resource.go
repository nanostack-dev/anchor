package service

import (
	"context"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/log"

	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
)

// located is the object a decision checks, and the registered resource its
// chain of ancestors starts at: the resource itself, or the parent an
// unregistered one names. chainStart is empty when the chain is the
// Organization alone. fallback, when set, is checked for the actions the
// model doesn't define on object.
type located struct {
	object     CheckedObject
	fallback   *CheckedObject
	chainStart string
}

// locate resolves what the decision checks once the subject reached its
// Organization.
func (e *evaluator) locate(ctx context.Context, request Request, subject reach) (reach, located, error) {
	switch request.Resource.Type {
	case ResourceOrganization:
		return subject, located{object: OrganizationObject(subject.identity.OrganizationID)}, nil
	case ResourceWorkspace:
		return e.reachWorkspace(ctx, request, subject)
	default:
		return e.reachResource(ctx, request, subject)
	}
}

// reachWorkspace resolves a workspace through the generic model (ADR-0022
// A6): the resource registered in the Organization under the product's
// workspace type, checked on its own object, so Organization roles reach it
// through its parent and roles assigned on it grant too. An action scoped
// above the workspace type, such as every Organization-scoped permission, is
// checked on the Organization, as it was before workspaces were registered.
//
// A resource the workspace mirror wrote counts only while its workspace
// exists: one an earlier release deleted without its resource stops like an
// unknown workspace. A resource the product registered itself is its own
// statement that the workspace exists, as for any type. A built-in workspace
// without a resource, because the product's type refused it or an earlier
// release created it, is checked on its Organization alone. Unknown and
// foreign workspaces stop the same way.
func (e *evaluator) reachWorkspace(ctx context.Context, request Request, subject reach) (reach, located, error) {
	organizationID := subject.identity.OrganizationID
	organization := OrganizationObject(organizationID)
	registered, found, err := e.workspaceResource(ctx, request, organizationID)
	if err != nil {
		return reach{}, located{}, err
	}
	if found {
		if gone, goneErr := e.resources.MirrorsDeletedWorkspace(ctx, registered.RowID); goneErr != nil || gone {
			return stoppedBy(ReasonWorkspaceNotInOrganization), located{},
				e.unexpectedOrNil(goneErr, request, "workspace mirror")
		}
		return subject, located{
			object:     CheckedObject{Type: registered.Type, ID: registered.RowID},
			fallback:   &organization,
			chainStart: registered.RowID,
		}, nil
	}
	builtIn, err := e.workspaces.FindByID(ctx, request.ProductID, organizationID, request.Resource.ID)
	if err != nil {
		return reach{}, located{}, e.unexpected(err, request, "workspace")
	}
	if builtIn.IsAbsent() {
		return stoppedBy(ReasonWorkspaceNotInOrganization), located{}, nil
	}
	return subject, located{object: organization}, nil
}

// workspaceResource finds the resource of the product's workspace type that
// the request names in the Organization.
func (e *evaluator) workspaceResource(
	ctx context.Context, request Request, organizationID string,
) (resource.Resource, bool, error) {
	tree, err := e.resourceTypes.Tree(ctx, request.ProductID)
	if err != nil {
		return resource.Resource{}, false, e.unexpected(err, request, "resource types")
	}
	rt, declared := tree.BySlug(resourcetype.WorkspaceSlug).Get()
	if !declared {
		return resource.Resource{}, false, nil
	}
	found, err := e.resources.Find(ctx, organizationID, rt.ID, request.Resource.ID)
	if err != nil {
		return resource.Resource{}, false, e.unexpected(err, request, "workspace resource")
	}
	registered, ok := found.Get()
	registered.Type = rt.Slug
	return registered, ok, nil
}

// reachResource resolves a resource of the Product's own types in Postgres,
// in this order (ADR-0022 B1, E2): its type, the resource registered in the
// Organization, and, for one the registry lacks, the parent the request
// names.
func (e *evaluator) reachResource(ctx context.Context, request Request, subject reach) (reach, located, error) {
	organizationID := subject.identity.OrganizationID
	tree, err := e.resourceTypes.Tree(ctx, request.ProductID)
	if err != nil {
		return reach{}, located{}, e.unexpected(err, request, "resource types")
	}
	rt, declared := tree.BySlug(string(request.Resource.Type)).Get()
	if !declared {
		return stoppedBy(ReasonResourceTypeNotFound), located{}, nil
	}
	found, err := e.resources.Find(ctx, organizationID, rt.ID, request.Resource.ID)
	if err != nil {
		return reach{}, located{}, e.unexpected(err, request, "resource")
	}
	if registered, ok := found.Get(); ok {
		e.reportParentMismatch(ctx, request, registered)
		return subject, located{
			object:     CheckedObject{Type: rt.Slug, ID: registered.RowID},
			chainStart: registered.RowID,
		}, nil
	}
	if request.Resource.Parent == nil {
		// Another Organization's resource of the same ID is a different one
		// (ADR-0022 B2), and saying it exists would leak it.
		return stoppedBy(ReasonResourceNotFound), located{}, nil
	}
	return e.reachParent(ctx, request, tree, rt, subject)
}

// reachParent resolves the parent an unregistered resource names: a type the
// resource's type may sit under, and the Organization itself or a resource
// registered in it. The check then gives OpenFGA that parent for this check
// only.
func (e *evaluator) reachParent(
	ctx context.Context, request Request, tree resourcetype.Tree, rt resourcetype.ResourceType, subject reach,
) (reach, located, error) {
	parent := *request.Resource.Parent
	organizationID := subject.identity.OrganizationID
	allowed := slices.Contains(rt.ParentTypes, parent.Type) || (rt.NestsInItself && parent.Type == rt.Slug)
	parentType, declared := tree.BySlug(parent.Type).Get()
	if !allowed || !declared {
		return stoppedBy(ReasonParentTypeNotAllowed), located{}, nil
	}
	if parentType.BuiltIn() {
		if parent.ID != organizationID {
			return stoppedBy(ReasonResourceNotInOrganization), located{}, nil
		}
		parentObject := OrganizationObject(organizationID)
		return subject, located{object: CheckedObject{Type: rt.Slug, Parent: &parentObject}}, nil
	}
	found, err := e.resources.Find(ctx, organizationID, parentType.ID, parent.ID)
	if err != nil {
		return reach{}, located{}, e.unexpected(err, request, "parent")
	}
	registered, ok := found.Get()
	if !ok {
		reason, lookupErr := e.missingParent(ctx, request, organizationID, parentType.ID, parent.ID)
		return stoppedBy(reason), located{}, lookupErr
	}
	return subject, located{
		object:     CheckedObject{Type: rt.Slug, Parent: &CheckedObject{Type: parentType.Slug, ID: registered.RowID}},
		chainStart: registered.RowID,
	}, nil
}

// missingParent tells a parent another Organization of the Product registered
// from one no Organization did; it costs a query only on that deny path.
func (e *evaluator) missingParent(
	ctx context.Context, request Request, organizationID, typeID, externalID string,
) (DenyReason, error) {
	elsewhere, err := e.resources.RegisteredElsewhere(ctx, request.ProductID, organizationID, typeID, externalID)
	switch {
	case err != nil:
		return "", e.unexpected(err, request, "parent elsewhere")
	case elsewhere:
		return ReasonResourceNotInOrganization, nil
	default:
		return ReasonResourceNotFound, nil
	}
}

// reportParentMismatch logs, sampled, a request whose parent differs from
// the registered resource's stored one; the stored parent decides.
func (e *evaluator) reportParentMismatch(ctx context.Context, request Request, registered resource.Resource) {
	named := request.Resource.Parent
	if named == nil || (resource.Ref{Type: named.Type, ID: named.ID}) == registered.Parent {
		return
	}
	logger := log.Ctx(ctx).Sample(e.parentSampler)
	logger.Warn().
		Str("event", "access.parent_mismatch").
		Str("product_id", request.ProductID).
		Str("organization_id", registered.OrganizationID).
		Str("resource", registered.Ref().String()).
		Str("stored_parent", registered.Parent.String()).
		Str("named_parent", resource.Ref{Type: named.Type, ID: named.ID}.String()).
		Msg("access.parent_mismatch")
}
