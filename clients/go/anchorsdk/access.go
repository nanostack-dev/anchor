package anchorsdk

import (
	"context"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// AccessCheck builds one access evaluation: may this subject perform these
// permissions on this organization or workspace? Start one from
// [Client.AccessForUser] or [Client.AccessForAPIKey]:
//
//	decision, err := c.AccessForUser(externalUserID).
//	    InWorkspace(orgID, workspaceID).
//	    Permissions("flows:read").
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
	a.body.Options = &nanoclient.AccessEvaluationsOptions{EvaluationsSemantic: &semantic}
	return a
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
