// Package resourcetype holds a product's tree of resource types (ADR-0022 A1-A5):
// the kinds of things a product protects, under the built-in organization type.
package resourcetype

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

const (
	// OrganizationSlug names the built-in root type every product has.
	OrganizationSlug = "organization"
	OrganizationName = "Organization"

	// MaxDepth counts the levels of the type tree, organization included.
	MaxDepth = 5
	// MaxOwnTypes caps the types a product declares besides organization.
	MaxOwnTypes = 50
	// MaxParentTypes caps the parent types one type may list.
	MaxParentTypes = 10
)

type ResourceType struct {
	ID            string
	ProductID     string
	Slug          string
	Name          string
	Description   *string
	ParentTypes   []string
	NestsInItself bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewOrganization is the root type seeded for a product.
func NewOrganization(productID string) ResourceType {
	return ResourceType{ID: NewID(), ProductID: productID, Slug: OrganizationSlug, Name: OrganizationName}
}

// NewID returns a new prefixed KSUID for a resource type.
func NewID() string {
	return ids.MustNew("rtype")
}

// BuiltIn reports whether the type is the product's organization root.
func (t ResourceType) BuiltIn() bool {
	return t.Slug == OrganizationSlug
}
