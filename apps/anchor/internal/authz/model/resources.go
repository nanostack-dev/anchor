package model

import "strings"

// Resource is a registered resource and its one parent, both by row ID and
// type slug. An empty ParentRowID means the Organization is the parent.
type Resource struct {
	OrganizationID string
	RowID          string
	Type           string
	ParentRowID    string
	ParentType     string
}

// Object names the resource in the store.
func (r Resource) Object() string { return ResourceObject(r.Type, r.RowID) }

func (r Resource) parent() (string, string) {
	if r.ParentRowID == "" {
		return TypeOrganization, Organization(r.OrganizationID)
	}
	return r.ParentType, ResourceObject(r.ParentType, r.ParentRowID)
}

// ResourceState is what Anchor's tables hold for one resource object. A
// resource sync reads only the objects a write marked, so its cost never
// grows with the Organization's registry; the Organization sync never reads
// or writes these tuples.
type ResourceState struct {
	Object string
	// Exists is false once the row is gone: the object then implies none.
	Exists   bool
	Resource Resource
}

// Tuples lists the relationships the object implies under the catalog: its
// one parent tuple. A parent the model's parent relation would refuse is left
// out, as a permission the catalog no longer defines is.
func (s ResourceState) Tuples(catalog Catalog) []Tuple {
	if !s.Exists {
		return nil
	}
	parentType, parent := s.Resource.parent()
	if !catalog.AllowsParent(s.Resource.Type, parentType) {
		return nil
	}
	return []Tuple{{Object: s.Object, Relation: RelationParent, User: parent}}
}

// ParseResourceObject splits a resource object back into its type slug and
// its row ID. It refuses an object of a fixed type, such as an Organization.
func ParseResourceObject(object string) (string, string, bool) {
	objectType, rowID, found := strings.Cut(object, ":")
	slug, prefixed := strings.CutPrefix(objectType, resourceTypePrefix)
	if !found || !prefixed || slug == "" || rowID == "" {
		return "", "", false
	}
	return slug, rowID, true
}
