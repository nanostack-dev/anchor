package anchorsdk

import (
	"context"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// AccessCheck builds one access evaluation: may this subject perform these
// permissions on this organization, workspace or resource? Start one from
// [Client.AccessForUser] or [Client.AccessForAPIKey]:
//
//	decision, err := c.AccessForUser(externalUserID).
//	    OnResource(orgID, "document", documentID).
//	    Under("folder", folderID).
//	    Permissions("document:view").
//	    Do(ctx)
//
// A denial is not an error: Do returns the decision with Decision false and
// Context.Reason set. Errors are transport, credential or request failures.
type AccessCheck struct {
	c    *Client
	body nanoclient.AccessEvaluationsRequest
}

// AccessForUser evaluates a product user named by its external (identity
// provider) user ID.
func (c *Client) AccessForUser(externalUserID string) *AccessCheck {
	return c.accessCheck(nanoclient.AccessSubject{Type: nanoclient.User, Id: externalUserID})
}

// AccessForAPIKey evaluates a raw organization API key. The key's own
// organization is used unless InOrganization or InWorkspace names one.
func (c *Client) AccessForAPIKey(rawKey string) *AccessCheck {
	return c.accessCheck(nanoclient.AccessSubject{Type: nanoclient.OrganizationApiKey, Id: rawKey})
}

func (c *Client) accessCheck(subject nanoclient.AccessSubject) *AccessCheck {
	return &AccessCheck{c: c, body: nanoclient.AccessEvaluationsRequest{
		Subject:     subject,
		Resource:    nanoclient.AccessResource{Type: nanoclient.Organization},
		Evaluations: &[]nanoclient.AccessEvaluationItem{},
	}}
}

// InOrganization targets an organization. An empty ID means the API key's own
// organization; a user subject needs an ID.
func (a *AccessCheck) InOrganization(organizationID string) *AccessCheck {
	a.body.Resource = nanoclient.AccessResource{Type: nanoclient.Organization, Id: optional(organizationID)}
	return a
}

// InWorkspace targets a workspace of an organization. organizationID may be
// empty for an API key subject.
func (a *AccessCheck) InWorkspace(organizationID, workspaceID string) *AccessCheck {
	resource := nanoclient.AccessResource{Type: nanoclient.Workspace, Id: &workspaceID}
	if organizationID != "" {
		resource.Properties = &nanoclient.AccessResourceProperties{OrganizationId: &organizationID}
	}
	a.body.Resource = resource
	return a
}

// OnResource targets a resource of one of the product's resource types, named
// by the product's own ID. organizationID may be empty for an API key subject.
func (a *AccessCheck) OnResource(organizationID, resourceType, id string) *AccessCheck {
	resource := nanoclient.AccessResource{Type: resourceType, Id: &id}
	if organizationID != "" {
		resource.Properties = &nanoclient.AccessResourceProperties{OrganizationId: &organizationID}
	}
	a.body.Resource = resource
	return a
}

// Under names the parent of a resource the product has not registered: a
// registered resource, or the organization ("organization", its ID). A
// registered resource is checked under its stored parent instead.
func (a *AccessCheck) Under(parentType, parentID string) *AccessCheck {
	properties := nanoclient.AccessResourceProperties{}
	if a.body.Resource.Properties != nil {
		properties = *a.body.Resource.Properties
	}
	properties.Parent = &nanoclient.ResourceParentRef{Type: parentType, Id: parentID}
	a.body.Resource.Properties = &properties
	return a
}

// Explain asks for what granted each allowed permission, in each evaluation's
// Context.GrantedBy.
func (a *AccessCheck) Explain() *AccessCheck {
	a.options().Explain = new(true)
	return a
}

// Permissions adds the actions to decide. With none, the check asks only
// whether the subject can reach the resource.
func (a *AccessCheck) Permissions(names ...string) *AccessCheck {
	for _, name := range names {
		*a.body.Evaluations = append(*a.body.Evaluations, nanoclient.AccessEvaluationItem{
			Action: &nanoclient.AccessAction{Name: name},
		})
	}
	return a
}

// DenyOnFirstDeny stops at the first missing permission; later ones report NOT_EVALUATED.
func (a *AccessCheck) DenyOnFirstDeny() *AccessCheck {
	semantic := nanoclient.DenyOnFirstDeny
	a.options().EvaluationsSemantic = &semantic
	return a
}

func (a *AccessCheck) options() *nanoclient.AccessEvaluationsOptions {
	if a.body.Options == nil {
		a.body.Options = &nanoclient.AccessEvaluationsOptions{}
	}
	return a.body.Options
}

// Do sends the evaluation. It is retried on transient failures: evaluating has
// no effect beyond an API key's hourly last_used_at update.
func (a *AccessCheck) Do(ctx context.Context) (*nanoclient.AccessEvaluationsResponse, error) {
	const op = "AccessCheck.Do"
	return retrying(ctx, a.c, func(ctx context.Context) (*nanoclient.AccessEvaluationsResponse, error) {
		resp, err := a.c.api.EvaluateAccessBatchWithResponse(ctx, a.c.productID, a.body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
