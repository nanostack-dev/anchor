package service_test

import (
	"testing"

	accesssvc "anchor/internal/access/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	allowed      = accesssvc.ItemDecision{Allowed: true}
	missing      = accesssvc.ItemDecision{Reason: accesssvc.ReasonMissingPermission}
	notEvaluated = accesssvc.ItemDecision{Reason: accesssvc.ReasonNotEvaluated}
)

func TestDecide(t *testing.T) {
	reader := &accesssvc.Identity{OrganizationID: "org", Permissions: []string{"flows:read", "flows:write"}}

	t.Run("every held action is allowed", func(t *testing.T) {
		decision := accesssvc.Decide(reader, accesssvc.ExecuteAll, "flows:read", "flows:write")
		assert.True(t, decision.Allowed)
		assert.Empty(t, decision.Reason)
		assert.Equal(t, []accesssvc.ItemDecision{allowed, allowed}, decision.Items)
	})

	t.Run("no action asks only for reach", func(t *testing.T) {
		decision := accesssvc.Decide(reader, accesssvc.ExecuteAll)
		assert.True(t, decision.Allowed)
		assert.Same(t, reader, decision.Identity)
	})

	t.Run("execute all reports every missing action", func(t *testing.T) {
		decision := accesssvc.Decide(reader, accesssvc.ExecuteAll, "a", "flows:read", "b")
		assert.False(t, decision.Allowed)
		assert.Equal(t, accesssvc.ReasonMissingPermission, decision.Reason)
		assert.Equal(t, []string{"a", "b"}, decision.MissingPermissions)
		assert.Equal(t, []accesssvc.ItemDecision{missing, allowed, missing}, decision.Items)
	})

	t.Run("deny on first deny skips the rest", func(t *testing.T) {
		decision := accesssvc.Decide(reader, accesssvc.DenyOnFirstDeny, "a", "flows:read")
		assert.Equal(t, []string{"a"}, decision.MissingPermissions)
		assert.Equal(t, []accesssvc.ItemDecision{missing, notEvaluated}, decision.Items)
	})

	t.Run("permit on first permit skips the rest and denies the batch", func(t *testing.T) {
		decision := accesssvc.Decide(reader, accesssvc.PermitOnFirstPermit, "flows:read", "a")
		assert.False(t, decision.Allowed)
		assert.Equal(t, accesssvc.ReasonNotEvaluated, decision.Reason)
		assert.Equal(t, []accesssvc.ItemDecision{allowed, notEvaluated}, decision.Items)
	})
}

func TestDeny(t *testing.T) {
	decision := accesssvc.Deny(accesssvc.ReasonNotAMember, "a", "b")
	assert.False(t, decision.Allowed)
	assert.Nil(t, decision.Identity)
	notAMember := accesssvc.ItemDecision{Reason: accesssvc.ReasonNotAMember}
	assert.Equal(t, []accesssvc.ItemDecision{notAMember, notAMember}, decision.Items)
}

func TestRequestBuilder(t *testing.T) {
	t.Run("builds a user question on a workspace", func(t *testing.T) {
		request := accesssvc.Ask("prd").User("user_1").InWorkspace("org", "ws").Can("flows:read")
		assert.Equal(t, accesssvc.Request{
			ProductID: "prd",
			Subject:   accesssvc.Subject{Type: accesssvc.SubjectUser, ID: "user_1"},
			Resource:  accesssvc.Resource{Type: accesssvc.ResourceWorkspace, ID: "ws", OrganizationID: "org"},
			Actions:   []string{"flows:read"},
		}, request)
		assert.Equal(t, "org", request.Organization())
	})

	t.Run("each step leaves the earlier request untouched", func(t *testing.T) {
		base := accesssvc.Ask("prd").APIKey("key").Can("a")
		_ = base.Can("b")
		assert.Equal(t, []string{"a"}, base.Actions)
	})
}

func TestCheckResource(t *testing.T) {
	user := accesssvc.Ask("prd").User("user_1")
	key := accesssvc.Ask("prd").APIKey("key")

	require.Error(t, accesssvc.CheckResource(key.InWorkspace("org", "")), "a workspace needs an id")
	require.Error(t, accesssvc.CheckResource(user.InWorkspace("", "ws")), "a user needs the workspace's organization")
	require.Error(t, accesssvc.CheckResource(user.InOrganization("")), "a user needs an organization id")
	require.NoError(t, accesssvc.CheckResource(key), "an API key defaults to its organization")
	require.NoError(t, accesssvc.CheckResource(user.InWorkspace("org", "ws")))
}

func TestCheckResourceOnProductTypes(t *testing.T) {
	user := accesssvc.Ask("prd").User("user_1")
	key := accesssvc.Ask("prd").APIKey("key")

	require.Error(t, accesssvc.CheckResource(user.OnResource("org", "document", "")), "a resource needs its id")
	require.Error(t, accesssvc.CheckResource(user.OnResource("", "document", "doc")),
		"a user needs the resource's organization")
	require.Error(t, accesssvc.CheckResource(user.InOrganization("org").Under("folder", "f")),
		"an organization takes no parent")
	require.Error(t, accesssvc.CheckResource(user.InWorkspace("org", "ws").Under("folder", "f")),
		"a workspace takes no parent")
	require.NoError(t, accesssvc.CheckResource(key.OnResource("", "document", "doc").Under("folder", "f")),
		"an API key defaults to its organization")
	require.NoError(t, accesssvc.CheckResource(user.OnResource("org", "document", "doc").Under("folder", "f")))
}
