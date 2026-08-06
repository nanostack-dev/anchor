package itdsl

import (
	"testing"
)

// Builder creates test state with a fluent API.
type Builder struct {
	t              *testing.T
	tenants        map[string]*Tenant
	users          map[string]*PlatformUser
	products       map[string]*ProductContext
	productUsers   map[string]*ProductUser
	productRoles   map[string]*ProductRole
	organizations  map[string]*ProductOrganization
	memberships    map[string]*OrganizationMembership
	licenseSchemas map[string]*LicenseSchema
}

// Given starts a fluent test state builder.
func Given(t *testing.T) *Builder {
	t.Helper()
	return &Builder{
		t:              t,
		tenants:        make(map[string]*Tenant),
		users:          make(map[string]*PlatformUser),
		products:       make(map[string]*ProductContext),
		productUsers:   make(map[string]*ProductUser),
		productRoles:   make(map[string]*ProductRole),
		organizations:  make(map[string]*ProductOrganization),
		memberships:    make(map[string]*OrganizationMembership),
		licenseSchemas: make(map[string]*LicenseSchema),
	}
}

// Build finalizes the fluent setup and returns test state accessors.
func (b *Builder) Build() *State {
	b.t.Helper()

	return &State{
		t:              b.t,
		tenants:        b.tenants,
		users:          b.users,
		products:       b.products,
		productUsers:   b.productUsers,
		productRoles:   b.productRoles,
		organizations:  b.organizations,
		memberships:    b.memberships,
		licenseSchemas: b.licenseSchemas,
	}
}
