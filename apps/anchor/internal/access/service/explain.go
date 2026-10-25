package service

import (
	"context"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/resource"
	"anchor/internal/domain/roleassignment"
	resourcerepository "anchor/internal/resource/repository"
)

// explain fills what granted each allowed item from Anchor's tables, once
// OpenFGA allowed it: the subject's roles on the resource and its ancestors,
// nearest first, then its Organization role, or the API key's own
// permission. It reads the chain and the assignments in one statement each,
// whatever the number of actions.
func (e *evaluator) explain(
	ctx context.Context, request Request, identity *Identity, target located, decision *Decision,
) error {
	var allowed []string
	for i, item := range decision.Items {
		if item.Allowed {
			allowed = append(allowed, request.Actions[i])
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	chain, err := e.chain(ctx, identity.OrganizationID, target.chainStart)
	if err != nil {
		return err
	}
	var held []roleassignment.Grant
	if identity.ProductUserID != "" && len(chain) > 0 {
		rowIDs := functional.Slice(chain).Map(func(n resourcerepository.Node) string { return n.RowID })
		if held, err = e.assignments.Grants(ctx, identity.OrganizationID, identity.ProductUserID, rowIDs,
			functional.Distinct(allowed)); err != nil {
			return err
		}
	}
	for i := range decision.Items {
		if decision.Items[i].Allowed {
			decision.Items[i].GrantedBy = grantsFor(request.Actions[i], identity, chain, held)
		}
	}
	return nil
}

func (e *evaluator) chain(ctx context.Context, organizationID, start string) ([]resourcerepository.Node, error) {
	if start == "" {
		return nil, nil
	}
	return e.resources.Ancestors(ctx, organizationID, start, resource.MaxDepth)
}

// grantsFor orders the grants of one action nearest first: each resource of
// the chain from the resource up, then the Organization.
func grantsFor(
	action string, identity *Identity, chain []resourcerepository.Node, held []roleassignment.Grant,
) []Grant {
	grants := []Grant{}
	for _, node := range chain {
		for _, grant := range held {
			if grant.ResourceRowID == node.RowID && grant.Permission == action {
				grants = append(grants, Grant{
					RoleID:   grant.Role.ID,
					RoleName: grant.Role.Name,
					Resource: GrantResource{Type: node.Ref.Type, ID: node.Ref.ID},
				})
			}
		}
	}
	if !slices.Contains(identity.Permissions, action) {
		return grants
	}
	organization := GrantResource{Type: string(ResourceOrganization), ID: identity.OrganizationID}
	if identity.APIKeyID != "" {
		return append(grants, Grant{
			APIKeyID: identity.APIKeyID, APIKeyName: identity.APIKeyName, Resource: organization,
		})
	}
	return append(grants, Grant{RoleID: identity.RoleID, RoleName: identity.RoleName, Resource: organization})
}
