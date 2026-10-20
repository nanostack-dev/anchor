package service_test

import (
	"testing"

	accesssvc "anchor/internal/access/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecide(t *testing.T) {
	identity := &accesssvc.Identity{OrganizationID: "org", Permissions: []string{"flows:read", "flows:write"}}

	t.Run("every held action is allowed", func(t *testing.T) {
		decision := accesssvc.Decide([]string{"flows:read", "flows:write"}, accesssvc.ExecuteAll, identity)
		assert.True(t, decision.Allowed)
		assert.Empty(t, decision.Reason)
		assert.Equal(t, []accesssvc.ItemDecision{{Allowed: true}, {Allowed: true}}, decision.Items)
	})

	t.Run("no action asks only for reach", func(t *testing.T) {
		decision := accesssvc.Decide(nil, accesssvc.ExecuteAll, identity)
		assert.True(t, decision.Allowed)
		assert.Same(t, identity, decision.Identity)
	})

	t.Run("execute all reports every missing action", func(t *testing.T) {
		decision := accesssvc.Decide([]string{"a", "flows:read", "b"}, accesssvc.ExecuteAll, identity)
		assert.False(t, decision.Allowed)
		assert.Equal(t, accesssvc.ReasonMissingPermission, decision.Reason)
		assert.Equal(t, []string{"a", "b"}, decision.MissingPermissions)
		assert.Equal(t, []accesssvc.ItemDecision{
			{Reason: accesssvc.ReasonMissingPermission}, {Allowed: true}, {Reason: accesssvc.ReasonMissingPermission},
		}, decision.Items)
	})

	t.Run("deny on first deny skips the rest", func(t *testing.T) {
		decision := accesssvc.Decide([]string{"a", "flows:read"}, accesssvc.DenyOnFirstDeny, identity)
		assert.Equal(t, []string{"a"}, decision.MissingPermissions)
		assert.Equal(
			t,
			[]accesssvc.ItemDecision{
				{Reason: accesssvc.ReasonMissingPermission},
				{Reason: accesssvc.ReasonNotEvaluated},
			},
			decision.Items,
		)
	})

	t.Run("permit on first permit skips the rest and denies the batch", func(t *testing.T) {
		decision := accesssvc.Decide([]string{"flows:read", "a"}, accesssvc.PermitOnFirstPermit, identity)
		assert.False(t, decision.Allowed)
		assert.Equal(t, accesssvc.ReasonNotEvaluated, decision.Reason)
		assert.Equal(
			t,
			[]accesssvc.ItemDecision{{Allowed: true}, {Reason: accesssvc.ReasonNotEvaluated}},
			decision.Items,
		)
	})
}

func TestDenyAll(t *testing.T) {
	decision := accesssvc.DenyAll([]string{"a", "b"}, accesssvc.ReasonNotAMember)
	assert.False(t, decision.Allowed)
	assert.Nil(t, decision.Identity)
	assert.Equal(
		t,
		[]accesssvc.ItemDecision{{Reason: accesssvc.ReasonNotAMember}, {Reason: accesssvc.ReasonNotAMember}},
		decision.Items,
	)
}

func TestValidateResource(t *testing.T) {
	require.Error(
		t,
		accesssvc.ValidateResource(
			accesssvc.Subject{Type: accesssvc.SubjectUser},
			accesssvc.Resource{Type: accesssvc.ResourceWorkspace},
		),
		"a workspace needs an id",
	)
	require.Error(
		t,
		accesssvc.ValidateResource(
			accesssvc.Subject{Type: accesssvc.SubjectUser},
			accesssvc.Resource{Type: accesssvc.ResourceWorkspace, ID: "ws"},
		),
		"a user needs the workspace's organization",
	)
	require.Error(
		t,
		accesssvc.ValidateResource(
			accesssvc.Subject{Type: accesssvc.SubjectUser},
			accesssvc.Resource{Type: accesssvc.ResourceOrganization},
		),
		"a user needs an organization id",
	)
	require.NoError(
		t,
		accesssvc.ValidateResource(
			accesssvc.Subject{Type: accesssvc.SubjectOrganizationAPIKey},
			accesssvc.Resource{Type: accesssvc.ResourceOrganization},
		),
		"an API key defaults to its organization",
	)
	require.NoError(t, accesssvc.ValidateResource(accesssvc.Subject{Type: accesssvc.SubjectUser},
		accesssvc.Resource{Type: accesssvc.ResourceWorkspace, ID: "ws", OrganizationID: "org"}))
}
