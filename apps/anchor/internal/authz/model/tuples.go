package model

import (
	"cmp"
	"slices"
)

// Tuple is one relationship: User holds Relation on Object.
type Tuple struct {
	Object   string
	Relation string
	User     string
}

// OrganizationState is what Postgres holds for one organization, read in one
// snapshot. The relationship store must hold exactly the tuples it implies.
type OrganizationState struct {
	ProductID      string
	OrganizationID string
	// Exists is false once the organization is deleted: it then implies no tuple.
	Exists     bool
	Members    []RoleAssignment
	Workspaces []WorkspaceState
	APIKeys    []APIKeyGrant
}

type WorkspaceState struct {
	ID      string
	Members []RoleAssignment
}

type RoleAssignment struct {
	ProductUserID string
	RoleID        string
}

// APIKeyGrant is an active organization API key and the permissions it holds.
type APIKeyGrant struct {
	ID          string
	Permissions []string
}

// Tuples lists the relationships the organization implies under the catalog.
//
// A workspace role counts only for a member of the organization, and a role or
// permission the catalog no longer defines is skipped, because the model has no
// relation to hold it.
func (s OrganizationState) Tuples(catalog Catalog) []Tuple {
	if !s.Exists {
		return nil
	}
	organization := Organization(s.OrganizationID)
	tuples := []Tuple{{Object: organization, Relation: RelationProduct, User: Product(s.ProductID)}}
	members := make(map[string]bool, len(s.Members))
	for _, member := range s.Members {
		members[member.ProductUserID] = true
		tuples = append(tuples, Tuple{Object: organization, Relation: RelationMember, User: User(member.ProductUserID)})
		tuples = appendRole(tuples, catalog, organization, member)
	}
	for _, workspace := range s.Workspaces {
		object := Workspace(workspace.ID)
		tuples = append(tuples, Tuple{Object: object, Relation: RelationOrganization, User: organization})
		for _, member := range workspace.Members {
			if members[member.ProductUserID] {
				tuples = appendRole(tuples, catalog, object, member)
			}
		}
	}
	for _, key := range s.APIKeys {
		for _, permission := range key.Permissions {
			if catalog.HasPermission(permission) {
				tuples = append(
					tuples,
					Tuple{Object: organization, Relation: PermissionRelation(permission), User: APIKey(key.ID)},
				)
			}
		}
	}
	return tuples
}

func appendRole(tuples []Tuple, catalog Catalog, object string, member RoleAssignment) []Tuple {
	if !catalog.HasRole(member.RoleID) {
		return tuples
	}
	return append(
		tuples,
		Tuple{Object: object, Relation: RoleRelation(member.RoleID), User: User(member.ProductUserID)},
	)
}

// Changes is what turns the stored tuples into the desired ones.
type Changes struct {
	Writes  []Tuple
	Deletes []Tuple
}

func (c Changes) IsEmpty() bool { return len(c.Writes) == 0 && len(c.Deletes) == 0 }

// Diff compares the tuples a store holds with the tuples it should hold. Both
// sides may repeat a tuple; the result never does, and is sorted.
func Diff(stored, desired []Tuple) Changes {
	have := setOf(stored)
	want := setOf(desired)
	var changes Changes
	for tuple := range want {
		if !have[tuple] {
			changes.Writes = append(changes.Writes, tuple)
		}
	}
	for tuple := range have {
		if !want[tuple] {
			changes.Deletes = append(changes.Deletes, tuple)
		}
	}
	slices.SortFunc(changes.Writes, compareTuples)
	slices.SortFunc(changes.Deletes, compareTuples)
	return changes
}

func setOf(tuples []Tuple) map[Tuple]bool {
	set := make(map[Tuple]bool, len(tuples))
	for _, tuple := range tuples {
		set[tuple] = true
	}
	return set
}

func compareTuples(a, b Tuple) int {
	return cmp.Or(cmp.Compare(a.Object, b.Object), cmp.Compare(a.Relation, b.Relation), cmp.Compare(a.User, b.User))
}
