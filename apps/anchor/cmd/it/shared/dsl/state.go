package itdsl

import (
	"testing"
)

// State exposes created test entities via aliases.
type State struct {
	t             *testing.T
	tenants       map[string]*Tenant
	users         map[string]*PlatformUser
	products      map[string]*ProductContext
	productUsers  map[string]*ProductUser
	productRoles  map[string]*ProductRole
	organizations map[string]*ProductOrganization
	memberships   map[string]*OrganizationMembership
}
