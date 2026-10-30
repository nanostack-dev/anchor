package itdsl

import (
	"context"
	"net/http"
	"testing"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// AccessClient calls access evaluation with a product API key holding exactly
// the scopes the route requires. Ask questions fluently:
//
//	product.Access().User(externalID).OnResource(orgID, "document", documentID).Can("file:read").Decide()
type AccessClient struct {
	t         *testing.T
	client    *client.ClientWithResponses
	productID string
}

func (p *ProductContext) Access() AccessClient {
	p.testingContext.Helper()
	evaluator, _ := p.CreateAPIKeyClientWithScopes([]string{"product_user:read", "organization_api_key:read"})
	return p.AccessClientWith(evaluator)
}

// AccessClientWith uses another credential, for tests whose subject is the caller's scopes.
func (p *ProductContext) AccessClientWith(credential *client.ClientWithResponses) AccessClient {
	p.testingContext.Helper()
	return AccessClient{t: p.testingContext, client: credential, productID: p.ProductID}
}

// User asks about a product user named by its external user ID.
func (c AccessClient) User(externalID string) AccessQuestion {
	return c.question(client.AccessSubject{Type: client.User, Id: externalID})
}

// APIKey asks about a raw organization API key, in its own organization.
func (c AccessClient) APIKey(rawKey string) AccessQuestion {
	return c.question(client.AccessSubject{Type: client.OrganizationApiKey, Id: rawKey})
}

// As asks about any subject type, including ones the contract refuses.
func (c AccessClient) As(subjectType client.AccessSubjectType, id string) AccessQuestion {
	return c.question(client.AccessSubject{Type: subjectType, Id: id})
}

func (c AccessClient) question(subject client.AccessSubject) AccessQuestion {
	return AccessQuestion{client: c, body: client.EvaluateAccessBatchJSONRequestBody{
		Subject:     subject,
		Resource:    client.AccessResource{Type: client.Organization},
		Evaluations: &[]client.AccessEvaluationItem{},
	}}
}

// AccessQuestion is one batch evaluation under construction. Each step returns
// a copy, so a base question can be reused across subtests.
type AccessQuestion struct {
	client AccessClient
	body   client.EvaluateAccessBatchJSONRequestBody
}

// InOrganization targets an organization; "" leaves the ID out.
func (q AccessQuestion) InOrganization(organizationID string) AccessQuestion {
	q.body.Resource = client.AccessResource{Type: client.Organization, Id: optional(organizationID)}
	return q
}

// OnResource targets a resource of the product's own types; organizationID ""
// leaves the organization out. A parent named before is kept.
func (q AccessQuestion) OnResource(organizationID, resourceType, id string) AccessQuestion {
	properties := client.AccessResourceProperties{OrganizationId: optional(organizationID)}
	if q.body.Resource.Properties != nil {
		properties.Parent = q.body.Resource.Properties.Parent
	}
	q.body.Resource = client.AccessResource{Type: resourceType, Id: optional(id)}
	if properties.OrganizationId != nil || properties.Parent != nil {
		q.body.Resource.Properties = &properties
	}
	return q
}

// Under names the parent of a resource the product has not registered.
func (q AccessQuestion) Under(parentType, parentID string) AccessQuestion {
	properties := client.AccessResourceProperties{}
	if q.body.Resource.Properties != nil {
		properties = *q.body.Resource.Properties
	}
	properties.Parent = &client.ResourceParentRef{Type: parentType, Id: parentID}
	q.body.Resource.Properties = &properties
	return q
}

// Explain asks for what granted each allowed item.
func (q AccessQuestion) Explain() AccessQuestion {
	options := client.AccessEvaluationsOptions{}
	if q.body.Options != nil {
		options = *q.body.Options
	}
	options.Explain = new(true)
	q.body.Options = &options
	return q
}

// Can appends one evaluation item per action.
func (q AccessQuestion) Can(actions ...string) AccessQuestion {
	items := append([]client.AccessEvaluationItem{}, *q.body.Evaluations...)
	for _, action := range actions {
		items = append(items, client.AccessEvaluationItem{Action: &client.AccessAction{Name: action}})
	}
	q.body.Evaluations = &items
	return q
}

// Default sets the top-level action AuthZEN uses for items without one.
func (q AccessQuestion) Default(action string) AccessQuestion {
	q.body.Action = &client.AccessAction{Name: action}
	return q
}

// Items replaces the evaluation items as given, for shapes Can cannot express.
func (q AccessQuestion) Items(items ...client.AccessEvaluationItem) AccessQuestion {
	q.body.Evaluations = &items
	return q
}

func (q AccessQuestion) DenyOnFirstDeny() AccessQuestion {
	semantic := client.DenyOnFirstDeny
	options := client.AccessEvaluationsOptions{}
	if q.body.Options != nil {
		options = *q.body.Options
	}
	options.EvaluationsSemantic = &semantic
	q.body.Options = &options
	return q
}

// Decide sends the question and requires a 200.
func (q AccessQuestion) Decide() client.AccessEvaluationsResponse {
	q.client.t.Helper()
	resp := q.Send()
	require.Equal(q.client.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(q.client.t, resp.JSON200)
	return *resp.JSON200
}

// Send returns the untouched response, for refusals.
func (q AccessQuestion) Send() *client.EvaluateAccessBatchResponse {
	q.client.t.Helper()
	resp, err := q.client.client.EvaluateAccessBatchWithResponse(context.Background(), q.client.productID, q.body)
	require.NoError(q.client.t, err)
	return resp
}

// Single sends the question's first action to the single-evaluation route.
func (q AccessQuestion) Single() *client.EvaluateAccessResponse {
	q.client.t.Helper()
	action := client.AccessAction{}
	if items := *q.body.Evaluations; len(items) > 0 && items[0].Action != nil {
		action = *items[0].Action
	}
	body := client.EvaluateAccessJSONRequestBody{Subject: q.body.Subject, Resource: q.body.Resource, Action: action}
	if q.body.Options != nil && q.body.Options.Explain != nil {
		body.Options = &client.AccessEvaluationOptions{Explain: q.body.Options.Explain}
	}
	resp, err := q.client.client.EvaluateAccessWithResponse(context.Background(), q.client.productID, body)
	require.NoError(q.client.t, err)
	return resp
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
