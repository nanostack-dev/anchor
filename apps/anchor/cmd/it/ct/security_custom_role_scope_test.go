package ct_test

import (
	"net/http"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
)

// A custom role scoped to `project` (PR 284 opens custom roles on the
// Product's own types) is accepted as an Organization membership role: the
// membership and invitation paths resolve the role inside the Organization but
// never check that it is scoped to `organization` (ADR-0022 DR-D4: the
// membership carries the Organization-scoped role). The membership is an
// assignment on the Organization, so the project role's permissions are then
// held on every project of the Organization. With only the `project` switch
// on, a customer can thereby write what is in effect an Organization-wide
// role, which the `organization` switch was meant to refuse ("a project switch
// opens nothing above project").
func TestSecurityCustomRoleScopedToProjectOnMembership(t *testing.T) {
	t.Parallel()
	t.Run("A project custom role as a membership role answers 400 and grants nothing", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		resources := w.product.Resources(w.organization.Id)
		project := resourceID()
		resources.Put("project", project, "Website redesign", resources.Top())
		releaseManager := w.roles().Create(scopedCustomRoleBody("Release manager", "project", "project:manage"))
		user := createDSLProductUser(t, w.product)
		externalID := "user_" + ids.MustNew("ext")
		setDSLProductUserExternalID(t, w.product, user.ID, externalID)

		resp := addMemberRaw(t, w.product, w.organization.Id, user.ID, releaseManager.Id)
		response := w.product.Access().User(externalID).
			OnResource(w.organization.Id, "project", project).Can("project:manage").Decide()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assert.False(t, response.Decision, "a project role held as membership granted project:manage org-wide")
	})
}
