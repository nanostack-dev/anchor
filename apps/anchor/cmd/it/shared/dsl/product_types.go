package itdsl

import (
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
)

// ProductContext is a DSL-level product runtime context.
type ProductContext struct {
	ProductID                  string
	AllScopeAPIKey             string
	DefaultResourcePermissions []nanostackClient.ProductResourcePermissionResponse
	ServerURL                  string
	testingContext             *testing.T
	ownerAuthenticatedClient   *nanostackClient.ClientWithResponses
	allScopeAPIKeyClient       *nanostackClient.ClientWithResponses
}

// ProductUser is a DSL-level product user runtime context.
type ProductUser struct {
	ID           string
	ProductAlias string
	Email        string
	Name         string
	Status       string
}

// ProductRole is a DSL-level product role runtime context.
type ProductRole struct {
	ID           string
	ProductAlias string
	Name         string
}

// ProductOrganization is a DSL-level product organization runtime context.
type ProductOrganization struct {
	ID           string
	ProductAlias string
	Name         string
}

// OrganizationMembership links product user, organization and role aliases.
type OrganizationMembership struct {
	ProductAlias      string
	ProductUserAlias  string
	OrganizationAlias string
	RoleAlias         string
}
