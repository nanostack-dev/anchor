// Package fga is the small, generic surface Anchor uses to talk to OpenFGA:
// tuples, a store pinned to one authorization model, and the diff that turns
// what a store holds into what it should hold. It knows nothing of Anchor's
// domain; the types and relations a tuple names are the caller's.
package fga

import (
	"cmp"
	"slices"
	"strings"
)

// Tuple is one relationship: User holds Relation on Object. Object and User
// are references in OpenFGA's form, "type:id"; User may also be a wildcard,
// "type:*", or a userset, "type:id#relation".
type Tuple struct {
	Object   string
	Relation string
	User     string
}

func (t Tuple) String() string { return t.Object + " # " + t.Relation + " @ " + t.User }

// ObjectType is the type part of the tuple's object.
func (t Tuple) ObjectType() string { return TypeOf(t.Object) }

// Ref formats a reference to one object.
func Ref(objectType, id string) string { return objectType + ":" + id }

// Wildcard formats a reference to every object of a type, as a user.
func Wildcard(objectType string) string { return objectType + ":*" }

// TypeOf returns the type part of a reference.
func TypeOf(ref string) string {
	objectType, _, _ := strings.Cut(ref, ":")
	return objectType
}

// IDOf returns the ID part of a reference, without a userset relation.
func IDOf(ref string) string {
	_, id, _ := strings.Cut(ref, ":")
	id, _, _ = strings.Cut(id, "#")
	return id
}

// Changes is what turns the tuples a store holds into the ones it should hold.
type Changes struct {
	Writes  []Tuple
	Deletes []Tuple
}

func (c Changes) IsEmpty() bool { return len(c.Writes) == 0 && len(c.Deletes) == 0 }

// Diff compares the tuples a store holds with the tuples it should hold. Either
// side may repeat a tuple; the result never does, and is sorted.
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
	Sort(changes.Writes)
	Sort(changes.Deletes)
	return changes
}

// Sort orders tuples by object, relation, then user.
func Sort(tuples []Tuple) {
	slices.SortFunc(tuples, func(a, b Tuple) int {
		return cmp.Or(cmp.Compare(a.Object, b.Object), cmp.Compare(a.Relation, b.Relation), cmp.Compare(a.User, b.User))
	})
}

func setOf(tuples []Tuple) map[Tuple]bool {
	set := make(map[Tuple]bool, len(tuples))
	for _, tuple := range tuples {
		set[tuple] = true
	}
	return set
}
