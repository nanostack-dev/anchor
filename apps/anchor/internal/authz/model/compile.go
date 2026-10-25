package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/language/pkg/go/transformer"
)

// Catalog is what a Product defines for access: its resource types and its
// permissions with their scope. It is the only input of its model.
type Catalog struct {
	Types       []Type
	Permissions []Permission
}

// Type is a resource type: organization, or one of the Product's own.
type Type struct {
	Slug          string
	ParentTypes   []string
	NestsInItself bool
}

// Permission is a permission name and the slug of the type it is scoped to.
type Permission struct {
	Name  string
	Scope string
}

// Model is a Product's authorization model, as DSL for people and as the proto
// OpenFGA stores, with a hash that tells two catalogs apart.
type Model struct {
	DSL   string
	Proto *openfgav1.AuthorizationModel
	Hash  string
}

// Compile writes the model of a catalog (ADR-0022 E3):
//   - role holds one grants_<permission> per permission, given to every user;
//   - role_assignment ties a role to its assignee, and holds a permission when
//     both hold;
//   - each resource type holds the permissions scoped to it or to a type below
//     it, through an assignment on it or through its parent; organization also
//     lets an API key hold them directly.
func Compile(catalog Catalog) (Model, error) {
	catalog = catalog.normalized()
	dsl := catalog.dsl()
	proto, err := transformer.TransformDSLToProto(dsl)
	if err != nil {
		return Model{}, fmt.Errorf("compile authorization model: %w", err)
	}
	return Model{DSL: dsl, Proto: proto, Hash: catalog.hash()}, nil
}

// Hash tells two catalogs apart without compiling either: the same catalog in
// any order has the same hash.
func Hash(catalog Catalog) string {
	return catalog.normalized().hash()
}

func (c Catalog) hash() string {
	var b strings.Builder
	b.WriteString(modelVersion)
	for _, t := range c.Types {
		fmt.Fprintf(&b, "\nt %q %v %v", t.Slug, t.ParentTypes, t.NestsInItself)
	}
	for _, p := range c.Permissions {
		fmt.Fprintf(&b, "\np %q %q", p.Name, p.Scope)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// modelVersion changes whenever Compile writes a different model for the same
// catalog, so every store gets the new model.
const modelVersion = "anchor-authz-model-2"

// HasPermission reports whether the model holds the permission, so tuples
// never name a relation it lacks.
func (c Catalog) HasPermission(name string) bool {
	return slices.ContainsFunc(c.Permissions, func(p Permission) bool { return p.Name == name })
}

// HasType reports whether the model defines the resource type, so an
// assignment link never names a type it lacks.
func (c Catalog) HasType(slug string) bool {
	return slices.ContainsFunc(c.Types, func(t Type) bool { return t.Slug == slug })
}

// AllowsParent reports whether the model lets a resource of the type sit under
// one of the parent type, so a parent tuple never names a type the model's
// parent relation refuses.
func (c Catalog) AllowsParent(slug, parentSlug string) bool {
	for _, t := range c.Types {
		if t.Slug == slug {
			return slices.Contains(t.ParentTypes, parentSlug) || (t.NestsInItself && parentSlug == slug)
		}
	}
	return false
}

// normalized sorts everything, so the same catalog always compiles the same
// model.
func (c Catalog) normalized() Catalog {
	types := slices.Clone(c.Types)
	for i := range types {
		types[i].ParentTypes = slices.Compact(slices.Sorted(slices.Values(types[i].ParentTypes)))
	}
	slices.SortFunc(types, func(a, b Type) int {
		if a.Slug == TypeOrganization {
			return -1
		}
		if b.Slug == TypeOrganization {
			return 1
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	permissions := slices.Clone(c.Permissions)
	slices.SortFunc(permissions, func(a, b Permission) int { return strings.Compare(a.Name, b.Name) })
	return Catalog{Types: types, Permissions: slices.CompactFunc(permissions, func(a, b Permission) bool {
		return a.Name == b.Name
	})}
}

// heldOn lists, for each type, the permissions scoped to it or below it: a
// permission is held on its scope and on every type above it.
func (c Catalog) heldOn() map[string][]string {
	parents := map[string][]string{}
	for _, t := range c.Types {
		parents[t.Slug] = t.ParentTypes
	}
	held := map[string][]string{}
	for _, permission := range c.Permissions {
		seen := map[string]bool{}
		queue := []string{permission.Scope}
		for len(queue) > 0 {
			slug := queue[0]
			queue = queue[1:]
			if seen[slug] {
				continue
			}
			seen[slug] = true
			held[slug] = append(held[slug], permission.Name)
			queue = append(queue, parents[slug]...)
		}
	}
	return held
}

func (c Catalog) dsl() string {
	var w dslWriter
	w.line(indentNone, "model")
	w.line(indentRelations, "schema 1.1")
	w.typeHeader(TypeUser)
	w.typeHeader(TypeAPIKey)

	w.typeHeader(TypeRole)
	if len(c.Permissions) > 0 {
		w.line(indentRelations, "relations")
		for _, permission := range c.Permissions {
			w.define(GrantsRelation(permission.Name), "["+AllUsers+"]")
		}
	}

	w.typeHeader(TypeRoleAssignment)
	w.line(indentRelations, "relations")
	w.define(RelationRole, "["+TypeRole+"]")
	w.define(RelationAssignee, "["+TypeUser+"]")
	for _, permission := range c.Permissions {
		w.define(CanRelation(permission.Name),
			RelationAssignee+" and "+GrantsRelation(permission.Name)+" from "+RelationRole)
	}

	held := c.heldOn()
	for _, t := range c.Types {
		w.typeHeader(ResourceType(t.Slug))
		w.line(indentRelations, "relations")
		parentTypes := slices.Clone(t.ParentTypes)
		if t.NestsInItself {
			parentTypes = append(parentTypes, t.Slug)
		}
		hasParent := len(parentTypes) > 0
		if hasParent {
			w.define(
				RelationParent,
				"["+strings.Join(mapSlugs(slices.Compact(slices.Sorted(slices.Values(parentTypes)))), ", ")+"]",
			)
		}
		w.define(RelationAssignment, "["+TypeRoleAssignment+"]")
		for _, permission := range held[t.Slug] {
			alternatives := []string{CanRelation(permission) + " from " + RelationAssignment}
			if t.Slug == TypeOrganization {
				alternatives = append([]string{"[" + TypeAPIKey + "]"}, alternatives...)
			}
			if hasParent {
				alternatives = append(alternatives, CanRelation(permission)+" from "+RelationParent)
			}
			w.define(CanRelation(permission), strings.Join(alternatives, " or "))
		}
	}
	return w.String()
}

func mapSlugs(slugs []string) []string {
	types := make([]string, len(slugs))
	for i, slug := range slugs {
		types[i] = ResourceType(slug)
	}
	return types
}

// DSL indentation levels, two spaces each.
const (
	indentNone = iota
	indentRelations
	indentDefinitions
)

type dslWriter struct{ lines []string }

func (w *dslWriter) line(indent int, text string) {
	w.lines = append(w.lines, strings.Repeat("  ", indent)+text)
}

func (w *dslWriter) typeHeader(name string) {
	w.lines = append(w.lines, "")
	w.line(indentNone, "type "+name)
}

func (w *dslWriter) define(relation, rewrite string) {
	w.line(indentDefinitions, "define "+relation+": "+rewrite)
}

func (w *dslWriter) String() string {
	return strings.Join(w.lines, "\n") + "\n"
}
