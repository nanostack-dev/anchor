// Package resource holds the registry of a product's own resources
// (ADR-0022 B1-B4): references named by the product's identifier inside an
// Organization and a resource type, each under exactly one parent.
package resource

import (
	"regexp"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/domain/resourcetype"
)

const (
	// MaxDepth counts the levels of a resource chain, the Organization
	// included (ADR-0022 A4).
	MaxDepth = 15
	// MaxNameLength and MaxIDLength bound a resource's name and the product's
	// identifier.
	MaxNameLength = 200
	MaxIDLength   = 200
)

// idPattern keeps a product identifier a valid URL path segment without
// escaping: RFC 3986 unreserved characters only.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,200}$`)

// ValidID reports whether id may name a resource. "." and ".." are path
// segments a client or proxy would collapse.
func ValidID(id string) bool {
	return idPattern.MatchString(id) && id != "." && id != ".."
}

// Ref names a resource the way the product does: its type's slug and its own
// identifier. The Organization is the Ref {organization, <organization id>}.
type Ref struct {
	Type string `validate:"required,max=49"`
	ID   string `validate:"required,max=255"`
}

func (r Ref) String() string { return r.Type + ":" + r.ID }

// IsOrganization reports whether the reference names the Organization.
func (r Ref) IsOrganization() bool { return r.Type == resourcetype.OrganizationSlug }

// OrganizationRef is the reference to an Organization as a parent.
func OrganizationRef(organizationID string) Ref {
	return Ref{Type: resourcetype.OrganizationSlug, ID: organizationID}
}

type Resource struct {
	// RowID is Anchor's own identifier. It names the resource in the
	// relationship store and never leaves Anchor.
	RowID          string
	ProductID      string
	OrganizationID string
	TypeID         string
	Type           string
	ExternalID     string
	Name           string
	// ParentRowID is nil when the Organization is the parent.
	ParentRowID *string
	Parent      Ref
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r Resource) Ref() Ref { return Ref{Type: r.Type, ID: r.ExternalID} }

// NewRowID returns a new prefixed KSUID for a resource row.
func NewRowID() string {
	return ids.MustNew("res")
}

// RegistrableType resolves the type a path names. Organization is the root of
// every chain, never a registered resource.
func RegistrableType(tree resourcetype.Tree, slug string) (resourcetype.ResourceType, error) {
	rt, ok := tree.BySlug(slug).Get()
	if !ok {
		return resourcetype.ResourceType{}, NewTypeNotFoundError(slug)
	}
	if rt.BuiltIn() {
		return resourcetype.ResourceType{}, ErrTypeNotRegistrable
	}
	return rt, nil
}
