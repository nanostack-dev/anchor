package model

import (
	"fmt"
	"slices"
	"strings"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/language/pkg/go/transformer"
)

// Catalog is what a product defines for access: its roles and the permissions
// each role grants. It is the only input of the product's authorization model.
type Catalog struct {
	Roles       []Role
	Permissions []string
}

type Role struct {
	ID          string
	Name        string
	Permissions []string
}

// HasRole reports whether the catalog defines the role, so tuples never name a
// relation the model lacks.
func (c Catalog) HasRole(roleID string) bool {
	return slices.ContainsFunc(c.Roles, func(role Role) bool { return role.ID == roleID })
}

// RoleByRelation finds the role a relation assigns.
func (c Catalog) RoleByRelation(relation string) (Role, bool) {
	for _, role := range c.Roles {
		if RoleRelation(role.ID) == relation {
			return role, true
		}
	}
	return Role{}, false
}

// HasPermission reports whether the catalog defines the permission.
func (c Catalog) HasPermission(permission string) bool {
	return slices.Contains(c.Permissions, permission)
}

// Model is a product's authorization model, as DSL for people and as the proto
// OpenFGA stores.
type Model struct {
	DSL   string
	Proto *openfgav1.AuthorizationModel
}

// Build writes the authorization model for a catalog.
//
// Every role becomes a relation that can be assigned on an organization and on
// a workspace. Every permission becomes a computed relation: on an organization
// it is held through any role that grants it, or directly by an API key; on a
// workspace it is held through a workspace role that grants it, or through the
// organization. So a member's permissions in a workspace are the union of their
// organization role and their workspace role.
func Build(catalog Catalog) (Model, error) {
	catalog = catalog.normalized()
	dsl := writeDSL(catalog)
	proto, err := transformer.TransformDSLToProto(dsl)
	if err != nil {
		return Model{}, fmt.Errorf("build authorization model: %w", err)
	}
	return Model{DSL: dsl, Proto: proto}, nil
}

// normalized sorts everything so the same catalog always builds the same model.
func (c Catalog) normalized() Catalog {
	roles := slices.Clone(c.Roles)
	for i := range roles {
		roles[i].Permissions = slices.Compact(slices.Sorted(slices.Values(roles[i].Permissions)))
	}
	slices.SortFunc(roles, func(a, b Role) int { return strings.Compare(a.ID, b.ID) })
	return Catalog{
		Roles:       roles,
		Permissions: slices.Compact(slices.Sorted(slices.Values(c.Permissions))),
	}
}

// grantingRoles returns the relations of the roles that grant a permission.
func (c Catalog) grantingRoles(permission string) []string {
	var relations []string
	for _, role := range c.Roles {
		if slices.Contains(role.Permissions, permission) {
			relations = append(relations, RoleRelation(role.ID))
		}
	}
	return relations
}

func writeDSL(catalog Catalog) string {
	var w dslWriter
	w.line(indentNone, "model")
	w.line(indentRelations, "schema 1.1")
	w.typeHeader(TypeUser)
	w.typeHeader(TypeAPIKey)
	w.typeHeader(TypeProduct)

	w.typeHeader(TypeOrganization)
	w.line(indentRelations, "relations")
	w.define(RelationProduct, "["+TypeProduct+"]")
	w.define(RelationMember, "["+TypeUser+"]")
	for _, role := range catalog.Roles {
		w.define(RoleRelation(role.ID), "["+TypeUser+"]")
	}
	for _, permission := range catalog.Permissions {
		w.permission(permission, append([]string{"[" + TypeAPIKey + "]"}, catalog.grantingRoles(permission)...))
	}

	w.typeHeader(TypeWorkspace)
	w.line(indentRelations, "relations")
	w.define(RelationOrganization, "["+TypeOrganization+"]")
	for _, role := range catalog.Roles {
		w.define(RoleRelation(role.ID), "["+TypeUser+"]")
	}
	for _, permission := range catalog.Permissions {
		relation := PermissionRelation(permission)
		w.permission(permission, append(catalog.grantingRoles(permission), relation+" from "+RelationOrganization))
	}
	return w.String()
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

// permission writes a permission relation with a comment naming the original
// permission, since the relation name only keeps part of it readable.
func (w *dslWriter) permission(permission string, alternatives []string) {
	w.line(indentDefinitions, "# "+permission)
	w.define(PermissionRelation(permission), strings.Join(alternatives, " or "))
}

func (w *dslWriter) String() string {
	return strings.Join(w.lines, "\n") + "\n"
}
