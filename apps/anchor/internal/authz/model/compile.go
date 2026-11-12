package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"google.golang.org/protobuf/proto"
)

// MaxBytes is the largest model OpenFGA stores. It lifts OpenFGA's 256 KiB
// default: a model grows by about 150 bytes per permission and type that holds
// or inherits it, so a catalog of 50 types and 120 permissions still fits.
const MaxBytes = 1 << 20

// modelIDBytes is the ID OpenFGA adds to a model before it measures it: a
// 26-character ULID with its field tag and length.
const modelIDBytes = 28

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
//     lets an API key hold them directly;
//   - each resource type also answers the permissions scoped to a type above
//     it, through its parent only: no role on it may hold them.
func Compile(catalog Catalog) Model {
	catalog = catalog.normalized()
	defs := catalog.relations()
	return Model{DSL: dsl(defs), Proto: authorizationModel(defs), Hash: catalog.hash()}
}

// Bytes is the size OpenFGA measures against MaxBytes before it stores the
// model: its wire encoding with the ID OpenFGA adds.
func (m Model) Bytes() int {
	return proto.Size(m.Proto) + modelIDBytes
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
const modelVersion = "anchor-authz-model-3"

// HasPermission reports whether the model holds the permission, so tuples
// never name a relation it lacks.
func (c Catalog) HasPermission(name string) bool {
	return slices.ContainsFunc(c.Permissions, func(p Permission) bool { return p.Name == name })
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

// relations lists each type of the model with its relations, in the order
// the DSL writes them.
func (c Catalog) relations() []typeDef {
	defs := []typeDef{{name: TypeUser}, {name: TypeAPIKey}}

	role := typeDef{name: TypeRole}
	for _, permission := range c.Permissions {
		role.add(GrantsRelation(permission.Name), direct(AllUsers))
	}
	defs = append(defs, role)

	assignment := typeDef{name: TypeRoleAssignment}
	assignment.add(RelationRole, direct(TypeRole))
	assignment.add(RelationAssignee, direct(TypeUser))
	for _, permission := range c.Permissions {
		assignment.add(CanRelation(permission.Name), intersection(
			computed(RelationAssignee), from(GrantsRelation(permission.Name), RelationRole)))
	}
	defs = append(defs, assignment)

	held := c.heldOn()
	inherited := c.inheritedOn(held)
	for _, t := range c.Types {
		def := typeDef{name: ResourceType(t.Slug)}
		parentTypes := slices.Clone(t.ParentTypes)
		if t.NestsInItself {
			parentTypes = append(parentTypes, t.Slug)
		}
		hasParent := len(parentTypes) > 0
		if hasParent {
			def.add(RelationParent, direct(mapSlugs(slices.Compact(slices.Sorted(slices.Values(parentTypes))))...))
		}
		def.add(RelationAssignment, direct(TypeRoleAssignment))
		for _, permission := range inherited[t.Slug] {
			def.add(CanRelation(permission), from(CanRelation(permission), RelationParent))
		}
		for _, permission := range held[t.Slug] {
			var alternatives []rewrite
			if t.Slug == TypeOrganization {
				alternatives = append(alternatives, direct(TypeAPIKey))
			}
			alternatives = append(alternatives, from(CanRelation(permission), RelationAssignment))
			if hasParent {
				alternatives = append(alternatives, from(CanRelation(permission), RelationParent))
			}
			def.add(CanRelation(permission), union(alternatives...))
		}
		defs = append(defs, def)
	}
	return defs
}

// inheritedOn lists, for each type, the permissions scoped to a type above it
// that it does not hold itself. Its parent answers them.
func (c Catalog) inheritedOn(held map[string][]string) map[string][]string {
	parents := map[string][]string{}
	for _, t := range c.Types {
		parents[t.Slug] = t.ParentTypes
	}
	scopes := map[string][]string{}
	for _, permission := range c.Permissions {
		scopes[permission.Scope] = append(scopes[permission.Scope], permission.Name)
	}
	inherited := map[string][]string{}
	for _, t := range c.Types {
		seen := map[string]bool{t.Slug: true}
		queue := slices.Clone(t.ParentTypes)
		for len(queue) > 0 {
			slug := queue[0]
			queue = queue[1:]
			if seen[slug] {
				continue
			}
			seen[slug] = true
			for _, name := range scopes[slug] {
				if !slices.Contains(held[t.Slug], name) && !slices.Contains(inherited[t.Slug], name) {
					inherited[t.Slug] = append(inherited[t.Slug], name)
				}
			}
			queue = append(queue, parents[slug]...)
		}
		slices.Sort(inherited[t.Slug])
	}
	return inherited
}

func mapSlugs(slugs []string) []string {
	types := make([]string, len(slugs))
	for i, slug := range slugs {
		types[i] = ResourceType(slug)
	}
	return types
}

// typeDef is one type of the model and its relations in order. Compile renders
// it both as DSL and as the proto OpenFGA stores, so the two always agree
// without parsing the DSL (parsing took most of a catalog write's CPU).
type typeDef struct {
	name      string
	relations []relationDef
}

type relationDef struct {
	name    string
	rewrite rewrite
}

func (d *typeDef) add(name string, r rewrite) {
	d.relations = append(d.relations, relationDef{name: name, rewrite: r})
}

// rewrite is one of the few relation definitions the model uses: subjects
// assigned directly, another relation of the same object, a relation of the
// objects a tupleset points to, or a union or intersection of those.
type rewrite struct {
	kind     rewriteKind
	types    []string // direct: the subject types, "user:*" for every user
	relation string   // computed and from: the relation answered
	tupleset string   // from: the relation whose objects answer it
	children []rewrite
}

type rewriteKind int

const (
	rewriteDirect rewriteKind = iota
	rewriteComputed
	rewriteFrom
	rewriteUnion
	rewriteIntersection
)

func direct(types ...string) rewrite { return rewrite{kind: rewriteDirect, types: types} }

func computed(relation string) rewrite { return rewrite{kind: rewriteComputed, relation: relation} }

func from(relation, tupleset string) rewrite {
	return rewrite{kind: rewriteFrom, relation: relation, tupleset: tupleset}
}

// union of one alternative is that alternative, as the DSL writes it.
func union(children ...rewrite) rewrite {
	if len(children) == 1 {
		return children[0]
	}
	return rewrite{kind: rewriteUnion, children: children}
}

func intersection(children ...rewrite) rewrite {
	return rewrite{kind: rewriteIntersection, children: children}
}

func (r rewrite) dsl() string {
	switch r.kind {
	case rewriteDirect:
		return "[" + strings.Join(r.types, ", ") + "]"
	case rewriteComputed:
		return r.relation
	case rewriteFrom:
		return r.relation + " from " + r.tupleset
	case rewriteUnion, rewriteIntersection:
		operator := " or "
		if r.kind == rewriteIntersection {
			operator = " and "
		}
		parts := make([]string, len(r.children))
		for i, child := range r.children {
			parts[i] = child.dsl()
		}
		return strings.Join(parts, operator)
	}
	panic(fmt.Sprintf("unknown rewrite kind %d", r.kind))
}

func (r rewrite) proto() *openfgav1.Userset {
	switch r.kind {
	case rewriteDirect:
		return &openfgav1.Userset{Userset: &openfgav1.Userset_This{This: &openfgav1.DirectUserset{}}}
	case rewriteComputed:
		return &openfgav1.Userset{Userset: &openfgav1.Userset_ComputedUserset{
			ComputedUserset: &openfgav1.ObjectRelation{Relation: r.relation},
		}}
	case rewriteFrom:
		return &openfgav1.Userset{Userset: &openfgav1.Userset_TupleToUserset{TupleToUserset: &openfgav1.TupleToUserset{
			Tupleset:        &openfgav1.ObjectRelation{Relation: r.tupleset},
			ComputedUserset: &openfgav1.ObjectRelation{Relation: r.relation},
		}}}
	case rewriteUnion, rewriteIntersection:
		children := &openfgav1.Usersets{Child: make([]*openfgav1.Userset, len(r.children))}
		for i, child := range r.children {
			children.Child[i] = child.proto()
		}
		if r.kind == rewriteIntersection {
			return &openfgav1.Userset{Userset: &openfgav1.Userset_Intersection{Intersection: children}}
		}
		return &openfgav1.Userset{Userset: &openfgav1.Userset_Union{Union: children}}
	}
	panic(fmt.Sprintf("unknown rewrite kind %d", r.kind))
}

// directTypes lists the subject types a relation takes directly, for its
// metadata.
func (r rewrite) directTypes() []*openfgav1.RelationReference {
	var refs []*openfgav1.RelationReference
	switch r.kind {
	case rewriteDirect:
		for _, t := range r.types {
			if name, ok := strings.CutSuffix(t, ":*"); ok {
				refs = append(refs, &openfgav1.RelationReference{
					Type:               name,
					RelationOrWildcard: &openfgav1.RelationReference_Wildcard{Wildcard: &openfgav1.Wildcard{}},
				})
				continue
			}
			refs = append(refs, &openfgav1.RelationReference{Type: t})
		}
	case rewriteUnion, rewriteIntersection:
		for _, child := range r.children {
			refs = append(refs, child.directTypes()...)
		}
	case rewriteComputed, rewriteFrom:
	}
	return refs
}

func dsl(defs []typeDef) string {
	var b strings.Builder
	b.WriteString("model\n  schema 1.1\n")
	for _, def := range defs {
		b.WriteString("\ntype " + def.name + "\n")
		if len(def.relations) == 0 {
			continue
		}
		b.WriteString("  relations\n")
		for _, relation := range def.relations {
			b.WriteString("    define " + relation.name + ": " + relation.rewrite.dsl() + "\n")
		}
	}
	return b.String()
}

func authorizationModel(defs []typeDef) *openfgav1.AuthorizationModel {
	m := &openfgav1.AuthorizationModel{
		SchemaVersion:   schemaVersion,
		TypeDefinitions: make([]*openfgav1.TypeDefinition, len(defs)),
	}
	for i, def := range defs {
		typeDefinition := &openfgav1.TypeDefinition{Type: def.name}
		if len(def.relations) > 0 {
			typeDefinition.Relations = make(map[string]*openfgav1.Userset, len(def.relations))
			metadata := make(map[string]*openfgav1.RelationMetadata, len(def.relations))
			for _, relation := range def.relations {
				typeDefinition.Relations[relation.name] = relation.rewrite.proto()
				metadata[relation.name] = &openfgav1.RelationMetadata{
					DirectlyRelatedUserTypes: relation.rewrite.directTypes(),
				}
			}
			typeDefinition.Metadata = &openfgav1.Metadata{Relations: metadata}
		}
		m.TypeDefinitions[i] = typeDefinition
	}
	return m
}

const schemaVersion = "1.1"
