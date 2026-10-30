package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesssvc "anchor/internal/access/service"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
	resourcerepository "anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
	coresvc "anchor/internal/service"
)

const (
	testOrganization = "org_1"
	testKeyID        = "oak_1"
)

// keyService introspects every key as an active key of testOrganization
// holding document:view.
type keyService struct {
	coresvc.OrganizationAPIKeyService
}

func (keyService) IntrospectAPIKey(
	context.Context, orgapikey.IntrospectOrganizationAPIKeyInput,
) (orgapikey.ValidateOrganizationAPIKeyScopesOutput, error) {
	return orgapikey.ValidateOrganizationAPIKeyScopesOutput{
		APIKey:      orgapikey.OrganizationAPIKey{ID: testKeyID, OrganizationID: testOrganization, Name: "Key"},
		Permissions: []string{"document:view"},
	}, nil
}

// holdsAll answers every action held.
type holdsAll struct{}

func (holdsAll) Holds(_ context.Context, check accesssvc.PermissionCheck) (map[string]bool, error) {
	held := map[string]bool{}
	for _, action := range check.Actions {
		held[action] = true
	}
	return held, nil
}

type documentTypes struct{ rtservice.Service }

func (documentTypes) Tree(context.Context, string) (resourcetype.Tree, error) {
	return resourcetype.NewTree([]resourcetype.ResourceType{
		{ID: "rt_org", Slug: resourcetype.OrganizationSlug},
		{ID: "rt_doc", Slug: "document", ParentTypes: []string{resourcetype.OrganizationSlug}},
	}), nil
}

// registry finds every document registered under the Organization; its
// chain fails when chainErr is set.
type registry struct {
	resourcerepository.Repository
	chainErr error
}

func (registry) Find(
	_ context.Context, organizationID, _, externalID string,
) (functional.Option[resource.Resource], error) {
	return functional.Some(resource.Resource{
		RowID: "res_1", OrganizationID: organizationID, Type: "document", ExternalID: externalID,
		Parent: resource.OrganizationRef(organizationID),
	}), nil
}

func (r registry) Ancestors(
	_ context.Context, _, rowID string, _ int,
) ([]resourcerepository.Node, error) {
	if r.chainErr != nil {
		return nil, r.chainErr
	}
	return []resourcerepository.Node{{RowID: rowID, Ref: resource.Ref{Type: "document", ID: "doc"}}}, nil
}

func evaluatorWith(resources resourcerepository.Repository) accesssvc.Evaluator {
	return accesssvc.NewEvaluator(nil, nil, nil, keyService{}, holdsAll{}, documentTypes{}, resources, nil,
		zerolog.Nop())
}

func explained() accesssvc.Request {
	return accesssvc.Ask("prd").APIKey("key").OnResource("", "document", "doc").Can("document:view").Explaining()
}

func TestEvaluateExplains(t *testing.T) {
	t.Run("an allowed decision lists what granted it", func(t *testing.T) {
		decision, err := evaluatorWith(registry{}).Evaluate(t.Context(), explained())

		require.NoError(t, err)
		assert.True(t, decision.Allowed)
		assert.Equal(t, []accesssvc.Grant{{
			APIKeyID: testKeyID, APIKeyName: "Key",
			Resource: accesssvc.GrantResource{Type: "organization", ID: testOrganization},
		}}, decision.Items[0].GrantedBy)
	})

	t.Run("a failed explanation leaves the decision allowed and unexplained", func(t *testing.T) {
		evaluator := evaluatorWith(registry{chainErr: errors.New("connection reset")})

		decision, err := evaluator.Evaluate(t.Context(), explained())

		require.NoError(t, err)
		assert.True(t, decision.Allowed)
		require.Len(t, decision.Items, 1)
		assert.True(t, decision.Items[0].Allowed)
		assert.Nil(t, decision.Items[0].GrantedBy)
	})
}

func TestRequestBuilderKeepsTheParent(t *testing.T) {
	user := accesssvc.Ask("prd").User("user_1")
	parent := &accesssvc.ParentRef{Type: "folder", ID: "f"}

	assert.Equal(t, parent, user.Under("folder", "f").OnResource("org", "document", "d").Resource.Parent,
		"a parent named first survives the resource")
	assert.Equal(t, parent, user.OnResource("org", "document", "d").Under("folder", "f").Resource.Parent)
	assert.Nil(t, user.OnResource("org", "document", "d").Resource.Parent)
}
