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

// OrganizationState is what Anchor's tables hold for one Organization. It is
// read by several statements, so a write may land between them; that write
// leaves a mark, and the next sync applies it. Its store must hold exactly
// the tuples it implies.
type OrganizationState struct {
	OrganizationID string
	// Exists is false once the Organization is deleted: it then implies none.
	Exists      bool
	Memberships []Membership
	// Roles holds every role a membership names, and every role the
	// Organization owns, with the permissions each holds.
	Roles   []RoleGrants
	APIKeys []APIKeyGrants
}

type Membership struct {
	ProductUserID string
	RoleRowID     string
}

type RoleGrants struct {
	RowID string
	// Owned is true for a role of the Organization itself; a role template a
	// membership still holds is shared with other Organizations.
	Owned       bool
	Permissions []string
}

// APIKeyGrants is an active Organization API key and the permissions it holds.
type APIKeyGrants struct {
	ID          string
	Permissions []string
}

// Tuples lists the relationships the Organization implies under the catalog.
// A permission the catalog no longer defines is skipped: the model has no
// relation to hold it.
func (s OrganizationState) Tuples(catalog Catalog) []Tuple {
	if !s.Exists {
		return nil
	}
	organization := Organization(s.OrganizationID)
	var tuples []Tuple
	held := map[string]bool{}
	for _, member := range s.Memberships {
		assignment := Assignment(s.OrganizationID, member.RoleRowID)
		if !held[member.RoleRowID] {
			held[member.RoleRowID] = true
			tuples = append(tuples, AssignmentTuples(assignment)...)
			tuples = append(tuples, Tuple{Object: organization, Relation: RelationAssignment, User: assignment})
		}
		tuples = append(tuples, Tuple{Object: assignment, Relation: RelationAssignee, User: User(member.ProductUserID)})
	}
	for _, role := range s.Roles {
		for _, permission := range role.Permissions {
			if catalog.HasPermission(permission) {
				tuples = append(
					tuples,
					Tuple{Object: Role(role.RowID), Relation: GrantsRelation(permission), User: AllUsers},
				)
			}
		}
	}
	for _, key := range s.APIKeys {
		for _, permission := range key.Permissions {
			if catalog.HasPermission(permission) {
				tuples = append(
					tuples,
					Tuple{Object: organization, Relation: CanRelation(permission), User: APIKey(key.ID)},
				)
			}
		}
	}
	return tuples
}

// AssignmentTuples is the role tuple an assignment's ID implies, so a sync
// that finds an assignment link it no longer wants removes it too without
// reading it. Its assignees are read.
func AssignmentTuples(assignment string) []Tuple {
	parts, ok := ParseAssignment(assignment)
	if !ok {
		return nil
	}
	return []Tuple{{Object: assignment, Relation: RelationRole, User: Role(parts.RoleRowID)}}
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
