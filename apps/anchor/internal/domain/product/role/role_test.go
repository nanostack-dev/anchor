package role_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/product/role"
)

func TestRoleTypes(t *testing.T) {
	t.Run("A managed role belongs to no Organization", func(t *testing.T) {
		r := role.NewManaged("prod_1", "Admin", "")

		assert.Equal(t, role.TypeManaged, r.Type)
		assert.Empty(t, r.OrganizationID)
		assert.NotEmpty(t, r.ID)
	})

	t.Run("A custom role belongs to its Organization", func(t *testing.T) {
		r := role.NewCustom("prod_1", "org_acme", "Billing auditor", "")

		assert.Equal(t, role.TypeCustom, r.Type)
		assert.Equal(t, "org_acme", r.OrganizationID)
		assert.NotEmpty(t, r.ID)
	})

	t.Run("A row with no stored type is managed", func(t *testing.T) {
		assert.Equal(t, role.TypeManaged, role.TypeOf(nil))
	})

	t.Run("A stored type is read as is", func(t *testing.T) {
		custom := string(role.TypeCustom)

		assert.Equal(t, role.TypeCustom, role.TypeOf(&custom))
	})

	t.Run("A member of any Organization may hold a managed role", func(t *testing.T) {
		managed := role.NewManaged("prod_1", "Admin", "")

		assert.True(t, managed.AssignableIn("org_globex"))
	})

	t.Run("A member may hold a custom role of their own Organization", func(t *testing.T) {
		custom := role.NewCustom("prod_1", "org_acme", "Auditor", "")

		assert.True(t, custom.AssignableIn("org_acme"))
	})

	t.Run("A member may not hold another Organization's custom role", func(t *testing.T) {
		custom := role.NewCustom("prod_1", "org_acme", "Auditor", "")

		assert.False(t, custom.AssignableIn("org_globex"))
	})
}
