// Package model compiles a Product's catalog into its OpenFGA authorization
// model, and an Organization's roles, members and API keys into the tuples
// that model reads (ADR-0022 E3). Roles are data: the model names types and
// permissions only, so a role change never changes it.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Fixed object types of every Product's model.
const (
	TypeUser           = "user"
	TypeAPIKey         = "api_key"
	TypeRole           = "role"
	TypeRoleAssignment = "role_assignment"
	TypeOrganization   = "organization"
)

// Relations every model defines.
const (
	RelationRole       = "role"
	RelationAssignee   = "assignee"
	RelationAssignment = "assignment"
	RelationParent     = "parent"
)

const (
	// OpenFGA refuses relation names longer than this.
	maxRelationLength = 50
	grantsPrefix      = "grants"
	canPrefix         = "can"
	// Enough of a SHA-256 to keep two names apart once they are truncated or
	// reduced to the same safe characters ("doc:read" and "doc_read").
	relationHashLength = 12
	// AllUsers is the wildcard subject of a role's grants.
	AllUsers = TypeUser + ":*"
)

// ResourceType names a resource type in the model. A Product's own types carry
// a prefix, so no valid slug can shadow a fixed type, collide with another, or
// be a word of OpenFGA's language ("define", "and", "self").
func ResourceType(slug string) string {
	if slug == TypeOrganization {
		return slug
	}
	return "rt_" + slug
}

// CanRelation answers whether a subject holds a permission on an object.
func CanRelation(permission string) string {
	return relationName(canPrefix, permission)
}

// GrantsRelation is a role's bundling of a permission.
func GrantsRelation(permission string) string {
	return relationName(grantsPrefix, permission)
}

// relationName keeps the readable part of a permission for debugging and
// appends a hash of the original, because permission names hold characters
// OpenFGA does not accept in a relation (":" and "*" among them).
func relationName(prefix, permission string) string {
	sum := sha256.Sum256([]byte(permission))
	suffix := hex.EncodeToString(sum[:])[:relationHashLength]
	readable := sanitize(permission)
	room := maxRelationLength - len(prefix+"_") - len("_"+suffix)
	if len(readable) > room {
		readable = readable[:room]
	}
	return prefix + "_" + readable + "_" + suffix
}

func sanitize(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Object references, formatted the way OpenFGA expects ("type:id").

func User(productUserID string) string { return TypeUser + ":" + productUserID }

func APIKey(apiKeyID string) string { return TypeAPIKey + ":" + apiKeyID }

func Role(roleRowID string) string { return TypeRole + ":" + roleRowID }

func Organization(organizationID string) string { return TypeOrganization + ":" + organizationID }

// Assignment names one role assignment. Its ID holds everything its tuples
// say, so they never change: a member who changes role gets a new assignment,
// and a sync only reads the Organization's assignment links.
func Assignment(organizationID, productUserID, roleRowID string) string {
	return TypeRoleAssignment + ":" + organizationID + "." + productUserID + "." + roleRowID
}

// AssignmentParts is what an assignment's ID holds.
type AssignmentParts struct {
	OrganizationID string
	ProductUserID  string
	RoleRowID      string
}

// ParseAssignment splits an assignment reference back into its parts.
func ParseAssignment(object string) (AssignmentParts, bool) {
	id, found := strings.CutPrefix(object, TypeRoleAssignment+":")
	if !found {
		return AssignmentParts{}, false
	}
	parts := strings.Split(id, ".")
	const assignmentParts = 3
	if len(parts) != assignmentParts || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return AssignmentParts{}, false
	}
	return AssignmentParts{OrganizationID: parts[0], ProductUserID: parts[1], RoleRowID: parts[2]}, true
}

// ObjectID returns the ID part of an object reference.
func ObjectID(object string) string {
	_, id, _ := strings.Cut(object, ":")
	return id
}
