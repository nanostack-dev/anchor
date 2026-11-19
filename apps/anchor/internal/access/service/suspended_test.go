package service_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesssvc "anchor/internal/access/service"
	"anchor/internal/applog"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
	"anchor/internal/repository"
)

// productUsers finds every external ID as one product user of the given status.
type productUsers struct {
	repository.ProductUserRepository
	status user.ProductUserStatus
}

func (p productUsers) FindByExternalID(
	_ context.Context, productID, externalID string,
) (functional.Option[user.ProductUser], error) {
	return functional.Some(user.ProductUser{
		ID: "pusr_1", ProductID: productID, ExternalID: &externalID, Email: "ada@example.com", Status: p.status,
	}), nil
}

// memberships finds every product user a member of testOrganization holding
// a role that lists document:view.
type memberships struct {
	repository.OrganizationMembershipRepository
}

func (memberships) FindByProductUserIDAndOrgID(
	_ context.Context, _, _, organizationID string, _ bool,
) (functional.Option[user.OrganizationMembership], error) {
	return functional.Some(user.OrganizationMembership{
		OrganizationID:   organizationID,
		OrganizationName: "Acme",
		Role:             &role.Ref{ID: "prole_1", Name: "Reader"},
		RolePermissions:  []string{"document:view"},
	}), nil
}

func userEvaluator(status user.ProductUserStatus) accesssvc.Evaluator {
	discard := slog.New(slog.DiscardHandler)
	return accesssvc.NewEvaluator(productUsers{status: status}, memberships{}, nil, keyService{}, holdsAll{},
		documentTypes{}, registry{}, nil, discard, applog.Scoped{Logger: discard})
}

func TestEvaluateUserStatus(t *testing.T) {
	ask := accesssvc.Ask("prd").User("user_ext_1").InOrganization(testOrganization).Can("document:view")

	t.Run("a suspended member holding the permission is denied as suspended", func(t *testing.T) {
		decision, err := userEvaluator(user.ProductUserStatusSuspended).Evaluate(t.Context(), ask)

		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Equal(t, accesssvc.ReasonSubjectSuspended, decision.Reason)
		require.Len(t, decision.Items, 1)
		assert.Equal(t, accesssvc.ReasonSubjectSuspended, decision.Items[0].Reason)
		assert.Nil(t, decision.Identity)
	})

	for _, status := range []user.ProductUserStatus{user.ProductUserStatusActive, user.ProductUserStatusInvited} {
		t.Run("an "+string(status)+" member holding the permission is allowed", func(t *testing.T) {
			decision, err := userEvaluator(status).Evaluate(t.Context(), ask)

			require.NoError(t, err)
			assert.True(t, decision.Allowed)
		})
	}
}
