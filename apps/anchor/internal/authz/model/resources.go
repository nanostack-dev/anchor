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
	// Assignments holds every role a member holds on the resource.
	Assignments []ResourceAssignment
}

// ResourceAssignment is a member holding a role row on a resource.
type ResourceAssignment struct {
	ProductUserID string
	RoleRowID     string
}

// Tuples lists the relationships the object implies under the catalog: its
// one parent tuple, and its role assignments. A parent the model's parent
// relation would refuse is left out, as a permission the catalog no longer
// defines is. Each role held on the resource is one assignment, linked once,
// with every member holding it there as an assignee; the role's grants are
// the Organization's to sync.
func (s ResourceState) Tuples(catalog Catalog) []Tuple {
	if !s.Exists {
		return nil
	}
	var tuples []Tuple
	parentType, parent := s.Resource.parent()
	if catalog.AllowsParent(s.Resource.Type, parentType) {
		tuples = append(tuples, Tuple{Object: s.Object, Relation: RelationParent, User: parent})
	}
	if !catalog.HasType(s.Resource.Type) {
		return tuples
	}
	linked := map[string]bool{}
	for _, held := range s.Assignments {
		assignment := Assignment(s.Resource.RowID, held.RoleRowID)
		if !linked[held.RoleRowID] {
			linked[held.RoleRowID] = true
			tuples = append(tuples, AssignmentTuples(assignment)...)
			tuples = append(tuples, Tuple{Object: s.Object, Relation: RelationAssignment, User: assignment})
		}
		tuples = append(tuples, Tuple{Object: assignment, Relation: RelationAssignee, User: User(held.ProductUserID)})
	}
	return tuples
}

// AssignmentObjects lists the assignment objects among the tuples' users,
// each once: the ones a resource object links.
func AssignmentObjects(tuples []Tuple) []string {
	var objects []string
	seen := map[string]bool{}
	for _, tuple := range tuples {
		if tuple.Relation == RelationAssignment && !seen[tuple.User] {
			seen[tuple.User] = true
			objects = append(objects, tuple.User)
		}
	}
	return objects
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
