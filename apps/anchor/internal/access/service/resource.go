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
// model doesn't define on object: a permission scoped to a type above the
// resource's type is checked on its Organization.
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
	default:
		return e.reachResource(ctx, request, subject)
	}
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
			fallback:   organizationFallback(organizationID),
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
		return subject, located{
			object:   CheckedObject{Type: rt.Slug, Parent: &parentObject},
			fallback: organizationFallback(organizationID),
		}, nil
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
		fallback:   organizationFallback(organizationID),
		chainStart: registered.RowID,
	}, nil
}

func organizationFallback(organizationID string) *CheckedObject {
	organization := OrganizationObject(organizationID)
	return &organization
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
